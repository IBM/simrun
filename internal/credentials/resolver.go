// Package credentials resolves per-connector credentials into the environment-
// variable maps consumed by detonators and CLI tools. Shared between scenario
// execution and the test-connection endpoint so per-cloud resolution lives in
// exactly one place.
// Package credentials resolves connector secret groups into the environment
// variables a run needs.
package credentials

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/IBM/simrun/internal/cloud/aws"
	"github.com/IBM/simrun/internal/cloud/azure"
	"github.com/IBM/simrun/internal/cloud/gcp"
	"github.com/IBM/simrun/internal/crypto"
	"github.com/IBM/simrun/internal/db"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// authTypeWIF is the auth_type value for Workload Identity Federation connectors.
const authTypeWIF = "workload_identity_federation"

// elasticConnectorConfig mirrors the fields of the elastic connector config
// JSON that credential resolution needs. Kept local to avoid an import cycle
// with the web package.
type elasticConnectorConfig struct {
	KibanaURL        string `json:"kibana_url"`
	CloudID          string `json:"cloud_id,omitempty"`
	ElasticsearchURL string `json:"elasticsearch_url,omitempty"`
}

// sshConnectorConfig mirrors the SSH connector config fields needed for
// credential resolution.
type sshConnectorConfig struct {
	Host     string `json:"host"`
	Username string `json:"username"`
	Port     int    `json:"port,omitempty"`
}

// Resolver builds the environment-variable map for a connector by combining
// its config JSON, linked secrets, and cloud-provider-specific credential
// flows (STS, WIF, kubeconfig generation, etc.).
type Resolver struct {
	connectorStore db.ConnectorStore
	secretStore    db.SecretStore
	encryptor      *crypto.Encryptor
}

// NewResolver constructs a Resolver.
func NewResolver(connectorStore db.ConnectorStore, secretStore db.SecretStore, encryptor *crypto.Encryptor) *Resolver {
	return &Resolver{
		connectorStore: connectorStore,
		secretStore:    secretStore,
		encryptor:      encryptor,
	}
}

