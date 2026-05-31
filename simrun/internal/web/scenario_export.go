package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/simrun/simrun/internal/credentials"
	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/IBM/simrun/simrun/internal/results"
	"github.com/elastic/go-elasticsearch/v9"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// ResultExporter pushes scenario results to external backends. Dispatch is
// keyed on connector type — today only "elastic" is supported, but adding a
// new case (splunk, datadog, …) is mechanical because the call site already
// has a single-method API.
type ResultExporter struct {
	connectorStore db.ConnectorStore
	creds          *credentials.Resolver
}

// NewResultExporter constructs a ResultExporter.
func NewResultExporter(connectorStore db.ConnectorStore, creds *credentials.Resolver) *ResultExporter {
	return &ResultExporter{
		connectorStore: connectorStore,
		creds:          creds,
	}
}

// Export iterates enabled connectors and dispatches to the matching backend.
func (e *ResultExporter) Export(ctx context.Context, runID uuid.UUID, scenarioResults []results.ScenarioRunResult) {
	if e.connectorStore == nil || len(scenarioResults) == 0 {
		return
	}

	connectors, err := e.connectorStore.List(ctx)
	if err != nil {
		log.WithError(err).Warn("Failed to load connectors for export")
		return
	}

	for i := range connectors {
		c := &connectors[i]
		if !c.Enabled {
			continue
		}
		switch c.Type {
		case "elastic":
			e.exportToElastic(ctx, c, runID, scenarioResults)
		}
	}
}

// exportToElastic indexes results into the configured datastream on an enabled
// Elastic connector. No-op if export is disabled on the connector.
func (e *ResultExporter) exportToElastic(ctx context.Context, connector *db.Connector, runID uuid.UUID, scenarioResults []results.ScenarioRunResult) {
	var cfg ElasticConnectorConfig
	if err := json.Unmarshal(connector.Config, &cfg); err != nil {
		return
	}

	if !cfg.ExportEnabled || cfg.CloudID == "" {
		return
	}

	datastream := cfg.ExportDatastream
	if datastream == "" {
		datastream = "asp.results"
	}
	indexName := fmt.Sprintf("logs-%s-default", datastream)

	apiKey, err := e.creds.GetElasticAPIKey(ctx, connector.SecretGroupID)
	if err != nil {
		log.WithFields(log.Fields{
			"connector": connector.Name,
			"run_id":    runID,
		}).WithError(err).Error("Failed to get API key for Elastic export")
		return
	}

	client, err := elasticsearch.New(
		elasticsearch.WithCloudID(cfg.CloudID),
		elasticsearch.WithAPIKey(apiKey),
	)
	if err != nil {
		log.WithFields(log.Fields{
			"connector": connector.Name,
			"run_id":    runID,
		}).WithError(err).Error("Failed to create Elasticsearch client for export")
		return
	}

	indexed := e.indexResults(ctx, client, indexName, runID.String(), scenarioResults)
	log.WithFields(log.Fields{
		"connector": connector.Name,
		"run_id":    runID,
		"index":     indexName,
		"indexed":   indexed,
		"total":     len(scenarioResults),
	}).Info("Exported scenario results to Elasticsearch")
}

// indexResults indexes each scenario result as a document in Elasticsearch.
// It continues on individual failures and returns the count of successfully indexed documents.
func (e *ResultExporter) indexResults(ctx context.Context, client *elasticsearch.Client, indexName, runID string, scenarioResults []results.ScenarioRunResult) int {
	indexed := 0
	for _, scenario := range scenarioResults {
		var assertions []map[string]interface{}
		for _, a := range scenario.Assertions {
			assertions = append(assertions, map[string]interface{}{
				"matcher_type": a.MatcherName(),
				"alert_name":   a.AlertName(),
			})
		}

		doc := map[string]interface{}{
			"@timestamp":    scenario.TimeExecuted.UTC(),
			"run_id":        runID,
			"name":          scenario.Name,
			"is_success":    scenario.Success,
			"error":         scenario.ErrorMessage,
			"duration":      scenario.DurationSeconds,
			"executed_at":   scenario.TimeExecuted.UTC(),
			"executor":      scenario.ExecutorName,
			"executor_type": scenario.ExecutorType,
			"execution_id":  scenario.ExecutionId,
			"assertions":    assertions,
		}

		if scenario.Metadata != nil {
			doc["metadata"] = map[string]interface{}{
				"name":        scenario.Metadata.Name,
				"description": scenario.Metadata.Description,
			}
		} else {
			doc["metadata"] = map[string]interface{}{
				"name": "undefined",
			}
		}

		docBytes, err := json.Marshal(doc)
		if err != nil {
			log.WithField("scenario", scenario.Name).WithError(err).Warn("Failed to marshal scenario result for export")
			continue
		}

		res, err := client.Index(
			indexName,
			bytes.NewReader(docBytes),
			client.Index.WithContext(ctx),
		)
		if err != nil {
			log.WithField("scenario", scenario.Name).WithError(err).Warn("Failed to index scenario result")
			continue
		}
		res.Body.Close()

		if res.IsError() {
			log.WithFields(log.Fields{
				"scenario": scenario.Name,
				"status":   res.Status(),
			}).Warn("Elasticsearch returned error indexing scenario result")
			continue
		}

		indexed++
	}
	return indexed
}
