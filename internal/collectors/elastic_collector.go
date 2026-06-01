// Package collectors gathers related logs from a SIEM after a simulation runs.
package collectors

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IBM/simrun/internal/envutil"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/sirupsen/logrus"
)

var logger *logrus.Entry

// ElasticCollectorConfig holds the configuration for Elastic collector from config file
type ElasticCollectorConfig struct {
	ElasticsearchURL string `json:"elasticsearchUrl" yaml:"elasticsearchUrl"`
	CloudID          string `json:"cloudId" yaml:"cloudId"` // alternative to ElasticsearchURL
	APIKey           string `json:"apiKey" yaml:"apiKey"`
	OutputDir        string `json:"outputDir" yaml:"outputDir"` // default: "logs"
	// UserAgentField is the field used to search for user-agent containing detonation UUID
	UserAgentField string `json:"userAgentField" yaml:"userAgentField"`
}

// ElasticCollectorScenarioConfig holds the per-scenario collector configuration
type ElasticCollectorScenarioConfig struct {
	Index string `json:"index" yaml:"index"`
	// AdditionalFields values can be templates like "{{ indicators.terraformOutput.key }}"
	AdditionalFields map[string]string `json:"additionalFields,omitempty" yaml:"additionalFields,omitempty"`
}

// ElasticCollector collects logs from Elasticsearch based on scenario indicators
type ElasticCollector struct {
	Config         *ElasticCollectorConfig
	ScenarioConfig *ElasticCollectorScenarioConfig
	Scenario       string
	outputPath     string
	client         *elasticsearch.Client // cached
}

// ElasticSearchResponse represents the response from Elasticsearch search API
type ElasticSearchResponse struct {
	Took int `json:"took"`
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []ElasticSearchHit `json:"hits"`
	} `json:"hits"`
}

// ElasticSearchHit represents a single hit from Elasticsearch
type ElasticSearchHit struct {
	ID     string                 `json:"_id"`
	Index  string                 `json:"_index"`
	Source map[string]interface{} `json:"_source"`
}

// NewElasticCollector creates a new ElasticCollector
func NewElasticCollector(config *ElasticCollectorConfig, scenarioConfig *ElasticCollectorScenarioConfig, scenario string) *ElasticCollector {
	return &ElasticCollector{
		Config:         config,
		ScenarioConfig: scenarioConfig,
		Scenario:       scenario,
	}
}

// Collect searches for logs matching the configured query and indicators,
// and writes them to the output file. Returns the number of documents collected.
func (c *ElasticCollector) Collect(ctx context.Context, indicators map[string]string) (int, error) {
	logger = logrus.WithFields(logrus.Fields{
		"collector": "ElasticCollector",
		"index":     c.ScenarioConfig.Index,
		"scenario":  c.Scenario,
	})

	if err := c.buildOutputPath(indicators); err != nil {
		return 0, fmt.Errorf("failed to build output path: %w", err)
	}

	query := c.buildSearchQuery(indicators)

	documents, err := c.executeSearch(query)
	if err != nil {
		return 0, fmt.Errorf("failed to execute search: %w", err)
	}

	if len(documents) == 0 {
		logger.Debug("No documents found matching the query")
		return 0, nil
	}

	if err := c.writeNDJSON(documents); err != nil {
		return 0, fmt.Errorf("failed to write NDJSON file: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"document_count": len(documents),
		"output_path":    c.outputPath,
	}).Info("Successfully collected and wrote documents to file")

	return len(documents), nil
}

// String returns the textual representation of the collector
func (c *ElasticCollector) String() string {
	return fmt.Sprintf("ElasticCollector(index=%s)", c.ScenarioConfig.Index)
}

// GetOutputPath returns the path where the collected logs are stored
func (c *ElasticCollector) GetOutputPath() string {
	return c.outputPath
}