// Build loads config and secrets from a connector and returns them as a
// credential map ready for environment injection.
func (r *Resolver) Build(ctx context.Context, connector *db.Connector) (map[string]string, error) {
	creds := make(map[string]string)

	// Load config fields
	var cfg map[string]any
	if err := json.Unmarshal(connector.Config, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse connector config: %w", err)
	}

	// Load linked secrets
	var secrets map[string]string
	if connector.SecretGroupID != nil && r.encryptor != nil {
		sg, err := r.secretStore.Get(ctx, *connector.SecretGroupID)
		if err != nil {
			return nil, fmt.Errorf("failed to load secret group: %w", err)
		}
		var entries map[string]string
		if err := json.Unmarshal(sg.Entries, &entries); err != nil {
			return nil, fmt.Errorf("failed to parse secret entries: %w", err)
		}
		secrets = make(map[string]string, len(entries))
		for key, encVal := range entries {
			decrypted, err := r.encryptor.Decrypt(encVal)
			if err != nil {
				return nil, fmt.Errorf("failed to decrypt secret '%s': %w", key, err)
			}
			secrets[key] = decrypted
		}
	}

	switch connector.Type {
	case "aws":
		roleArn, _ := cfg["role_arn"].(string)
		if roleArn != "" {
			// Get external ID from linked secrets
			externalID := ""
			if secrets != nil {
				externalID = secrets["SR_AWS_EXTERNAL_ID"]
			}

			// Assume the role and get temporary credentials
			assumedCreds, err := aws.AssumeRole(ctx, roleArn, externalID)
			if err != nil {
				return nil, fmt.Errorf("failed to assume AWS role %s: %w", roleArn, err)
			}
			for k, v := range assumedCreds {
				creds[k] = v
			}
		}
		// Pass through any additional secrets (e.g., direct AWS keys)
		for key, val := range secrets {
			if key != "SR_AWS_EXTERNAL_ID" {
				creds[key] = val
			}
		}

	case "gcp":
		if projectID := strVal(cfg, "project_id"); projectID != "" {
			creds["GOOGLE_CLOUD_PROJECT"] = projectID
		}
		authType, _ := cfg["auth_type"].(string)
		if authType == authTypeWIF {
			// Resolve AWS credentials for GCP WIF. On EKS with IRSA, the Google
			// external account library doesn't understand AWS_WEB_IDENTITY_TOKEN_FILE,
			// so we resolve via the AWS SDK and pass as env vars to the subprocess.
			awsCreds, err := aws.ResolveCredentials(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve AWS credentials for GCP WIF: %w", err)
			}
			for k, v := range awsCreds {
				creds[k] = v
			}
			wifCfg := gcp.WIFConfig{
				ProjectNumber:       strVal(cfg, "project_number"),
				PoolID:              strVal(cfg, "pool_id"),
				ProviderID:          strVal(cfg, "provider_id"),
				ServiceAccountEmail: strVal(cfg, "service_account_email"),
			}
			credJSON, err := gcp.BuildCredentialConfig(wifCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to build GCP WIF credentials: %w", err)
			}
			// GOOGLE_CREDENTIALS is Terraform-specific (inline JSON)
			creds["GOOGLE_CREDENTIALS"] = credJSON
			// GOOGLE_APPLICATION_CREDENTIALS is used by the GCP SDK's ADC chain (file path)
			credFile, err := gcp.BuildCredentialFile(wifCfg)
			if err != nil {
				return nil, fmt.Errorf("failed to write GCP WIF credential file: %w", err)
			}
			creds["GOOGLE_APPLICATION_CREDENTIALS"] = credFile
		} else {
			// Legacy: service account credentials from secret group
			if gcpCreds, ok := secrets["SR_GCP_CREDENTIALS"]; ok {
				creds["SR_GCP_CREDENTIALS"] = gcpCreds
			}
			if credFile, _ := cfg["credentials_file"].(string); credFile != "" {
				creds["SR_GCP_CREDENTIALS_FILE"] = credFile
			}
		}

	case "azure":
		authType, _ := cfg["auth_type"].(string)
		if authType == authTypeWIF {
			wifCfg := azure.WIFConfig{
				TenantID:       strVal(cfg, "tenant_id"),
				ClientID:       strVal(cfg, "client_id"),
				SubscriptionID: strVal(cfg, "subscription_id"),
				TokenFile:      strVal(cfg, "token_file"),
			}
			for k, v := range azure.CredentialEnvVars(wifCfg) {
				creds[k] = v
			}
		} else {
			// Legacy: service principal with client secret.
			// Set both ARM_* (Terraform azurerm provider) and AZURE_* (Azure SDK) env vars.
			if tenantID, _ := cfg["tenant_id"].(string); tenantID != "" {
				creds["ARM_TENANT_ID"] = tenantID
				creds["AZURE_TENANT_ID"] = tenantID
			}
			if subscriptionID, _ := cfg["subscription_id"].(string); subscriptionID != "" {
				creds["ARM_SUBSCRIPTION_ID"] = subscriptionID
				creds["AZURE_SUBSCRIPTION_ID"] = subscriptionID
			}
			if clientID, _ := cfg["client_id"].(string); clientID != "" {
				creds["ARM_CLIENT_ID"] = clientID
				creds["AZURE_CLIENT_ID"] = clientID
			}
			if clientSecret, ok := secrets["ARM_CLIENT_SECRET"]; ok {
				creds["ARM_CLIENT_SECRET"] = clientSecret
				creds["AZURE_CLIENT_SECRET"] = clientSecret
			}
		}

	case "kubernetes":
		k8sCreds, err := r.buildKubernetesCredentials(ctx, connector)
		if err != nil {
			return nil, err
		}
		for k, v := range k8sCreds {
			creds[k] = v
		}

	case "ssh":
		var sshCfg sshConnectorConfig
		if err := json.Unmarshal(connector.Config, &sshCfg); err != nil {
			return nil, fmt.Errorf("failed to parse ssh connector config: %w", err)
		}

		creds["SR_SSH_HOST"] = sshCfg.Host
		creds["SR_SSH_USERNAME"] = sshCfg.Username
		if sshCfg.Port != 0 {
			creds["SR_SSH_PORT"] = strconv.Itoa(sshCfg.Port)
		}

		if key, ok := secrets["SR_SSH_KEY"]; ok {
			keyPath, err := writeTempSSHKey(key)
			if err != nil {
				return nil, fmt.Errorf("failed to materialize ssh key: %w", err)
			}
			creds["SR_SSH_KEY"] = keyPath
		}
	}

	return creds, nil
}

// BuildTargets resolves cloud credentials from a top-level target map.
// Each entry maps a cloud type (aws, gcp, azure) to a connector name.
// Returns a merged map of all credential env vars.
func (r *Resolver) BuildTargets(ctx context.Context, target map[string]string) (map[string]string, error) {
	if r.connectorStore == nil || len(target) == 0 {
		return nil, nil
	}

	allCreds := make(map[string]string)
	for cloudType, connectorName := range target {
		connector, err := r.connectorStore.GetByName(ctx, connectorName)
		if err != nil {
			return nil, fmt.Errorf("target: connector '%s' for %s not found", connectorName, cloudType)
		}
		if !connector.Enabled {
			return nil, fmt.Errorf("target: connector '%s' for %s is disabled", connectorName, cloudType)
		}
		if connector.Type != cloudType {
			return nil, fmt.Errorf("target: connector '%s' is type '%s', expected '%s'", connectorName, connector.Type, cloudType)
		}

		creds, err := r.Build(ctx, connector)
		if err != nil {
			return nil, fmt.Errorf("target: failed to resolve credentials for connector '%s': %w", connectorName, err)
		}

		for k, v := range creds {
			allCreds[k] = v
		}

		log.WithFields(log.Fields{
			"connector":  connectorName,
			"cloud_type": cloudType,
		}).Debug("Resolved target connector credentials")
	}

	return allCreds, nil
}

