package gcp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCredentialConfig(t *testing.T) {
	cfg := WIFConfig{
		ProjectNumber:       "123456789",
		PoolID:              "my-pool",
		ProviderID:          "aws-provider",
		ServiceAccountEmail: "simrun@project.iam.gserviceaccount.com",
	}

	result, err := BuildCredentialConfig(cfg)
	require.NoError(t, err)

	var parsed externalAccountConfig
	err = json.Unmarshal([]byte(result), &parsed)
	require.NoError(t, err)

	assert.Equal(t, "external_account", parsed.Type)
	assert.Equal(t,
		"//iam.googleapis.com/projects/123456789/locations/global/workloadIdentityPools/my-pool/providers/aws-provider",
		parsed.Audience,
	)
	assert.Equal(t, "urn:ietf:params:aws:token-type:aws4_request", parsed.SubjectTokenType)
	assert.Equal(t, "https://sts.googleapis.com/v1/token", parsed.TokenURL)
	assert.Equal(t, "aws1", parsed.CredentialSource.EnvironmentID)
	assert.Equal(t, "http://169.254.169.254/latest/meta-data/placement/availability-zone", parsed.CredentialSource.RegionURL)
	assert.Equal(t, "http://169.254.169.254/latest/meta-data/iam/security-credentials", parsed.CredentialSource.URL)
	assert.Equal(t, "http://169.254.169.254/latest/api/token", parsed.CredentialSource.IMDSv2SessionTokenURL)
	assert.Equal(t,
		"https://iamcredentials.googleapis.com/v1/projects/-/serviceAccounts/simrun@project.iam.gserviceaccount.com:generateAccessToken",
		parsed.ServiceAccountImpersonationURL,
	)
}

func TestBuildCredentialConfig_MissingFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  WIFConfig
	}{
		{
			name: "missing project number",
			cfg:  WIFConfig{PoolID: "pool", ProviderID: "provider", ServiceAccountEmail: "sa@proj.iam.gserviceaccount.com"},
		},
		{
			name: "missing pool ID",
			cfg:  WIFConfig{ProjectNumber: "123", ProviderID: "provider", ServiceAccountEmail: "sa@proj.iam.gserviceaccount.com"},
		},
		{
			name: "missing provider ID",
			cfg:  WIFConfig{ProjectNumber: "123", PoolID: "pool", ServiceAccountEmail: "sa@proj.iam.gserviceaccount.com"},
		},
		{
			name: "missing service account email",
			cfg:  WIFConfig{ProjectNumber: "123", PoolID: "pool", ProviderID: "provider"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildCredentialConfig(tt.cfg)
			assert.Error(t, err)
		})
	}
}
