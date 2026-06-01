package web

import (
	"context"
	"net/http"
	"strconv"

	"github.com/IBM/simrun/internal/connectors/elastic"
	"github.com/IBM/simrun/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

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

// getElasticClient builds an Elastic client from a persisted connector.
func (h *ConnectorHandlers) getElasticClient(ctx context.Context, connector *db.Connector) (*elastic.Client, error) {
	return h.buildElasticClient(ctx, connector.SecretGroupID, connector.Config)
}
