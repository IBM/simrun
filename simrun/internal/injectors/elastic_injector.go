// Package injectors writes log documents directly into a SIEM, bypassing
// detonation.
package injectors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/IBM/simrun/simrun/internal/envutil"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type ElasticInjectorDocument struct {
	Index    string            `json:"index" yaml:"index" mapstructure:"index"`
	File     string            `json:"file,omitempty" yaml:"file,omitempty" mapstructure:"file,omitempty"`
	Template string            `json:"template,omitempty" yaml:"template,omitempty" mapstructure:"template,omitempty"`
	Pack     string            `json:"pack,omitempty" yaml:"pack,omitempty" mapstructure:"pack,omitempty"`
	Vars     map[string]string `json:"vars" yaml:"vars" mapstructure:"vars"`
}

type ElasticInjector struct {
	Documents     []ElasticInjectorDocument `json:"documents" yaml:"documents" mapstructure:"documents"`
	TemplateCache map[string]string         `json:"-" yaml:"-"` // template ID -> decoded content
	EnvVars       map[string]string         `json:"-" yaml:"-"` // run-specific env vars
}

func (m *ElasticInjector) Inject() (map[string]string, error) {
	execUuid := uuid.New()
	executionId := execUuid.String()

	logger := logrus.WithFields(logrus.Fields{
		"injector":     "ElasticInjector",
		"execution_id": executionId,
	})

	client, err := m.createElasticsearchClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create Elasticsearch client: %w", err)
	}

	for i, doc := range m.Documents {
		logger.WithFields(logrus.Fields{
			"document": i + 1,
			"index":    doc.Index,
			"file":     doc.File,
		}).Info("Injecting document")

		err := m.injectDocument(client, doc, executionId)
		if err != nil {
			return nil, fmt.Errorf("failed to inject document %d: %w", i+1, err)
		}
	}

	logger.Info("All documents injected successfully")
	return map[string]string{"execution_id": executionId}, nil
}

func (m *ElasticInjector) String() string {
	return "ElasticInjector"
}

func (m *ElasticInjector) createElasticsearchClient() (*elasticsearch.Client, error) {
	var opts []elasticsearch.Option

	if cloudID := envutil.Lookup(m.EnvVars, "SR_ELASTIC_CLOUD_ID"); cloudID != "" {
		opts = append(opts, elasticsearch.WithCloudID(cloudID))
	} else if url := envutil.Lookup(m.EnvVars, "SR_ELASTIC_URL"); url != "" {
		opts = append(opts, elasticsearch.WithAddresses(url))
	}

	if apiKey := envutil.Lookup(m.EnvVars, "SR_ELASTIC_API_KEY"); apiKey != "" {
		opts = append(opts, elasticsearch.WithAPIKey(apiKey))
	} else if username := envutil.Lookup(m.EnvVars, "SR_ELASTIC_USERNAME"); username != "" {
		if password := envutil.Lookup(m.EnvVars, "SR_ELASTIC_PASSWORD"); password != "" {
			opts = append(opts, elasticsearch.WithBasicAuth(username, password))
		}
	}

	return elasticsearch.New(opts...)
}

// resolveTemplate gets template content from the pack cache or from a local file.
func (m *ElasticInjector) resolveTemplate(doc ElasticInjectorDocument) ([]byte, error) {
	if doc.Template != "" {
		if m.TemplateCache == nil {
			return nil, fmt.Errorf("template %q specified but no template cache available", doc.Template)
		}
		content, ok := m.TemplateCache[doc.Template]
		if !ok {
			available := make([]string, 0, len(m.TemplateCache))
			for k := range m.TemplateCache {
				available = append(available, k)
			}
			return nil, fmt.Errorf("template %q not found in pack (available: %v)", doc.Template, available)
		}
		return []byte(content), nil
	}

	if doc.File != "" {
		content, err := os.ReadFile(doc.File)
		if err != nil {
			return nil, fmt.Errorf("failed to read template file %s: %w", doc.File, err)
		}
		return content, nil
	}

	return nil, fmt.Errorf("document must specify either 'template' with 'pack', or 'file'")
}

func (m *ElasticInjector) injectDocument(client *elasticsearch.Client, doc ElasticInjectorDocument, executionId string) error {
	templateContent, err := m.resolveTemplate(doc)
	if err != nil {
		return err
	}

	templateVars := make(map[string]string)
	for k, v := range doc.Vars {
		templateVars[k] = v
	}
	templateVars["ExecutionID"] = executionId
	// templateVars["Timestamp"] = time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	templateVars["Timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// Parse and execute the template (insert variables into the document)
	tmpl, err := template.New("injection").Parse(string(templateContent))
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	var result bytes.Buffer
	err = tmpl.Execute(&result, templateVars)
	if err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	var jsonData map[string]interface{}
	err = json.Unmarshal(result.Bytes(), &jsonData)
	if err != nil {
		return fmt.Errorf("template result is not valid JSON: %w", err)
	}

	// If message field exists and is an object, marshal it to a string
	if messageData, exists := jsonData["message"]; exists {
		if messageObj, ok := messageData.(map[string]interface{}); ok {
			messageJSON, err := json.Marshal(messageObj)
			if err != nil {
				return fmt.Errorf("failed to marshal message field to JSON: %w", err)
			}
			jsonData["message"] = string(messageJSON)
		}
		// If message is already a string, leave it as is
	}

	jsonData["orchestrator"] = map[string]interface{}{
		"type":          "simrun",
		"resource.type": "scenario",
		"resource.id":   executionId,
	}

	document, err := json.Marshal(jsonData)
	if err != nil {
		return fmt.Errorf("failed to marshal full document to JSON: %w", err)
	}

	res, err := client.Index(
		doc.Index,
		strings.NewReader(string(document)),
		client.Index.WithContext(context.Background()),
	)
	if err != nil {
		return fmt.Errorf("failed to index document: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("elasticsearch returned error: %s - %s", res.Status(), string(body))
	}

	return nil
}
