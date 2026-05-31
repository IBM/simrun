package credentials

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/simrun/simrun/internal/crypto"
	"github.com/IBM/simrun/simrun/internal/db"
	"github.com/IBM/simrun/simrun/internal/testutil/fakes"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestResolver wires a Resolver against in-memory fakes + a freshly-generated
// encryption key, returning the stores so a test can insert connectors/secrets.
func newTestResolver(t *testing.T) (*Resolver, *fakes.Stores, *crypto.Encryptor) {
	t.Helper()
	keyPath := filepath.Join(t.TempDir(), "enc.key")
	enc, err := crypto.LoadOrGenerateKey(keyPath)
	require.NoError(t, err)

	stores := fakes.New()
	r := NewResolver(stores.Connector, stores.Secret, enc)
	return r, stores, enc
}

// saveEncryptedSecretGroup encrypts every entry value and persists the group;
// returns its ID for linking to a connector.
func saveEncryptedSecretGroup(t *testing.T, stores *fakes.Stores, enc *crypto.Encryptor, plain map[string]string) uuid.UUID {
	t.Helper()
	encrypted := make(map[string]string, len(plain))
	for k, v := range plain {
		ev, err := enc.Encrypt(v)
		require.NoError(t, err)
		encrypted[k] = ev
	}
	entriesJSON, err := json.Marshal(encrypted)
	require.NoError(t, err)
	sg, err := stores.Secret.Save(context.Background(), "test-sg", "", entriesJSON, "tester")
	require.NoError(t, err)
	return sg.ID
}

func TestResolver_Build_AWS_PassesThroughDirectKeys(t *testing.T) {
	// AWS without role_arn skips STS — secrets flow through as env vars.
	r, stores, enc := newTestResolver(t)
	sgID := saveEncryptedSecretGroup(t, stores, enc, map[string]string{
		"AWS_ACCESS_KEY_ID":     "AKIATEST",
		"AWS_SECRET_ACCESS_KEY": "secret-value",
		"SR_AWS_EXTERNAL_ID":    "ext-id-should-be-stripped",
	})
	conn := &db.Connector{
		Type:          "aws",
		Config:        json.RawMessage(`{}`),
		SecretGroupID: &sgID,
		Enabled:       true,
	}

	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	assert.Equal(t, "AKIATEST", creds["AWS_ACCESS_KEY_ID"])
	assert.Equal(t, "secret-value", creds["AWS_SECRET_ACCESS_KEY"])
	_, externalIDPresent := creds["SR_AWS_EXTERNAL_ID"]
	assert.False(t, externalIDPresent, "SR_AWS_EXTERNAL_ID is consumed, not passed through")
}

func TestResolver_Build_AWS_AssumeRoleSkippedWithoutLiveSTS(t *testing.T) {
	// AssumeRole hits live STS — only run when the user opts in by setting
	// SR_AWS_TEST_ROLE_ARN. Documents the contract: the roleArn branch returns
	// AWS_ACCESS_KEY_ID/SECRET/SESSION_TOKEN when STS is reachable.
	roleArn := os.Getenv("SR_AWS_TEST_ROLE_ARN")
	if roleArn == "" {
		t.Skip("set SR_AWS_TEST_ROLE_ARN to exercise the AssumeRole path")
	}
	r, _, _ := newTestResolver(t)
	conn := &db.Connector{
		Type:    "aws",
		Config:  json.RawMessage(`{"role_arn":"` + roleArn + `"}`),
		Enabled: true,
	}
	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	assert.NotEmpty(t, creds["AWS_ACCESS_KEY_ID"])
	assert.NotEmpty(t, creds["AWS_SECRET_ACCESS_KEY"])
	assert.NotEmpty(t, creds["AWS_SESSION_TOKEN"])
}

func TestResolver_Build_GCP_Legacy(t *testing.T) {
	// Legacy GCP: project_id + credentials_file from config, SR_GCP_CREDENTIALS from secrets.
	r, stores, enc := newTestResolver(t)
	sgID := saveEncryptedSecretGroup(t, stores, enc, map[string]string{
		"SR_GCP_CREDENTIALS": `{"type":"service_account"}`,
	})
	conn := &db.Connector{
		Type:          "gcp",
		Config:        json.RawMessage(`{"project_id":"my-proj","credentials_file":"/path/to/key.json"}`),
		SecretGroupID: &sgID,
		Enabled:       true,
	}

	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	assert.Equal(t, "my-proj", creds["GOOGLE_CLOUD_PROJECT"])
	assert.Equal(t, `{"type":"service_account"}`, creds["SR_GCP_CREDENTIALS"])
	assert.Equal(t, "/path/to/key.json", creds["SR_GCP_CREDENTIALS_FILE"])
	_, hasWIF := creds["GOOGLE_CREDENTIALS"]
	assert.False(t, hasWIF, "legacy path does not emit GOOGLE_CREDENTIALS")
}

