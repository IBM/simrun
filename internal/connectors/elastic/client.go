// Package elastic is a minimal client for the Elastic Security detection-engine
// API, used to validate Elastic connectors.
package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClientConfig holds Elastic connection configuration.
type ClientConfig struct {
	KibanaURL string
	APIKey    string
}

// Client provides Elastic Security API operations.
type Client struct {
	config ClientConfig
	client *http.Client
}

// NewClient creates a new Elastic integration client.
func NewClient(config ClientConfig) *Client {
	return &Client{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// TestConnection verifies the API key and Kibana URL are valid.
func (c *Client) TestConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/status", c.config.KibanaURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "ApiKey "+c.config.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("authentication failed with status: %d", resp.StatusCode)
	}
	return nil
}

// RuleSummary represents a detection rule from Elastic Security.
type RuleSummary struct {
	ID          string   `json:"id"`
	RuleID      string   `json:"rule_id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Enabled     bool     `json:"enabled"`
	Tags        []string `json:"tags"`
	Severity    string   `json:"severity"`
	RiskScore   int      `json:"risk_score"`
	Type        string   `json:"type"`
	CreatedAt   string   `json:"created_at"`
	UpdatedAt   string   `json:"updated_at"`
}

// ListRulesResponse is the response from the Kibana Detection Engine _find API.
type ListRulesResponse struct {
	Page    int           `json:"page"`
	PerPage int           `json:"perPage"`
	Total   int           `json:"total"`
	Data    []RuleSummary `json:"data"`
}

// ListRules retrieves detection rules from Elastic Security.
// If enabledOnly is true, only enabled rules are returned.
func (c *Client) ListRules(ctx context.Context, page, perPage int, enabledOnly bool) (*ListRulesResponse, error) {
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 100
	}

	url := fmt.Sprintf("%s/api/detection_engine/rules/_find?page=%d&per_page=%d&sort_field=name&sort_order=asc", c.config.KibanaURL, page, perPage)
	if enabledOnly {
		url += "&filter=alert.attributes.enabled:true"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "ApiKey "+c.config.APIKey)
	req.Header.Set("kbn-xsrf", "true")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("list rules failed with status: %d", resp.StatusCode)
	}

	var result ListRulesResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}

// GetRule retrieves a single detection rule by ID.
func (c *Client) GetRule(ctx context.Context, ruleID string) (*RuleSummary, error) {
	url := fmt.Sprintf("%s/api/detection_engine/rules?rule_id=%s", c.config.KibanaURL, ruleID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "ApiKey "+c.config.APIKey)
	req.Header.Set("kbn-xsrf", "true")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("get rule failed with status: %d", resp.StatusCode)
	}

	var result RuleSummary
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