// LoadAllSecrets decrypts all secret groups and returns a flat key→value map.
func (r *Resolver) LoadAllSecrets(ctx context.Context) map[string]string {
	if r.secretStore == nil || r.encryptor == nil {
		return nil
	}

	groups, err := r.secretStore.List(ctx)
	if err != nil {
		log.WithError(err).Warn("Failed to load secrets")
		return nil
	}

	result := make(map[string]string)
	for _, g := range groups {
		var entries map[string]string
		if err := json.Unmarshal(g.Entries, &entries); err != nil {
			continue
		}
		for key, encVal := range entries {
			decrypted, err := r.encryptor.Decrypt(encVal)
			if err != nil {
				log.WithField("key", key).WithError(err).Warn("Failed to decrypt secret")
				continue
			}
			result[key] = decrypted
		}
	}
	return result
}

// ResolveElasticEnv resolves the first enabled elastic connector and returns
// SR_KIBANA_URL/SR_ELASTIC_URL/SR_ELASTIC_CLOUD_ID/SR_ELASTIC_API_KEY as a
// runEnv map. Returns nil if no enabled elastic connector exists.
//
// Decryption errors on the linked secret group cause the function to return
// what config it has resolved — the API key just won't be present in the
// returned map (callers will get a clear "missing API key" error from the
// downstream Elastic client).
func (r *Resolver) ResolveElasticEnv(ctx context.Context) map[string]string {
	if r.connectorStore == nil {
		return nil
	}

	connectors, err := r.connectorStore.List(ctx)
	if err != nil {
		log.WithError(err).Warn("Failed to load connectors")
		return nil
	}

	var elastic *db.Connector
	for i := range connectors {
		c := &connectors[i]
		if c.Enabled && c.Type == "elastic" {
			elastic = c
			break
		}
	}
	if elastic == nil {
		return nil
	}

	var cfg elasticConnectorConfig
	if err := json.Unmarshal(elastic.Config, &cfg); err != nil {
		log.WithField("connector", elastic.Name).WithError(err).Warn("Failed to parse elastic connector config")
		return nil
	}

	result := make(map[string]string)
	if cfg.KibanaURL != "" {
		result["SR_KIBANA_URL"] = cfg.KibanaURL
	}
	if cfg.CloudID != "" {
		result["SR_ELASTIC_CLOUD_ID"] = cfg.CloudID
	}
	if cfg.ElasticsearchURL != "" {
		result["SR_ELASTIC_URL"] = cfg.ElasticsearchURL
	}

	if elastic.SecretGroupID != nil && r.encryptor != nil {
		if apiKey, err := r.GetElasticAPIKey(ctx, elastic.SecretGroupID); err != nil {
			log.WithField("connector", elastic.Name).WithError(err).Warn("Failed to load elastic api key")
		} else if apiKey != "" {
			result["SR_ELASTIC_API_KEY"] = apiKey
		}
	}

	return result
}

// GetElasticAPIKey decrypts the SR_ELASTIC_API_KEY from a connector's linked
// secret group. Used by both ResolveElasticEnv and the result-export path.
func (r *Resolver) GetElasticAPIKey(ctx context.Context, secretGroupID *uuid.UUID) (string, error) {
	if secretGroupID == nil {
		return "", fmt.Errorf("secret group is required for Elastic export")
	}
	if r.encryptor == nil {
		return "", fmt.Errorf("encryption is not configured")
	}

	sg, err := r.secretStore.Get(ctx, *secretGroupID)
	if err != nil {
		return "", fmt.Errorf("failed to load secret group: %w", err)
	}

	var entries map[string]string
	if err := json.Unmarshal(sg.Entries, &entries); err != nil {
		return "", fmt.Errorf("failed to parse secret entries: %w", err)
	}

	encryptedKey, ok := entries["SR_ELASTIC_API_KEY"]
	if !ok {
		return "", fmt.Errorf("SR_ELASTIC_API_KEY not found in secret group '%s'", sg.Name)
	}

	return r.encryptor.Decrypt(encryptedKey)
}

// writeTempSSHKey writes a private key to a 0600 temp file and returns the
// absolute path. The OS will reclaim these on reboot, which is acceptable
// for an internal tool. Refining lifecycle is a follow-up.
func writeTempSSHKey(content string) (string, error) {
	f, err := os.CreateTemp("", "simrun-ssh-*.key")
	if err != nil {
		return "", err
	}
	if err := os.Chmod(f.Name(), 0600); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", fmt.Errorf("failed to close temp ssh key: %w", err)
	}
	return f.Name(), nil
}

// strVal extracts a string value from a map, returning "" if not found or not a string.
func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}