func (c *ElasticCollector) buildOutputPath(indicators map[string]string) error {
	baseDir := c.Config.OutputDir
	if baseDir == "" {
		baseDir = "detonation-logs"
	}

	// Output structure: {outputDir}/{simulation_id}/{timestamp}_{execution_id}.ndjson
	timestamp := time.Now().UTC().Format("20060102-150405")

	// Use simulation_id if available, otherwise fall back to scenario name
	dirName := c.Scenario
	if simulationID, exists := indicators["simulation_id"]; exists && simulationID != "" {
		dirName = simulationID
	}

	dir := filepath.Join(baseDir, dirName)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory %s: %w", dir, err)
	}

	filename := timestamp
	if executionID, exists := indicators["execution_id"]; exists && executionID != "" {
		filename = fmt.Sprintf("%s_%s", timestamp, executionID)
	}

	c.outputPath = filepath.Join(dir, fmt.Sprintf("%s.ndjson", filename))
	return nil
}

func (c *ElasticCollector) buildSearchQuery(indicators map[string]string) string {
	should := []map[string]interface{}{}

	if executionID, exists := indicators["execution_id"]; exists && c.Config.UserAgentField != "" {
		should = append(should, map[string]interface{}{
			"wildcard": map[string]interface{}{
				c.Config.UserAgentField: fmt.Sprintf("*%s*", executionID),
			},
		})
	}

	// Also search for execution_uuid in user-agent (stratus packs inject a
	// derived UUID into cloud provider User-Agent headers instead of the nanoid).
	if executionUUID, exists := indicators["execution_uuid"]; exists && c.Config.UserAgentField != "" {
		should = append(should, map[string]interface{}{
			"wildcard": map[string]interface{}{
				c.Config.UserAgentField: fmt.Sprintf("*%s*", executionUUID),
			},
		})
	}

	for field, value := range c.ScenarioConfig.AdditionalFields {
		resolvedValue := c.resolveTemplateValue(value, indicators)
		if resolvedValue != "" {
			should = append(should, map[string]interface{}{
				"match_phrase": map[string]interface{}{
					field: resolvedValue,
				},
			})
		}
	}

	query := map[string]interface{}{
		"size": 100,
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"should":               should,
				"minimum_should_match": 1,
				"filter": []map[string]interface{}{
					{
						"range": map[string]interface{}{
							"@timestamp": map[string]interface{}{
								"gte": "now-1h",
							},
						},
					},
				},
			},
		},
		"sort": []map[string]interface{}{
			{
				"@timestamp": map[string]string{
					"order": "asc",
				},
			},
		},
	}

	queryBytes, _ := json.Marshal(query)
	// logger.WithField("query", string(queryBytes)).Debug("Built Elasticsearch query")
	return string(queryBytes)
}

// resolveTemplateValue resolves "{{ indicators.terraformOutput.key }}" style templates
// Supports embedded templates like "prefix/{{ indicators.terraformOutput.key }}/suffix"
func (c *ElasticCollector) resolveTemplateValue(value string, indicators map[string]string) string {
	if !strings.Contains(value, "{{") {
		return value
	}

	result := value
	for {
		start := strings.Index(result, "{{")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], "}}")
		if end == -1 {
			break
		}
		end += start + 2

		template := result[start:end]
		key := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(template, "{{"), "}}"))

		// Strip known prefixes to normalize indicator keys
		if strings.HasPrefix(key, "indicators.terraformOutput.") {
			key = strings.TrimPrefix(key, "indicators.terraformOutput.")
		} else if strings.HasPrefix(key, "indicators.static.") {
			key = strings.TrimPrefix(key, "indicators.static.")
		}

		if resolved, exists := indicators[key]; exists {
			result = result[:start] + resolved + result[end:]
		} else {
			// Template not resolved, return empty to skip this field
			return ""
		}
	}
	return result
}

func (c *ElasticCollector) getClient() (*elasticsearch.Client, error) {
	if c.client != nil {
		return c.client, nil
	}

	var opts []elasticsearch.Option

	if c.Config.CloudID != "" {
		opts = append(opts, elasticsearch.WithCloudID(c.Config.CloudID))
	} else if c.Config.ElasticsearchURL != "" {
		opts = append(opts, elasticsearch.WithAddresses(c.Config.ElasticsearchURL))
	}

	if c.Config.APIKey != "" {
		opts = append(opts, elasticsearch.WithAPIKey(c.Config.APIKey))
	}

	client, err := elasticsearch.New(opts...)
	if err != nil {
		return nil, err
	}

	c.client = client
	return c.client, nil
}

