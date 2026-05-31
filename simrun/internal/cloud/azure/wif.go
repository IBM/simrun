// Package azureauth provides Azure Workload Identity Federation for simrun.
// When an Azure connector is configured with WIF, simrun sets environment
// variables that allow the Azure SDK and Terraform azurerm provider to
// exchange a Kubernetes service account token for Azure credentials.
package azure

import (
	"context"
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// DefaultTokenFile is the standard OIDC token path on EKS. Azure WIF uses
// the Kubernetes service account token projected by IRSA for federation.
const DefaultTokenFile = "/var/run/secrets/eks.amazonaws.com/serviceaccount/token"

// WIFConfig holds the Azure Workload Identity Federation configuration.
type WIFConfig struct {
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	SubscriptionID string `json:"subscription_id"`
	TokenFile      string `json:"token_file"` // defaults to DefaultTokenFile
}

// EffectiveTokenFile returns the token file path, falling back to the default.
func (c WIFConfig) EffectiveTokenFile() string {
	if c.TokenFile != "" {
		return c.TokenFile
	}
	return DefaultTokenFile
}

// TestConnection validates WIF credentials by requesting a token from Azure.
func TestConnection(ctx context.Context, cfg WIFConfig) error {
	cred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
		TenantID:      cfg.TenantID,
		ClientID:      cfg.ClientID,
		TokenFilePath: cfg.EffectiveTokenFile(),
	})
	if err != nil {
		return fmt.Errorf("failed to create Azure credential: %w", err)
	}

	_, err = cred.GetToken(ctx, policy.TokenRequestOptions{
		Scopes: []string{"https://management.azure.com/.default"},
	})
	if err != nil {
		return fmt.Errorf("Azure credential validation failed: %w", err)
	}
	return nil
}

// CredentialEnvVars returns the environment variables needed for both the
// Terraform azurerm provider and the Azure SDK to use workload identity federation.
func CredentialEnvVars(cfg WIFConfig) map[string]string {
	tokenFile := cfg.EffectiveTokenFile()
	return map[string]string{
		// Terraform azurerm provider
		"ARM_USE_OIDC":             "true",
		"ARM_USE_CLI":              "false",
		"ARM_OIDC_TOKEN_FILE_PATH": tokenFile,
		"ARM_TENANT_ID":            cfg.TenantID,
		"ARM_CLIENT_ID":            cfg.ClientID,
		"ARM_SUBSCRIPTION_ID":      cfg.SubscriptionID,
		// Azure SDK
		"AZURE_TENANT_ID":           cfg.TenantID,
		"AZURE_CLIENT_ID":           cfg.ClientID,
		"AZURE_SUBSCRIPTION_ID":     cfg.SubscriptionID,
		"AZURE_FEDERATED_TOKEN_FILE": tokenFile,
	}
}
