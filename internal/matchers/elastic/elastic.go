// Package elastic matches expected Elastic Security detection alerts.
package elastic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/IBM/simrun/internal/envutil"
	"github.com/sirupsen/logrus"
)

// ElasticSecurityDetectionAlert represents a security detection alert document in Elastic Security
type ElasticSecurityDetectionAlert struct {
	ID        string                 `json:"_id"`
	Index     string                 `json:"_index"`
	Source    map[string]interface{} `json:"_source"`
	Timestamp time.Time              `json:"@timestamp"`
}

// ElasticSecurityDetectionEngineSearchResponse represents the response from Kibana Detection Engine search API
type ElasticSecurityDetectionEngineSearchResponse struct {
	Took int `json:"took"`
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []ElasticSecurityDetectionAlert `json:"hits"`
	} `json:"hits"`
}

type ElasticSecurityDetectionAlertsAPI interface {
	SearchAlerts(query string) ([]ElasticSecurityDetectionAlert, error)
	CloseAlert(id string) error
}

type ElasticSecurityDetectionAlertsAPIImpl struct {
	elasticURL string
	apiKey     string
}

func (m *ElasticSecurityDetectionAlertsAPIImpl) SearchAlerts(query string) ([]ElasticSecurityDetectionAlert, error) {
	url := fmt.Sprintf("%s/api/detection_engine/signals/search", m.elasticURL)

	req, err := http.NewRequest("POST", url, strings.NewReader(query))
	if err != nil {
		return nil, fmt.Errorf("error creating search request: %w", err)
	}

	req.Header.Set("Authorization", "ApiKey "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("kbn-xsrf", "true")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing search request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search failed with status code: %d", resp.StatusCode)
	}

	var searchResp ElasticSecurityDetectionEngineSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("error parsing search response: %w", err)
	}

	return searchResp.Hits.Hits, nil
}

