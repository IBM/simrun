// Package gcp provides GCP SDK helpers for simulation packs.
package gcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/IBM/simrun/pack"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

const (
	// CredentialsEnvVar is the environment variable for GCP credentials JSON.
	CredentialsEnvVar = "SR_GCP_CREDENTIALS"
	// CredentialsFileEnvVar is the environment variable for GCP credentials file path.
	CredentialsFileEnvVar = "SR_GCP_CREDENTIALS_FILE"
	// GoogleApplicationCredentialsEnvVar is the standard GCP SDK environment variable.
	GoogleApplicationCredentialsEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"
	// defaultCredentialsFileName is the default filename for temporary credentials file.
	defaultCredentialsFileName = "gcp-credentials.json"
)

// InitCredentials checks for GCP credentials environment variables and sets up
// GOOGLE_APPLICATION_CREDENTIALS to point to a credentials file.
//
// Returns the path to the credentials file if created, empty string if no credentials were found,
// or an error if the credentials are invalid or cannot be written.
func InitCredentials() (string, error) {
	// Check if credentials file path is already set
	if existingPath := os.Getenv(CredentialsFileEnvVar); existingPath != "" {
		if os.Getenv(GoogleApplicationCredentialsEnvVar) == "" {
			if err := os.Setenv(GoogleApplicationCredentialsEnvVar, existingPath); err != nil {
				return "", fmt.Errorf("failed to set %s: %w", GoogleApplicationCredentialsEnvVar, err)
			}
		}
		return existingPath, nil
	}

	if existingPath := os.Getenv(GoogleApplicationCredentialsEnvVar); existingPath != "" {
		return existingPath, nil
	}

	// Check for credentials JSON
	credentialsJSON := os.Getenv(CredentialsEnvVar)
	if credentialsJSON == "" {
		return "", nil
	}

	// Validate that the credentials are valid JSON
	var credentials map[string]any
	if err := json.Unmarshal([]byte(credentialsJSON), &credentials); err != nil {
		return "", fmt.Errorf("invalid JSON in %s: %w", CredentialsEnvVar, err)
	}

	// Validate required fields for service account credentials
	requiredFields := []string{"type", "project_id", "private_key_id", "private_key", "client_email"}
	for _, field := range requiredFields {
		if _, exists := credentials[field]; !exists {
			return "", fmt.Errorf("missing required field '%s' in GCP credentials", field)
		}
	}

	// Create temporary file for credentials
	tempDir := os.TempDir()
	credentialsPath := filepath.Join(tempDir, defaultCredentialsFileName)

	if err := os.WriteFile(credentialsPath, []byte(credentialsJSON), 0600); err != nil {
		return "", fmt.Errorf("failed to write GCP credentials: %w", err)
	}

	if err := os.Setenv(GoogleApplicationCredentialsEnvVar, credentialsPath); err != nil {
		_ = os.Remove(credentialsPath)
		return "", fmt.Errorf("failed to set %s: %w", GoogleApplicationCredentialsEnvVar, err)
	}

	return credentialsPath, nil
}

// CleanupCredentials removes the temporary credentials file if it was created by InitCredentials.
func CleanupCredentials(credentialsPath string) error {
	if credentialsPath == "" {
		return nil
	}

	tempDir := os.TempDir()
	expectedPath := filepath.Join(tempDir, defaultCredentialsFileName)

	if credentialsPath != expectedPath {
		return nil
	}

	if err := os.Remove(credentialsPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove temporary GCP credentials: %w", err)
	}

	return nil
}

// Credentials returns the default GCP credentials for the current environment.
func Credentials(ctx context.Context) (*google.Credentials, error) {
	creds, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("unable to find default GCP credentials: %w", err)
	}
	return creds, nil
}

// ClientOptions returns Google Cloud client options using standard environment variables.
// Supported environment variables:
//   - GOOGLE_CREDENTIALS: JSON credentials string
//   - GOOGLE_APPLICATION_CREDENTIALS: Path to credentials JSON file
//
// If neither is set, uses Application Default Credentials.
func ClientOptions(ctx context.Context) ([]option.ClientOption, error) {
	var opts []option.ClientOption

	if executionID := pack.ExecutionIDFromContext(ctx); executionID != "" {
		opts = append(opts, option.WithUserAgent(pack.UserAgent(executionID)))
	}

	// Check for inline credentials JSON
	if credsJSON := os.Getenv("GOOGLE_CREDENTIALS"); credsJSON != "" {
		creds, err := google.CredentialsFromJSONWithTypeAndParams(ctx, []byte(credsJSON),
			google.ServiceAccount,
			google.CredentialsParams{Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"}},
		)
		if err != nil {
			return nil, fmt.Errorf("parse GCP credentials JSON: %w", err)
		}
		opts = append(opts, option.WithCredentials(creds))
		return opts, nil
	}

	// GOOGLE_APPLICATION_CREDENTIALS is handled automatically by the SDK
	// Use Application Default Credentials
	return opts, nil
}

// ImpersonateServiceAccount returns a client option that uses impersonated credentials
// for the specified service account. This allows simulations to test privilege escalation
// scenarios by attempting operations with limited permissions.
//
// The caller must have the iam.serviceAccountTokenCreator role on the target service account.
func ImpersonateServiceAccount(ctx context.Context, serviceAccountEmail string, scopes []string) (option.ClientOption, error) {
	if len(scopes) == 0 {
		scopes = []string{"https://www.googleapis.com/auth/cloud-platform"}
	}

	config := impersonate.CredentialsConfig{
		TargetPrincipal: serviceAccountEmail,
		Scopes:          scopes,
	}

	tokenSource, err := impersonate.CredentialsTokenSource(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("impersonate service account %s: %w", serviceAccountEmail, err)
	}

	return option.WithTokenSource(tokenSource), nil
}
