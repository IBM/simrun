// Package gcpauth provides GCP Workload Identity Federation for simrun.
// When a GCP connector is configured with WIF, simrun generates an external
// account credential configuration that the Google Cloud SDK uses to exchange
// AWS credentials for GCP access tokens.
package gcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// WIFConfig holds the Workload Identity Federation configuration.
type WIFConfig struct {
	ProjectNumber       string `json:"project_number"`
	PoolID              string `json:"pool_id"`
	ProviderID          string `json:"provider_id"`
	ServiceAccountEmail string `json:"service_account_email"`
}

// externalAccountConfig is the JSON structure expected by the Google Cloud SDK
// for AWS-based workload identity federation.
type externalAccountConfig struct {
	Type                           string           `json:"type"`
	Audience                       string           `json:"audience"`
	SubjectTokenType               string           `json:"subject_token_type"`
	TokenURL                       string           `json:"token_url"`
	CredentialSource               credentialSource `json:"credential_source"`
	ServiceAccountImpersonationURL string           `json:"service_account_impersonation_url,omitempty"`
}

type credentialSource struct {
	EnvironmentID               string `json:"environment_id"`
	RegionURL                   string `json:"region_url"`
	URL                         string `json:"url"`
	RegionalCredVerificationURL string `json:"regional_cred_verification_url"`
	IMDSv2SessionTokenURL       string `json:"imdsv2_session_token_url"`
}

// BuildCredentialConfig generates a Google Cloud external account credential
// configuration JSON for AWS workload identity federation.
// The returned JSON can be set as GOOGLE_CREDENTIALS for the Google SDK to
// automatically exchange AWS credentials for GCP access tokens.
func BuildCredentialConfig(cfg WIFConfig) (string, error) {
	if cfg.ProjectNumber == "" || cfg.PoolID == "" || cfg.ProviderID == "" {
		return "", fmt.Errorf("project_number, pool_id, and provider_id are required for WIF")
	}
	if cfg.ServiceAccountEmail == "" {
		return "", fmt.Errorf("service_account_email is required for WIF")
	}

	audience := fmt.Sprintf(
		"//iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/%s/providers/%s",
		cfg.ProjectNumber, cfg.PoolID, cfg.ProviderID,
	)

	config := externalAccountConfig{
		Type:             "external_account",
		Audience:         audience,
		SubjectTokenType: "urn:ietf:params:aws:token-type:aws4_request",
		TokenURL:         "https://sts.googleapis.com/v1/token",
		CredentialSource: credentialSource{
			EnvironmentID:               "aws1",
			RegionURL:                   "http://169.254.169.254/latest/meta-data/placement/availability-zone",
			URL:                         "http://169.254.169.254/latest/meta-data/iam/security-credentials",
			RegionalCredVerificationURL: "https://sts.{region}.amazonaws.com?Action=GetCallerIdentity&Version=2011-06-15",
			IMDSv2SessionTokenURL:       "http://169.254.169.254/latest/api/token",
		},
		ServiceAccountImpersonationURL: fmt.Sprintf(
			"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/%s:generateAccessToken",
			cfg.ServiceAccountEmail,
		),
	}

	data, err := json.Marshal(config)
	if err != nil {
		return "", fmt.Errorf("failed to marshal WIF config: %w", err)
	}

	return string(data), nil
}

// BuildCredentialFile writes the WIF credential config JSON to a temporary file
// and returns the file path. This is needed because the GCP SDK's ADC chain
// reads GOOGLE_APPLICATION_CREDENTIALS (a file path), not GOOGLE_CREDENTIALS
// (inline JSON which is Terraform-specific).
func BuildCredentialFile(cfg WIFConfig) (string, error) {
	credJSON, err := BuildCredentialConfig(cfg)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(os.TempDir(), "simrun-gcp-creds", cfg.ProjectNumber+"-"+cfg.PoolID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create credential directory: %w", err)
	}

	path := filepath.Join(dir, "wif.json")
	if err := os.WriteFile(path, []byte(credJSON), 0600); err != nil {
		return "", fmt.Errorf("failed to write credential file: %w", err)
	}

	return path, nil
}
