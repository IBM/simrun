package web

import (
	"encoding/json"
	"fmt"
)

// AuthTypeWIF is the auth_type value for Workload Identity Federation connectors.
const AuthTypeWIF = "workload_identity_federation"

// Scenario type constants.
const (
	ScenarioTypeStandard = "standard"
	ScenarioTypeExplore  = "explore"
	ScenarioTypeCollect  = "collect"
)

// validScenarioTypes is the set of valid scenario type values.
var validScenarioTypes = map[string]bool{
	ScenarioTypeStandard: true,
	ScenarioTypeExplore:  true,
	ScenarioTypeCollect:  true,
}

// Request types

type LintRequest struct {
	YAML string `json:"yaml"`
}

type RunRequest struct {
	ScenarioID    string `json:"scenarioId"`
	Parallelism   int    `json:"parallelism,omitempty"`
	ExploreMode   bool   `json:"exploreMode,omitempty"`
	CleanupAlerts bool   `json:"cleanupAlerts,omitempty"`
	Timeout       string `json:"timeout,omitempty"`
}

type SaveScenarioRequest struct {
	Name string `json:"name"`
	Type string `json:"type,omitempty"`
	YAML string `json:"yaml"`
}

type UpdateConfigRequest struct {
	Key   string          `json:"key"`
	Value json.RawMessage `json:"value"`
}

type InstallPackRequest struct {
	Name       string         `json:"name"`
	Type       string         `json:"type"`
	Source     string         `json:"source"`
	Version    string         `json:"version,omitempty"`
	Parameters map[string]any `json:"parameters,omitempty"`
}

type UpdatePackParametersRequest struct {
	Parameters map[string]any `json:"parameters"`
}

// Secret entry in a create/update request.
// Value is plaintext on write; null means keep existing encrypted value on update.
type SecretEntryRequest struct {
	Key   string  `json:"key"`
	Value *string `json:"value"` // nil = keep existing
}

type CreateSecretRequest struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Entries     []SecretEntryRequest  `json:"entries"`
}

type UpdateSecretRequest struct {
	Name        string               `json:"name"`
	Description string               `json:"description"`
	Entries     []SecretEntryRequest  `json:"entries"`
}

// SecretGroupResponse is returned by list/get — values are stripped, only keys returned.
type SecretGroupResponse struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Keys        []string `json:"keys"`
	CreatedBy   string   `json:"createdBy"`
	UpdatedBy   string   `json:"updatedBy"`
	CreatedAt   string   `json:"createdAt"`
	UpdatedAt   string   `json:"updatedAt"`
}

// Schedule request types

type CreateScheduleRequest struct {
	ScenarioID     string `json:"scenarioId"`
	CronExpression string `json:"cronExpression"`
	Enabled        bool   `json:"enabled"`
	Parallelism    int    `json:"parallelism,omitempty"`
}

type UpdateScheduleRequest struct {
	CronExpression string `json:"cronExpression"`
	Enabled        bool   `json:"enabled"`
	Parallelism    int    `json:"parallelism,omitempty"`
}

// Response types

type LintResponse struct {
	Valid     bool             `json:"valid"`
	Scenarios []LintedScenario `json:"scenarios,omitempty"`
	Error     string           `json:"error,omitempty"`
}

type LintedScenario struct {
	Name         string `json:"name"`
	ExecutorType string `json:"executorType"`
	ExecutorName string `json:"executorName"`
	Assertions   int    `json:"assertions"`
}

type RunResponse struct {
	RunID string `json:"runId"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	GoVersion string `json:"goVersion"`
}

// Connector request/response types

type CreateConnectorRequest struct {
	Name          string         `json:"name"`
	Type          string         `json:"type"`
	Description   string         `json:"description"`
	SecretGroupID string         `json:"secretGroupId,omitempty"`
	Config        map[string]any `json:"config"`
	IsDefault     bool           `json:"isDefault,omitempty"`
}

type UpdateConnectorRequest struct {
	Name          string         `json:"name"`
	Description   string         `json:"description"`
	SecretGroupID string         `json:"secretGroupId,omitempty"`
	Config        map[string]any `json:"config"`
	Enabled       bool           `json:"enabled"`
	IsDefault     bool           `json:"isDefault,omitempty"`
}

type TestConnectorRequest struct {
	Type          string         `json:"type"`
	SecretGroupID string         `json:"secretGroupId"`
	Config        map[string]any `json:"config"`
}

type TestConnectorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

// ElasticConnectorConfig is the structure stored in Connector.Config JSONB for elastic type.
type ElasticConnectorConfig struct {
	KibanaURL        string `json:"kibana_url"`
	CloudID          string `json:"cloud_id,omitempty"`
	ElasticsearchURL string `json:"elasticsearch_url,omitempty"`
	ExportEnabled    bool   `json:"export_enabled,omitempty"`
	ExportDatastream string `json:"export_datastream,omitempty"`
}

// KubernetesConnectorConfig is the structure stored in Connector.Config JSONB for kubernetes type.
// The cloud provider (EKS/GKE/AKS) is auto-detected from the referenced cloud connector type.
type KubernetesConnectorConfig struct {
	ClusterName    string `json:"cluster_name"`
	Region         string `json:"region"`
	CloudConnector string `json:"cloud_connector"`           // name of AWS/GCP/Azure connector
	ResourceGroup  string `json:"resource_group,omitempty"`  // AKS only
	Project        string `json:"project,omitempty"`         // GKE only (falls back to GCP connector's project_id)
}

// AzureConnectorConfig is the structure stored in Connector.Config JSONB for azure type.
type AzureConnectorConfig struct {
	AuthType       string `json:"auth_type"`                  // "workload_identity_federation" or "" (legacy service principal)
	TenantID       string `json:"tenant_id"`                  // Azure AD tenant ID
	SubscriptionID string `json:"subscription_id"`            // Azure subscription ID
	ClientID       string `json:"client_id"`                  // Azure AD application (client) ID
	TokenFile      string `json:"token_file,omitempty"`       // OIDC token file path (WIF) — defaults to EKS path
}

// GCPConnectorConfig is the structure stored in Connector.Config JSONB for gcp type.
type GCPConnectorConfig struct {
	AuthType            string `json:"auth_type"`                        // "workload_identity_federation" or "" (legacy service account)
	ProjectID           string `json:"project_id,omitempty"`             // GCP project ID (e.g. "my-project") — injected as GOOGLE_CLOUD_PROJECT
	ProjectNumber       string `json:"project_number,omitempty"`         // GCP project number (WIF)
	PoolID              string `json:"pool_id,omitempty"`                // Workload Identity Pool ID (WIF)
	ProviderID          string `json:"provider_id,omitempty"`            // Workload Identity Provider ID (WIF)
	ServiceAccountEmail string `json:"service_account_email,omitempty"`  // Target service account (WIF)
	CredentialsFile     string `json:"credentials_file,omitempty"`       // Legacy: path to credentials file
}

// SSHConnectorConfig is the JSON payload stored in connectors.config for type="ssh".
// The private key lives in the linked secret group as SR_SSH_KEY.
type SSHConnectorConfig struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Port     int    `json:"port,omitempty"` // default 22 if omitted
}

// Validate returns an error if the SSH connector config is missing required fields.
func (c SSHConnectorConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("ssh connector: host is required")
	}
	if c.Username == "" {
		return fmt.Errorf("ssh connector: username is required")
	}
	if c.Port < 0 || c.Port > 65535 {
		return fmt.Errorf("ssh connector: port must be 0-65535")
	}
	return nil
}

