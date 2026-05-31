package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"strconv"
	"strings"

	"github.com/IBM/simrun/simrun/internal/cloud/azure"
	"github.com/IBM/simrun/simrun/internal/cloud/k8s"
	"github.com/IBM/simrun/simrun/internal/connectors/elastic"
	"github.com/IBM/simrun/simrun/internal/credentials"
	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/IBM/simrun/simrun/internal/envutil"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ConnectorHandlers provides REST handlers for connector management.
type ConnectorHandlers struct {
	connectorStore db.ConnectorStore
	secretStore    db.SecretStore
	scenarioStore  db.ScenarioStore
	runStore       db.RunStore
	credResolver   *credentials.Resolver
}

// NewConnectorHandlers creates a new ConnectorHandlers instance.
func NewConnectorHandlers(connectorStore db.ConnectorStore, secretStore db.SecretStore, scenarioStore db.ScenarioStore, runStore db.RunStore, credResolver *credentials.Resolver) *ConnectorHandlers {
	return &ConnectorHandlers{
		connectorStore: connectorStore,
		secretStore:    secretStore,
		scenarioStore:  scenarioStore,
		runStore:       runStore,
		credResolver:   credResolver,
	}
}

// isCloudType returns true if the connector type can be flagged as the
// default for its kind (cloud detonation targets plus ssh).
func isCloudType(t string) bool {
	return t == "aws" || t == "gcp" || t == "azure" || t == "kubernetes" || t == "ssh"
}

// HandleListConnectors handles GET /api/connectors
func (h *ConnectorHandlers) HandleListConnectors(w http.ResponseWriter, r *http.Request) {
	connectors, err := h.connectorStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, connectors)
}

// HandleGetConnector handles GET /api/connectors/{id}
func (h *ConnectorHandlers) HandleGetConnector(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid connector ID")
		return
	}

	connector, err := h.connectorStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "connector not found")
		return
	}

	writeJSON(w, http.StatusOK, connector)
}

// HandleCreateConnector handles POST /api/connectors
func (h *ConnectorHandlers) HandleCreateConnector(w http.ResponseWriter, r *http.Request) {
	var req CreateConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}

	if req.IsDefault && !isCloudType(req.Type) {
		writeError(w, http.StatusBadRequest, "default flag is only allowed for cloud connectors (aws, gcp, azure, kubernetes, ssh)")
		return
	}

	var secretGroupID *uuid.UUID
	if req.SecretGroupID != "" {
		id, err := uuid.Parse(req.SecretGroupID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid secret group ID")
			return
		}
		// Validate secret group exists
		if _, err := h.secretStore.Get(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, "secret group not found")
			return
		}
		secretGroupID = &id
	}

	// Validate config structure matches the connector type
	if req.Type == "elastic" {
		var elasticConfig ElasticConnectorConfig
		configBytes, _ := json.Marshal(req.Config)
		if err := json.Unmarshal(configBytes, &elasticConfig); err != nil || elasticConfig.KibanaURL == "" {
			writeError(w, http.StatusBadRequest, "kibana_url is required for elastic connector")
			return
		}
	}

	if req.Type == "kubernetes" {
		var k8sConfig KubernetesConnectorConfig
		configBytes, _ := json.Marshal(req.Config)
		if err := json.Unmarshal(configBytes, &k8sConfig); err != nil || k8sConfig.ClusterName == "" || k8sConfig.Region == "" || k8sConfig.CloudConnector == "" {
			writeError(w, http.StatusBadRequest, "cluster_name, region, and cloud_connector are required for kubernetes connector")
			return
		}
	}

	if req.Type == "ssh" {
		var sshCfg SSHConnectorConfig
		configBytes, _ := json.Marshal(req.Config)
		if err := json.Unmarshal(configBytes, &sshCfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid ssh config")
			return
		}
		if err := sshCfg.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config")
		return
	}

	connector, err := h.connectorStore.Save(r.Context(), req.Name, req.Type, req.Description, secretGroupID, configJSON, req.IsDefault, getUserEmail(r))
	if err != nil {
		if strings.Contains(err.Error(), "idx_connectors_default_per_type") {
			writeError(w, http.StatusConflict, fmt.Sprintf("another %s connector is already set as default", req.Type))
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, connector)
}

