# Connectors and Secrets

Point SimRun at your platforms and store the credentials securely.

## Connectors vs secret groups

A **connector** (`/connectors`) stores the endpoint and non-secret configuration needed to reach a target platform — for example, a Kibana URL or an AWS role ARN. Connector records are stored in the database and are referenced by name in scenario targets and run defaults.

A **secret group** (`/secrets`) holds the credentials (API keys, passwords, private keys) that a connector uses. A connector links to at most one secret group. Secrets are encrypted at rest using the key in `SR_DATA_DIR` (default `~/.simrun`). See [configuration.md](configuration.md) for how `SR_DATA_DIR` is set.

The split keeps non-sensitive config — URLs, region names, cluster identifiers — visible in the UI while keeping credentials out of plain sight. Secret values are never returned in plaintext after saving.

## SIEM connectors

### Elastic

The `elastic` connector type points SimRun at an Elastic Security deployment. It is the primary SIEM: the Elastic Security alert matcher polls Kibana for detection alerts, and the Elastic Injector writes documents into Elasticsearch.

**Config fields (confirmed in `connector_handlers.go`):**

| Field | Required | Description |
|---|---|---|
| `kibana_url` | Yes | Base URL of your Kibana instance (e.g. `https://kibana.example.com`). |
| `cloud_id` | No | Elastic Cloud ID. Alternative to explicit URLs for Elastic Cloud deployments. |
| `elasticsearch_url` | No | Explicit Elasticsearch URL when not derived from `cloud_id`. |
| `export_enabled` | No | When `true`, run results are exported to Elasticsearch after completion. |
| `export_datastream` | No | Data stream name for result export. |

**Secret group:** Link a secret group that contains `SR_ELASTIC_API_KEY` (the Elasticsearch API key used for all Kibana and Elasticsearch calls).

When no target overrides are specified, SimRun uses the first enabled Elastic connector as the active SIEM connection.

### Datadog

The `datadog` connector type is used by the Datadog security signal matcher to poll for expected signals after detonation.

Datadog credentials are supplied via environment variables (or a linked secret group that injects them into the environment). The matcher resolves them in this order, checking both SimRun-namespaced and native Datadog variable names:

| Purpose | Env var (preferred) | Env var (fallback) |
|---|---|---|
| API key | `SR_DATADOG_API_KEY` | `DD_API_KEY` |
| Application key | `SR_DATADOG_APP_KEY` | `DD_APP_KEY` |
| Site | `SR_DATADOG_SITE` | `DD_SITE` (default: `datadoghq.com`) |

There is no Datadog-specific connector config form; create a connector of type `datadog`, link a secret group that contains the keys above, and SimRun will inject them at run time.

## Cloud connectors

Cloud connectors supply credentials to detonators that execute attack simulations in cloud environments. They are referenced from a scenario's `targets` block by connector name.

### AWS

The `aws` connector resolves credentials for the AWS CLI detonator and Terraform-based simulations that target AWS.

**Config fields:**

| Field | Description |
|---|---|
| `role_arn` | IAM role ARN to assume before running the simulation. When set, SimRun calls `sts:AssumeRole` and passes the resulting temporary credentials to the subprocess. |

**Secret group:** Optionally store `SR_AWS_EXTERNAL_ID` (the external ID required when assuming the role) and any additional AWS credential env vars (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, etc.) for static-key auth.

### GCP

The `gcp` connector resolves credentials for Terraform simulations that target GCP.

**Config fields:**

| Field | Description |
|---|---|
| `auth_type` | Set to `workload_identity_federation` for WIF auth, or leave empty for legacy service account auth. |
| `project_id` | GCP project ID, injected as `GOOGLE_CLOUD_PROJECT`. |
| `project_number` | GCP project number (WIF only). |
| `pool_id` | Workload Identity Pool ID (WIF only). |
| `provider_id` | Workload Identity Provider ID (WIF only). |
| `service_account_email` | Target service account to impersonate (WIF only). |
| `credentials_file` | Path to a service account JSON file (legacy auth only). |

**Secret group:** For legacy auth, store `SR_GCP_CREDENTIALS` (inline service account JSON).

### Azure

The `azure` connector resolves credentials for Terraform simulations that target Azure.

**Config fields:**

| Field | Description |
|---|---|
| `auth_type` | Set to `workload_identity_federation` for WIF auth, or leave empty for legacy service principal auth. |
| `tenant_id` | Azure AD tenant ID. |
| `subscription_id` | Azure subscription ID. |
| `client_id` | Azure AD application (client) ID. |
| `token_file` | OIDC token file path (WIF only; defaults to the EKS IRSA path). |

**Secret group:** For legacy auth, store `ARM_CLIENT_SECRET` (the service principal client secret).

## Other connectors

### Kubernetes

The `kubernetes` connector resolves a kubeconfig for simulations that run Kubernetes-native attack steps. The cloud provider (EKS, GKE, AKS) is auto-detected from the referenced cloud connector type.

**Config fields (all required):**

| Field | Description |
|---|---|
| `cluster_name` | Name of the target cluster. |
| `region` | Cloud region where the cluster is deployed. |
| `cloud_connector` | Name of the AWS, GCP, or Azure connector that provides cloud credentials for fetching the kubeconfig. |
| `resource_group` | Azure resource group (AKS only). |
| `project` | GCP project ID (GKE only; falls back to the linked GCP connector's `project_id`). |

`cluster_name`, `region`, and `cloud_connector` are required. The Kubernetes connector does not have a dedicated secret group; it inherits credentials from the referenced cloud connector.

### SSH

The `ssh` connector is used by the SimRun detonator to execute Terraform simulations on a remote host over SSH.

**Config fields:**

| Field | Required | Description |
|---|---|---|
| `host` | Yes | Hostname or IP address of the remote target. |
| `username` | Yes | SSH username. |
| `port` | No | SSH port (default: 22). |

**Secret group:** Store `SR_SSH_KEY` (the PEM-encoded private key). The key is written to a temporary file and passed to the SSH client at runtime.

## See also

- [walkthrough.md](walkthrough.md) — end-to-end example including connector setup
- [packs.md](packs.md) — install and configure simulation packs
- [scenarios.md](scenarios.md) — reference connector names in scenario targets