func TestResolver_Build_GCP_WIF_SkippedWithoutAWSEnv(t *testing.T) {
	// GCP WIF calls aws.ResolveCredentials which requires an AWS environment
	// (IRSA on EKS, an aws cred file, etc.). Document the contract; skip when
	// the AWS env isn't present.
	if os.Getenv("SR_AWS_TEST_AVAILABLE") == "" {
		t.Skip("set SR_AWS_TEST_AVAILABLE=1 when AWS credentials are configured to exercise GCP WIF")
	}
	r, _, _ := newTestResolver(t)
	conn := &db.Connector{
		Type:    "gcp",
		Config:  json.RawMessage(`{"project_id":"p","auth_type":"workload_identity_federation","project_number":"123","pool_id":"pool","provider_id":"prov","service_account_email":"sa@p.iam.gserviceaccount.com"}`),
		Enabled: true,
	}
	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	assert.NotEmpty(t, creds["GOOGLE_CREDENTIALS"], "WIF emits inline credentials JSON")
	assert.NotEmpty(t, creds["GOOGLE_APPLICATION_CREDENTIALS"], "WIF emits credentials file path")
	assert.Equal(t, "p", creds["GOOGLE_CLOUD_PROJECT"])
}

func TestResolver_Build_Azure_WIF(t *testing.T) {
	// WIF emits ARM_*/AZURE_* env vars from config; no secrets needed.
	r, _, _ := newTestResolver(t)
	conn := &db.Connector{
		Type:    "azure",
		Config:  json.RawMessage(`{"auth_type":"workload_identity_federation","tenant_id":"tenant","client_id":"client","subscription_id":"sub","token_file":"/var/run/token"}`),
		Enabled: true,
	}
	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	// Don't assert the exact set — azure.CredentialEnvVars owns that — but at
	// minimum tenant/client/subscription must appear.
	assert.Equal(t, "tenant", creds["ARM_TENANT_ID"])
	assert.Equal(t, "client", creds["ARM_CLIENT_ID"])
	assert.Equal(t, "sub", creds["ARM_SUBSCRIPTION_ID"])
}

func TestResolver_Build_Azure_Legacy(t *testing.T) {
	// Legacy: service principal config + ARM_CLIENT_SECRET from secrets.
	r, stores, enc := newTestResolver(t)
	sgID := saveEncryptedSecretGroup(t, stores, enc, map[string]string{
		"ARM_CLIENT_SECRET": "shh",
	})
	conn := &db.Connector{
		Type:          "azure",
		Config:        json.RawMessage(`{"tenant_id":"t","client_id":"c","subscription_id":"s"}`),
		SecretGroupID: &sgID,
		Enabled:       true,
	}
	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	// Both ARM_* and AZURE_* must be present — Terraform uses one set, the
	// Azure SDK uses the other.
	assert.Equal(t, "t", creds["ARM_TENANT_ID"])
	assert.Equal(t, "t", creds["AZURE_TENANT_ID"])
	assert.Equal(t, "c", creds["ARM_CLIENT_ID"])
	assert.Equal(t, "c", creds["AZURE_CLIENT_ID"])
	assert.Equal(t, "s", creds["ARM_SUBSCRIPTION_ID"])
	assert.Equal(t, "s", creds["AZURE_SUBSCRIPTION_ID"])
	assert.Equal(t, "shh", creds["ARM_CLIENT_SECRET"])
	assert.Equal(t, "shh", creds["AZURE_CLIENT_SECRET"])
}

func TestResolver_Build_Kubernetes_SkippedWithoutLiveCSPCLI(t *testing.T) {
	// Kubernetes credential build invokes the underlying CSP CLI (aws/gcloud/az)
	// to materialize a kubeconfig. Document the contract; skip when no CSP CLI
	// is available.
	if os.Getenv("SR_K8S_TEST_AVAILABLE") == "" {
		t.Skip("set SR_K8S_TEST_AVAILABLE=1 when a CSP CLI is configured to exercise the kubernetes path")
	}
	r, stores, _ := newTestResolver(t)
	// Pre-seed a cloud connector to be referenced.
	_, err := stores.Connector.Save(context.Background(), "my-aws", "aws", "", nil, json.RawMessage(`{}`), false, "tester")
	require.NoError(t, err)
	conn := &db.Connector{
		Type:    "kubernetes",
		Config:  json.RawMessage(`{"cluster_name":"c","region":"us-east-1","cloud_connector":"my-aws"}`),
		Enabled: true,
	}
	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	assert.NotEmpty(t, creds["KUBECONFIG"])
	assert.Equal(t, creds["KUBECONFIG"], creds["KUBE_CONFIG_PATH"])
}

func TestResolver_Build_SSH_MaterializesKey(t *testing.T) {
	// SSH writes the private key to a 0600 temp file and emits SR_SSH_HOST/USERNAME/KEY.
	r, stores, enc := newTestResolver(t)
	sgID := saveEncryptedSecretGroup(t, stores, enc, map[string]string{
		"SR_SSH_KEY": "-----BEGIN OPENSSH PRIVATE KEY-----\nfake\n-----END OPENSSH PRIVATE KEY-----",
	})
	conn := &db.Connector{
		Type:          "ssh",
		Config:        json.RawMessage(`{"host":"10.0.0.1","username":"ubuntu","port":2222}`),
		SecretGroupID: &sgID,
		Enabled:       true,
	}
	creds, err := r.Build(context.Background(), conn)
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.1", creds["SR_SSH_HOST"])
	assert.Equal(t, "ubuntu", creds["SR_SSH_USERNAME"])
	assert.Equal(t, "2222", creds["SR_SSH_PORT"])

	keyPath := creds["SR_SSH_KEY"]
	require.NotEmpty(t, keyPath, "SR_SSH_KEY must be a file path, not the raw key")
	t.Cleanup(func() { os.Remove(keyPath) })

	info, err := os.Stat(keyPath)
	require.NoError(t, err)
	// 0600 — the key file must not be world- or group-readable.
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