// HandleUpdateConnector handles PUT /api/connectors/{id}
func (h *ConnectorHandlers) HandleUpdateConnector(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid connector ID")
		return
	}

	var req UpdateConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	// Verify connector exists
	existing, err := h.connectorStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "connector not found")
		return
	}

	if req.IsDefault && !isCloudType(existing.Type) {
		writeError(w, http.StatusBadRequest, "default flag is only allowed for cloud connectors (aws, gcp, azure)")
		return
	}

	var secretGroupID *uuid.UUID
	if req.SecretGroupID != "" {
		sgID, err := uuid.Parse(req.SecretGroupID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid secret group ID")
			return
		}
		// Validate secret group exists
		if _, err := h.secretStore.Get(r.Context(), sgID); err != nil {
			writeError(w, http.StatusBadRequest, "secret group not found")
			return
		}
		secretGroupID = &sgID
	}

	// Validate config structure matches the connector type
	if existing.Type == "elastic" {
		var elasticConfig ElasticConnectorConfig
		configBytes, _ := json.Marshal(req.Config)
		if err := json.Unmarshal(configBytes, &elasticConfig); err != nil || elasticConfig.KibanaURL == "" {
			writeError(w, http.StatusBadRequest, "kibana_url is required for elastic connector")
			return
		}
	}

	if existing.Type == "kubernetes" {
		var k8sConfig KubernetesConnectorConfig
		configBytes, _ := json.Marshal(req.Config)
		if err := json.Unmarshal(configBytes, &k8sConfig); err != nil || k8sConfig.ClusterName == "" || k8sConfig.Region == "" || k8sConfig.CloudConnector == "" {
			writeError(w, http.StatusBadRequest, "cluster_name, region, and cloud_connector are required for kubernetes connector")
			return
		}
	}

	if existing.Type == "ssh" {
		var sshCfg SSHConnectorConfig
		configBytes, _ := json.Marshal(req.Config)
		if err := json.Unmarshal(configBytes, &sshCfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid ssh config")
			return
		}
		if err := sshCfg.Validate(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config")
		return
	}

	if err := h.connectorStore.Update(r.Context(), id, req.Name, req.Description, secretGroupID, configJSON, req.Enabled, req.IsDefault, getUserEmail(r)); err != nil {
		if strings.Contains(err.Error(), "idx_connectors_default_per_type") {
			writeError(w, http.StatusConflict, fmt.Sprintf("another %s connector is already set as default", existing.Type))
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleDeleteConnector handles DELETE /api/connectors/{id}
func (h *ConnectorHandlers) HandleDeleteConnector(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid connector ID")
		return
	}

	if err := h.connectorStore.Delete(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// HandleTestConnector handles POST /api/connectors/test
// Stateless connection test — validates config + secret group without persisting.
func (h *ConnectorHandlers) HandleTestConnector(w http.ResponseWriter, r *http.Request) {
	var req TestConnectorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Type == "" {
		writeError(w, http.StatusBadRequest, "type is required")
		return
	}

	var secretGroupID *uuid.UUID
	if req.SecretGroupID != "" {
		id, err := uuid.Parse(req.SecretGroupID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid secret group ID")
			return
		}
		secretGroupID = &id
	}

	configJSON, err := json.Marshal(req.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal config")
		return
	}

	testErr := h.testConnection(r.Context(), req.Type, secretGroupID, configJSON)
	if testErr != nil {
		writeJSON(w, http.StatusOK, TestConnectorResponse{Success: false, Error: testErr.Error()})
		return
	}

	writeJSON(w, http.StatusOK, TestConnectorResponse{Success: true})
}

// HandleListElasticRules handles GET /api/connectors/{id}/elastic/rules
func (h *ConnectorHandlers) HandleListElasticRules(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid connector ID")
		return
	}

	connector, err := h.connectorStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "connector not found")
		return
	}

	if connector.Type != "elastic" {
		writeError(w, http.StatusBadRequest, "connector is not of type elastic")
		return
	}

	client, err := h.getElasticClient(r.Context(), connector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	page := 1
	perPage := 100
	if p := r.URL.Query().Get("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if v, err := strconv.Atoi(pp); err == nil && v > 0 && v <= 100 {
			perPage = v
		}
	}

	rules, err := client.ListRules(r.Context(), page, perPage, false)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rules)
}

// HandleGetElasticRule handles GET /api/connectors/{id}/elastic/rules/{ruleId}
func (h *ConnectorHandlers) HandleGetElasticRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid connector ID")
		return
	}

	connector, err := h.connectorStore.Get(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "connector not found")
		return
	}

	if connector.Type != "elastic" {
		writeError(w, http.StatusBadRequest, "connector is not of type elastic")
		return
	}

	client, err := h.getElasticClient(r.Context(), connector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ruleID := chi.URLParam(r, "ruleId")
	rule, err := client.GetRule(r.Context(), ruleID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rule)
}

