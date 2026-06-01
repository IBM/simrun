package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEffectiveTokenFile(t *testing.T) {
	t.Run("uses default when empty", func(t *testing.T) {
		cfg := WIFConfig{TenantID: "t", ClientID: "c"}
		assert.Equal(t, DefaultTokenFile, cfg.EffectiveTokenFile())
	})

	t.Run("uses custom when set", func(t *testing.T) {
		cfg := WIFConfig{TenantID: "t", ClientID: "c", TokenFile: "/custom/path/token"}
		assert.Equal(t, "/custom/path/token", cfg.EffectiveTokenFile())
	})
}

func TestCredentialEnvVars(t *testing.T) {
	cfg := WIFConfig{
		TenantID:       "tenant-123",
		ClientID:       "client-456",
		SubscriptionID: "sub-789",
		TokenFile:      "/custom/token",
	}

	envVars := CredentialEnvVars(cfg)

	// Terraform azurerm provider vars
	assert.Equal(t, "true", envVars["ARM_USE_OIDC"])
	assert.Equal(t, "false", envVars["ARM_USE_CLI"])
	assert.Equal(t, "/custom/token", envVars["ARM_OIDC_TOKEN_FILE_PATH"])
	assert.Equal(t, "tenant-123", envVars["ARM_TENANT_ID"])
	assert.Equal(t, "client-456", envVars["ARM_CLIENT_ID"])
	assert.Equal(t, "sub-789", envVars["ARM_SUBSCRIPTION_ID"])

	// Azure SDK vars
	assert.Equal(t, "tenant-123", envVars["AZURE_TENANT_ID"])
	assert.Equal(t, "client-456", envVars["AZURE_CLIENT_ID"])
	assert.Equal(t, "sub-789", envVars["AZURE_SUBSCRIPTION_ID"])
	assert.Equal(t, "/custom/token", envVars["AZURE_FEDERATED_TOKEN_FILE"])
}

func TestCredentialEnvVars_DefaultTokenFile(t *testing.T) {
	cfg := WIFConfig{
		TenantID:       "t",
		ClientID:       "c",
		SubscriptionID: "s",
	}

	envVars := CredentialEnvVars(cfg)
	assert.Equal(t, DefaultTokenFile, envVars["ARM_OIDC_TOKEN_FILE_PATH"])
	assert.Equal(t, DefaultTokenFile, envVars["AZURE_FEDERATED_TOKEN_FILE"])
}