func (c *ElasticCollector) executeSearch(query string) ([]map[string]interface{}, error) {
	client, err := c.getClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get Elasticsearch client: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"index": c.ScenarioConfig.Index,
		"query": query,
	}).Debug("Executing Elasticsearch search")

	res, err := client.Search(
		client.Search.WithIndex(c.ScenarioConfig.Index),
		client.Search.WithBody(strings.NewReader(query)),
	)
	if err != nil {
		return nil, fmt.Errorf("error executing search request: %w", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		var errorBody map[string]interface{}
		if err := json.NewDecoder(res.Body).Decode(&errorBody); err == nil {
			logger.WithFields(logrus.Fields{
				"status": res.Status(),
				"error":  errorBody,
			}).Error("Elasticsearch search failed")
		}
		return nil, fmt.Errorf("search failed with status: %s", res.Status())
	}

	var searchResp ElasticSearchResponse
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("error parsing search response: %w", err)
	}

	logger.WithFields(logrus.Fields{
		"total_hits": searchResp.Hits.Total.Value,
		"returned":   len(searchResp.Hits.Hits),
	}).Info("Elasticsearch search completed")

	documents := make([]map[string]interface{}, 0, len(searchResp.Hits.Hits))
	for _, hit := range searchResp.Hits.Hits {
		doc := hit.Source
		doc["_id"] = hit.ID
		doc["_index"] = hit.Index
		documents = append(documents, doc)
	}

	return documents, nil
}

func (c *ElasticCollector) writeNDJSON(documents []map[string]interface{}) (err error) {
	file, err := os.Create(c.outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if cerr := file.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("failed to close output file: %w", cerr)
		}
	}()

	writer := bufio.NewWriter(file)

	for _, doc := range documents {
		var buf bytes.Buffer
		encoder := json.NewEncoder(&buf)
		encoder.SetEscapeHTML(false)
		if err := encoder.Encode(doc); err != nil {
			return fmt.Errorf("failed to encode document: %w", err)
		}
		if _, err := writer.Write(buf.Bytes()); err != nil {
			return fmt.Errorf("failed to write document: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush output file: %w", err)
	}
	return nil
}

// LoadElasticCollectorConfig builds the collector configuration from a
// run-specific env-var map. The caller is responsible for populating envVars
// from a resolved Elastic connector (post-Phase 4) or from inherited process
// env (transitional CLI path). Pass nil for a zero-valued config.
//
// Recognised keys:
//   - SR_ELASTIC_URL, SR_ELASTIC_CLOUD_ID, SR_ELASTIC_API_KEY
//   - SR_COLLECTOR_OUTPUT_DIR (defaults to "./logs")
//   - SR_COLLECTOR_USER_AGENT_FIELD (defaults to "user_agent.original")
func LoadElasticCollectorConfig(envVars map[string]string) *ElasticCollectorConfig {
	outputDir := envutil.Lookup(envVars, "SR_COLLECTOR_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "./logs"
	}
	// Default to the ECS user_agent.original field so automatic user-agent
	// correlation works out of the box. This is the primary correlation path for
	// the collector (the standard collect config sets only an index); without it
	// buildSearchQuery emits no user-agent clause.
	userAgentField := envutil.Lookup(envVars, "SR_COLLECTOR_USER_AGENT_FIELD")
	if userAgentField == "" {
		userAgentField = "user_agent.original"
	}
	return &ElasticCollectorConfig{
		ElasticsearchURL: envutil.Lookup(envVars, "SR_ELASTIC_URL"),
		CloudID:          envutil.Lookup(envVars, "SR_ELASTIC_CLOUD_ID"),
		APIKey:           envutil.Lookup(envVars, "SR_ELASTIC_API_KEY"),
		OutputDir:        outputDir,
		UserAgentField:   userAgentField,
	}
}
