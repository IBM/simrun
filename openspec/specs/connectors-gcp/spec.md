# GCP Connector Specification

## Purpose
Type-specific behavior of `gcp`-typed connectors. Supports two auth modes:
**Workload Identity Federation** (preferred, AWS-IRSA-as-identity-source)
and a **legacy** mode that consumes a service-account credentials file
from a linked secret group. WIF is the only mode with a test-connection
implementation today.

## Requirements

### Requirement: Auth Mode Selection
The system SHALL select the auth mode from `config.auth_type`:
`"workload_identity_federation"` for WIF, any other value (including
empty) SHALL be treated as the legacy mode.

#### Scenario: WIF selected
- **WHEN** an operator sets `auth_type: "workload_identity_federation"`
- **THEN** the run-time auth flow uses WIF

#### Scenario: Legacy default
- **WHEN** `auth_type` is empty
- **THEN** the run-time auth flow uses the legacy credentials-file path

### Requirement: WIF Required Fields
The system SHALL require `project_number`, `pool_id`, `provider_id`, and
`service_account_email` in `config` when `auth_type` is WIF.
`project_id` SHALL be optional and is used as `GOOGLE_CLOUD_PROJECT`.

#### Scenario: Missing WIF fields
- **WHEN** `auth_type` is WIF and `provider_id` is empty
- **THEN** WIF credential resolution fails at test-connection or run time

### Requirement: WIF Run-Time Auth
The system SHALL build a GCP external-account credential JSON at run
time, source AWS credentials from the simrun process's default chain
(used as the WIF identity source), set `GOOGLE_CREDENTIALS` to inline
JSON for Terraform, write a temp file and set
`GOOGLE_APPLICATION_CREDENTIALS` to its path for the GCP SDK, and set
`GOOGLE_CLOUD_PROJECT` from `project_id` when provided.

#### Scenario: WIF env injection
- **WHEN** a WIF GCP connector is selected for a run
- **THEN** the run env contains both `GOOGLE_CREDENTIALS` (inline JSON) and `GOOGLE_APPLICATION_CREDENTIALS` (temp file path)

### Requirement: Legacy Run-Time Auth
The system SHALL pass `SR_GCP_CREDENTIALS` from the linked secret group
through to the run environment in legacy mode. Legacy mode SHALL also
forward `SR_GCP_CREDENTIALS_FILE` from `config` if set.

#### Scenario: Legacy key passed through
- **WHEN** a legacy GCP connector has secret `SR_GCP_CREDENTIALS=<json>`
- **THEN** the run env contains `SR_GCP_CREDENTIALS=<json>`

### Requirement: WIF-Only Test Connection
The system SHALL implement `POST /api/connectors/test` for GCP only when
`auth_type` is WIF: it builds the credential JSON, writes it to a temp
file, and runs `gcloud auth print-access-token` as a subprocess. For
non-WIF GCP connectors, the test SHALL return
`{success: false, error: "connection test is only supported for Workload Identity Federation"}`.

#### Scenario: Legacy test rejected
- **WHEN** a client posts a test for a GCP connector with `auth_type: ""`
- **THEN** the response is the WIF-only error

### Requirement: is_default Allowed
The system SHALL allow `is_default = true` on GCP connectors, subject to
the umbrella one-default-per-cloud-type rule.

#### Scenario: Default GCP connector
- **WHEN** no other GCP connector is the default
- **THEN** setting `is_default: true` succeeds
