# Azure Connector Specification

## Purpose
Type-specific behavior of `azure`-typed connectors. Supports two auth
modes: **Workload Identity Federation** (federated token from an EKS
service-account token file) and a **legacy** mode using
`tenant_id` / `client_id` / `subscription_id` plus a client secret from
the linked secret group.

## Requirements

### Requirement: Auth Mode Selection
The system SHALL select the auth mode from `config.auth_type`:
`"workload_identity_federation"` for WIF, any other value SHALL be
treated as legacy.

#### Scenario: WIF selected
- **WHEN** `auth_type: "workload_identity_federation"`
- **THEN** WIF env vars are populated at run time

### Requirement: WIF Required Fields
The system SHALL require `tenant_id`, `client_id`, and `subscription_id`
in `config` when `auth_type` is WIF. `token_file` SHALL be optional,
defaulting to `/var/run/secrets/eks.amazonaws.com/serviceaccount/token`
(the EKS IRSA path).

#### Scenario: Default token file path
- **WHEN** a WIF Azure connector has no `token_file`
- **THEN** WIF auth uses the default EKS IRSA path

### Requirement: WIF Env Injection
The system SHALL inject the following env vars in WIF mode:
`ARM_USE_OIDC=true`, `ARM_USE_CLI=false`, `ARM_OIDC_TOKEN_FILE_PATH`,
`ARM_TENANT_ID`, `ARM_CLIENT_ID`, `ARM_SUBSCRIPTION_ID`,
`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_SUBSCRIPTION_ID`,
`AZURE_FEDERATED_TOKEN_FILE`. The duplicated `ARM_*` and `AZURE_*` sets
are required for Terraform and the Azure SDK respectively.

#### Scenario: WIF env complete
- **WHEN** a WIF Azure connector is selected
- **THEN** all the listed `ARM_*` and `AZURE_*` env vars are set in the run env

### Requirement: Legacy Run-Time Auth
The system SHALL inject `tenant_id`, `client_id`, `subscription_id` as
both `ARM_*` and `AZURE_*` env vars in legacy mode. The client secret
SHALL come from the linked secret group's `ARM_CLIENT_SECRET` entry
and SHALL be set as both `ARM_CLIENT_SECRET` and `AZURE_CLIENT_SECRET`.

#### Scenario: Legacy secret resolved
- **WHEN** the secret group has `ARM_CLIENT_SECRET=s` and the connector configures `tenant_id, client_id, subscription_id`
- **THEN** the run env contains both `ARM_CLIENT_SECRET=s` and `AZURE_CLIENT_SECRET=s`

### Requirement: WIF-Only Test Connection
The system SHALL implement `POST /api/connectors/test` for Azure only
when `auth_type` is WIF: it uses
`azidentity.WorkloadIdentityCredential` to request a token for
`https://management.azure.com/.default`. Non-WIF Azure connectors
SHALL return `{success: false, error: "connection test is only supported for Workload Identity Federation"}`.

#### Scenario: WIF test
- **WHEN** a WIF Azure connector tests successfully
- **THEN** the response is `{success: true}`

### Requirement: is_default Allowed
The system SHALL allow `is_default = true` on Azure connectors, subject
to the umbrella one-default-per-cloud-type rule.

#### Scenario: Default Azure connector
- **WHEN** no other Azure connector is the default
- **THEN** setting `is_default: true` succeeds