func (m *ElasticSecurityDetectionAlertsAPIImpl) CloseAlert(id string) error {
	url := fmt.Sprintf("%s/api/detection_engine/signals/status", m.elasticURL)

	payload := map[string]interface{}{
		"signal_ids": []string{id},
		"status":     "closed",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("error marshaling close alert payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Authorization", "ApiKey "+m.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("kbn-xsrf", "true")

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error executing close alert request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unable to close alert, got status code: %d", resp.StatusCode)
	}

	return nil
}

// getAPI returns the API client, lazily initializing it if needed.
// This allows matchers to be created without credentials (for lint),
// with credential validation deferred until the API is actually used.
func (m *ElasticSecurityAlertMatcher) getAPI() (ElasticSecurityDetectionAlertsAPI, error) {
	if m.AlertsAPI != nil {
		return m.AlertsAPI, nil
	}

	kibanaURL := envutil.Lookup(m.envVars, "SR_KIBANA_URL")
	if kibanaURL == "" {
		return nil, errors.New("SR_KIBANA_URL environment variable is required")
	}

	apiKey := envutil.Lookup(m.envVars, "SR_ELASTIC_API_KEY")
	if apiKey == "" {
		return nil, errors.New("SR_ELASTIC_API_KEY environment variable is required")
	}

	m.AlertsAPI = &ElasticSecurityDetectionAlertsAPIImpl{
		elasticURL: kibanaURL,
		apiKey:     apiKey,
	}
	return m.AlertsAPI, nil
}

func (m *ElasticSecurityAlertMatcher) HasExpectedAlert(indicators []string, logger *logrus.Entry) (bool, error) {
	logger = m.prepareLogger(logger)

	alerts, err := m.searchAndMatch(indicators, logger)
	if err != nil {
		return false, err
	}

	return alerts != nil, nil
}

func (m *ElasticSecurityAlertMatcher) String() string {
	return fmt.Sprintf("Elastic Security alert '%s'", m.AlertFilter.RuleName)
}

func (m *ElasticSecurityAlertMatcher) MatcherName() string {
	return "Elastic Security alert"
}

func (m *ElasticSecurityAlertMatcher) AlertName() string {
	return m.AlertFilter.RuleName
}

func (m *ElasticSecurityAlertMatcher) Cleanup(indicators []string, logger *logrus.Entry) error {
	logger = m.prepareLogger(logger)

	matchingAlerts, err := m.searchAndMatch(indicators, logger)
	if err != nil {
		return err
	}

	if matchingAlerts == nil {
		return nil
	}

	api, err := m.getAPI()
	if err != nil {
		return err
	}

	for _, alert := range matchingAlerts {
		if err := api.CloseAlert(alert.ID); err != nil {
			return errors.New("unable to close alert " + alert.ID + ": " + err.Error())
		}
		logger.WithField("alert_id", alert.ID).Info("Successfully closed alert")
	}

	return nil
}

func (m *ElasticSecurityAlertMatcher) buildElasticAlertQuery() string {
	type queryStruct struct {
		Size  int                      `json:"size"`
		Query map[string]interface{}   `json:"query"`
		Sort  []map[string]interface{} `json:"sort"`
	}

	must := []map[string]interface{}{
		{"match_phrase": map[string]interface{}{"kibana.alert.rule.name": m.AlertFilter.RuleName}},
	}

	if m.AlertFilter.Severity != "" {
		must = append(must, map[string]interface{}{
			"match_phrase": map[string]interface{}{"kibana.alert.severity": m.AlertFilter.Severity},
		})
	}

	query := queryStruct{
		Size: 1000,
		Query: map[string]interface{}{
			"bool": map[string]interface{}{
				"must": must,
				"filter": []map[string]interface{}{
					{"match_phrase": map[string]interface{}{"kibana.alert.workflow_status": "open"}},
					{"range": map[string]interface{}{"@timestamp": map[string]interface{}{"gte": m.sinceValue()}}},
				},
				"must_not": []map[string]interface{}{
					{"exists": map[string]interface{}{"field": "kibana.alert.building_block_type"}},
				},
			},
		},
		Sort: []map[string]interface{}{
			{"@timestamp": map[string]string{"order": "desc"}},
		},
	}

	queryBytes, _ := json.Marshal(query)
	return string(queryBytes)
}

// sinceValue returns the timestamp filter value for the matcher query.
// Uses the configured since time if set, otherwise falls back to "now-10h"
// for backward compatibility with CLI usage where start time isn't tracked.
func (m *ElasticSecurityAlertMatcher) sinceValue() string {
	if m.since.IsZero() {
		return "now-15m"
	}
	return m.since.UTC().Format(time.RFC3339)
}

func (m *ElasticSecurityAlertMatcher) prepareLogger(logger *logrus.Entry) *logrus.Entry {
	if logger == nil {
		logger = logrus.NewEntry(logrus.StandardLogger())
	}
	return logger.WithFields(logrus.Fields{
		"matcher":   "ElasticSecurityDetectionAlert",
		"rule_name": m.AlertFilter.RuleName,
	})
}

func (m *ElasticSecurityAlertMatcher) searchAndMatch(indicators []string, logger *logrus.Entry) ([]ElasticSecurityDetectionAlert, error) {
	api, err := m.getAPI()
	if err != nil {
		return nil, err
	}

	query := m.buildElasticAlertQuery()
	alerts, err := api.SearchAlerts(query)
	if err != nil {
		return nil, errors.New("unable to search for Elastic Security alerts: " + err.Error())
	}

	logger.WithField("alert_count", len(alerts)).Info("Received alerts from Elastic")

	if len(alerts) == 0 {
		return nil, nil
	}

	var matchingAlerts []ElasticSecurityDetectionAlert
	for i := range alerts {
		if m.alertMatchesExecution(alerts[i], indicators, logger) {
			matchingAlerts = append(matchingAlerts, alerts[i])
		}
	}

	return matchingAlerts, nil
}

// ExploreResult represents a single alert discovered during explore mode.
type ExploreResult struct {
	RuleName string
	AlertID  string
	Severity string
}

// CreateAPIFromEnvVars creates an API client from run-specific environment variables.
// The returned client should be reused across multiple calls to avoid repeated allocations.
func CreateAPIFromEnvVars(envVars map[string]string) (ElasticSecurityDetectionAlertsAPI, error) {
	kibanaURL := envutil.Lookup(envVars, "SR_KIBANA_URL")
	if kibanaURL == "" {
		return nil, errors.New("SR_KIBANA_URL environment variable is required")
	}
	apiKey := envutil.Lookup(envVars, "SR_ELASTIC_API_KEY")
	if apiKey == "" {
		return nil, errors.New("SR_ELASTIC_API_KEY environment variable is required")
	}
	return &ElasticSecurityDetectionAlertsAPIImpl{
		elasticURL: kibanaURL,
		apiKey:     apiKey,
	}, nil
}

// BuildExploreQuery builds the Elasticsearch query for explore mode (all open alerts, no rule name filter).
// Only alerts created after `since` are included so previous runs don't add noise.
func BuildExploreQuery(since time.Time) string {
	type queryStruct struct {
		Size  int                      `json:"size"`
		Query map[string]interface{}   `json:"query"`
		Sort  []map[string]interface{} `json:"sort"`
	}

	query := queryStruct{
		Size: 1000,
		Query: map[string]interface{}{
			"bool": map[string]interface{}{
				"filter": []map[string]interface{}{
					{"match_phrase": map[string]interface{}{"kibana.alert.workflow_status": "open"}},
					{"range": map[string]interface{}{"@timestamp": map[string]interface{}{"gte": since.UTC().Format(time.RFC3339)}}},
				},
				"must_not": []map[string]interface{}{
					{"exists": map[string]interface{}{"field": "kibana.alert.building_block_type"}},
				},
			},
		},
		Sort: []map[string]interface{}{
			{"@timestamp": map[string]string{"order": "desc"}},
		},
	}

	queryBytes, _ := json.Marshal(query)
	return string(queryBytes)
}

// ExploreAlerts queries all open Elastic Security alerts and returns those
// that match any of the provided indicators.
func ExploreAlerts(api ElasticSecurityDetectionAlertsAPI, query string, indicators []string, logger *logrus.Entry) ([]ExploreResult, error) {
	alerts, err := api.SearchAlerts(query)
	if err != nil {
		return nil, fmt.Errorf("unable to search for Elastic Security alerts: %w", err)
	}

	logger.WithField("alert_count", len(alerts)).Info("Explore mode: received alerts from Elastic")

	var results []ExploreResult
	for i := range alerts {
		if alertMatchesIndicators(alerts[i], indicators) {
			ruleName, _ := extractAlertField(alerts[i], "kibana.alert.rule.name")
			severity, _ := extractAlertField(alerts[i], "kibana.alert.severity")
			results = append(results, ExploreResult{
				RuleName: ruleName,
				AlertID:  alerts[i].ID,
				Severity: severity,
			})
		}
	}

	return results, nil
}

// CloseAlerts closes the specified alerts by ID.
func CloseAlerts(api ElasticSecurityDetectionAlertsAPI, alertIDs []string, logger *logrus.Entry) error {
	for _, id := range alertIDs {
		if err := api.CloseAlert(id); err != nil {
			return fmt.Errorf("unable to close alert %s: %w", id, err)
		}
		logger.WithField("alert_id", id).Info("Explore mode: closed alert")
	}
	return nil
}

func alertMatchesIndicators(alert ElasticSecurityDetectionAlert, indicators []string) bool {
	alertBytes, _ := json.Marshal(alert.Source)
	alertString := strings.ToLower(string(alertBytes))

	for _, indicator := range indicators {
		if strings.Contains(alertString, strings.ToLower(indicator)) {
			return true
		}
	}
	return false
}

func extractAlertField(alert ElasticSecurityDetectionAlert, field string) (string, bool) {
	parts := strings.Split(field, ".")
	current := alert.Source
	for i, part := range parts {
		if i == len(parts)-1 {
			if val, ok := current[part].(string); ok {
				return val, true
			}
			return "", false
		}
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return "", false
		}
	}
	return "", false
}

func (m *ElasticSecurityAlertMatcher) alertMatchesExecution(alert ElasticSecurityDetectionAlert, indicators []string, logger *logrus.Entry) bool {
	matched := alertMatchesIndicators(alert, indicators)
	if matched {
		logger.WithField("alert_id", alert.ID).Debug("Found matching alert based on provided indicators")
	}
	return matched
}