// HandleListElasticRulesAuto handles GET /api/elastic/rules
// Auto-detects the first enabled Elastic connector and returns its enabled rules.
func (h *ConnectorHandlers) HandleListElasticRulesAuto(w http.ResponseWriter, r *http.Request) {
	connectors, err := h.connectorStore.List(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Find first enabled elastic connector
	var elasticConnector *db.Connector
	for i := range connectors {
		if connectors[i].Type == "elastic" && connectors[i].Enabled {
			elasticConnector = &connectors[i]
			break
		}
	}

	if elasticConnector == nil {
		writeError(w, http.StatusNotFound, "no enabled elastic connector found")
		return
	}

	client, err := h.getElasticClient(r.Context(), elasticConnector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	rules, err := client.ListRules(r.Context(), 1, 10000, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, rules)
}

// HandleRuleCoverage handles GET /api/rules/coverage
// Returns coverage data showing which Elastic rules are covered by saved scenarios.
func (h *ConnectorHandlers) HandleRuleCoverage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Find first enabled elastic connector (same pattern as HandleListElasticRulesAuto)
	connectors, err := h.connectorStore.List(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var elasticConnector *db.Connector
	for i := range connectors {
		if connectors[i].Type == "elastic" && connectors[i].Enabled {
			elasticConnector = &connectors[i]
			break
		}
	}

	if elasticConnector == nil {
		writeError(w, http.StatusNotFound, "no enabled elastic connector found")
		return
	}

	client, err := h.getElasticClient(ctx, elasticConnector)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch all enabled rules from Elastic
	rulesResp, err := client.ListRules(ctx, 1, 10000, true)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// List saved scenarios
	scenarios, err := h.scenarioStore.ListAll(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build rule-name-to-scenarios mapping
	scenarioMap := buildRuleNameToScenariosMap(scenarios)

	// Get latest assertion results
	assertionResults, err := h.runStore.GetLatestAssertionResults(ctx)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Build and return response
	response := buildCoverageResponse(rulesResp.Data, scenarioMap, assertionResults)
	writeJSON(w, http.StatusOK, response)
}

// testConnection validates connectivity for a given connector type. Credential
// resolution is delegated to credentials.Resolver so this path and the
// scenario-run path share one implementation per connector type.
func (h *ConnectorHandlers) testConnection(ctx context.Context, connectorType string, secretGroupID *uuid.UUID, configJSON json.RawMessage) error {
	// Elastic doesn't go through env-var credential resolution — it uses an
	// HTTP client with its own auth path.
	if connectorType == "elastic" {
		client, err := h.buildElasticClient(ctx, secretGroupID, configJSON)
		if err != nil {
			return err
		}
		return client.TestConnection(ctx)
	}

	connector := &db.Connector{
		Type:          connectorType,
		Config:        configJSON,
		SecretGroupID: secretGroupID,
	}
	creds, err := h.credResolver.Build(ctx, connector)
	if err != nil {
		return err
	}

	switch connectorType {
	case "aws":
		return runCLITest(ctx, "aws", []string{"sts", "get-caller-identity"}, creds)
	case "gcp":
		var cfg map[string]any
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("failed to parse GCP config: %w", err)
		}
		if authType, _ := cfg["auth_type"].(string); authType != AuthTypeWIF {
			return fmt.Errorf("connection test is only supported for Workload Identity Federation")
		}
		return runCLITest(ctx, "gcloud", []string{"auth", "print-access-token"}, creds)
	case "azure":
		// azure.TestConnection takes the same WIFConfig fields the resolver
		// already pulled from the config JSON; re-parse here so we don't
		// reach into the resolver's internals.
		var cfg map[string]any
		if err := json.Unmarshal(configJSON, &cfg); err != nil {
			return fmt.Errorf("failed to parse Azure config: %w", err)
		}
		if authType, _ := cfg["auth_type"].(string); authType != AuthTypeWIF {
			return fmt.Errorf("connection test is only supported for Workload Identity Federation")
		}
		azCfg := azure.WIFConfig{
			TenantID:       cfgStr(cfg, "tenant_id"),
			ClientID:       cfgStr(cfg, "client_id"),
			SubscriptionID: cfgStr(cfg, "subscription_id"),
			TokenFile:      cfgStr(cfg, "token_file"),
		}
		return azure.TestConnection(ctx, azCfg)
	case "kubernetes":
		return k8s.ValidateKubeconfig(creds["KUBECONFIG"])
	default:
		return fmt.Errorf("unsupported connector type: %s", connectorType)
	}
}

// cfgStr extracts a string field from a parsed config map, returning "" when
// missing or wrong-typed.
func cfgStr(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

// runCLITest spawns a CLI command with the given credentials as env vars
// and returns an error if the command fails.
func runCLITest(ctx context.Context, name string, args []string, creds map[string]string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = envutil.MergeWithProcessEnv(creds)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s credential validation failed: %w\nOutput: %s", name, err, strings.TrimSpace(string(output)))
	}
	return nil
}

// getElasticClient builds an Elastic client from a persisted connector.
func (h *ConnectorHandlers) getElasticClient(ctx context.Context, connector *db.Connector) (*elastic.Client, error) {
	return h.buildElasticClient(ctx, connector.SecretGroupID, connector.Config)
}

// buildElasticClient creates an Elastic client from config JSON and a secret group reference.
func (h *ConnectorHandlers) buildElasticClient(ctx context.Context, secretGroupID *uuid.UUID, configJSON json.RawMessage) (*elastic.Client, error) {
	var config ElasticConnectorConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("failed to parse elastic config: %w", err)
	}

	if config.KibanaURL == "" {
		return nil, fmt.Errorf("kibana_url is required in connector config")
	}

	apiKey, err := h.credResolver.GetElasticAPIKey(ctx, secretGroupID)
	if err != nil {
		return nil, err
	}

	return elastic.NewClient(elastic.ClientConfig{
		KibanaURL: config.KibanaURL,
		APIKey:    apiKey,
	}), nil
}
