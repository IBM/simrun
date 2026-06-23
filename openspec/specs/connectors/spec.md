# Connectors Specification

## Purpose
Defines the umbrella behavior for all connectors: typed, named, DB-backed
records that bind credentials (via linked secret groups) to remote systems
(SIEMs and clouds). This spec covers shared CRUD, default-per-type
selection, secret linkage, and the test-connection endpoint. Per-type
behavior — what fields each type carries and how it's used at run time —
is in sibling specs (`connectors-elastic`, `connectors-aws`, etc.).

## Requirements

### Requirement: Connector Resource
The system SHALL persist connectors in the `connectors` table with fields:
`id` (UUID), `name` (unique, non-empty), `type`, `description`,
`secret_group_id` (nullable FK with `ON DELETE SET NULL`), `config`
(JSONB), `enabled` (default true), `is_default` (default false),
`created_by`, `updated_by`, `created_at`, `updated_at`.

#### Scenario: Create with required fields
- **WHEN** a client creates a connector with `name`, `type`, and optional `config`
- **THEN** a new row is inserted with a generated UUID and the provided fields

### Requirement: Recognized Connector Types
The system SHALL recognize seven types: `elastic`, `datadog`, `aws`,
`gcp`, `azure`, `kubernetes`, `ssh`. Other type values are stored
without complaint at the storage layer but SHALL fail at run time and
test-connection time.

#### Scenario: Known type
- **WHEN** a client creates a connector with `type: "aws"`
- **THEN** the connector is created and may be set as default

### Requirement: Required Fields on Create
The system SHALL reject `POST /api/connectors` with HTTP 400 when `name`
or `type` is missing or empty.

#### Scenario: Missing name
- **WHEN** a client posts a connector without `name`
- **THEN** the response is HTTP 400

### Requirement: is_default Restricted to Cloud Types
The system SHALL allow `is_default = true` only on connectors of type
`aws`, `gcp`, `azure`, `kubernetes`, or `ssh`. Setting `is_default: true`
on `elastic` or `datadog` connectors SHALL be rejected with HTTP 400.

#### Scenario: Default on elastic rejected
- **WHEN** a client creates an elastic connector with `is_default: true`
- **THEN** the response is HTTP 400

### Requirement: One Default Per Cloud Type
The system SHALL enforce at most one connector with `is_default = true`
per type via a unique partial index (`is_default = true AND type IN (aws, gcp, azure, kubernetes, ssh)`).
Attempting to create or update a second default for a type SHALL return
HTTP 409 with `"another <type> connector is already set as default"`.

#### Scenario: Conflict
- **WHEN** an `aws` connector with `is_default: true` exists and a client creates another `aws` connector with `is_default: true`
- **THEN** the response is HTTP 409

### Requirement: Secret Group Linkage Validated
The system SHALL verify that any `secret_group_id` provided at create or
update time references an existing secret group, returning HTTP 400
otherwise. `secret_group_id` MAY be null.

#### Scenario: Unknown secret group
- **WHEN** a client supplies a `secret_group_id` that does not match any row
- **THEN** the response is HTTP 400 before any DB write

### Requirement: List All Connectors
The system SHALL return all connectors from `GET /api/connectors` ordered
by name, always as a JSON array.

#### Scenario: Empty list
- **WHEN** no connectors exist
- **THEN** the response is `[]`

### Requirement: Update Replaces All Mutable Fields
The system SHALL replace `name`, `description`, `config`,
`secret_group_id`, `enabled`, and `is_default` on `PUT /api/connectors/{id}`.
The `type` field SHALL NOT be changeable. The handler SHALL return HTTP
204 on success.

#### Scenario: Update succeeds
- **WHEN** a client PUTs valid changes to an existing connector
- **THEN** the response is HTTP 204 and all mutable fields reflect the request

### Requirement: Delete Is Idempotent
The system SHALL respond to `DELETE /api/connectors/{id}` with HTTP 204
regardless of whether the connector exists. Cascade behavior is limited
to nulling `connectors.secret_group_id` references in other rows
(unaffected by this delete).

#### Scenario: Delete missing connector
- **WHEN** a client deletes an ID that does not exist
- **THEN** the response is HTTP 204

### Requirement: Test Connection Endpoint
The system SHALL accept `POST /api/connectors/test` with `type` (required)
and optional `secretGroupId` and `config`. The endpoint SHALL be
stateless (no DB writes) and always return HTTP 200 with body
`{"success": true}` or `{"success": false, "error": "<msg>"}`. The HTTP
status SHALL NOT reflect test failure.

#### Scenario: Successful test
- **WHEN** an Elastic connector test succeeds
- **THEN** the response is HTTP 200 with `{"success": true}`

#### Scenario: Failed test
- **WHEN** the test fails (e.g., bad API key)
- **THEN** the response is HTTP 200 with `{"success": false, "error": "..."}`

### Requirement: Test Connection Type Coverage
The system SHALL implement test-connection for `elastic`, `aws`,
`kubernetes`, and the WIF variants of `gcp` and `azure`. For unsupported
types (`datadog`, `ssh`, and legacy `gcp`/`azure` non-WIF), the endpoint
SHALL return `{"success": false, "error": "unsupported connector type: <type>"}`
or a similar "only supported for WIF" message. **Note:** flagged as a
documented gap for `datadog` and `ssh`.

#### Scenario: Datadog test rejected
- **WHEN** a client posts a Datadog test
- **THEN** the response is `{"success": false, "error": "unsupported connector type: datadog"}`

### Requirement: User Attribution
The system SHALL set `created_by` from the session email on
`POST /api/connectors` and `updated_by` on `PUT /api/connectors/{id}`.
When auth is disabled both SHALL be the empty string.

#### Scenario: Authenticated create
- **WHEN** a user `bob@example.com` creates a connector
- **THEN** the row has `created_by = "bob@example.com"`

### Requirement: Run-Time Resolution by Type
The system SHALL select connectors at scenario-run time as follows: for
target slots `aws`, `gcp`, `azure`, `kubernetes`, `ssh` — by name from
the YAML `targets:` block; for the implicit Elastic SIEM connection — the
first enabled `elastic` connector by name order. **Note:** there is no
`is_default` flag for `elastic` connectors and selection by name order
is the contract.

#### Scenario: Target by name
- **WHEN** a scenario has `targets: {aws: "prod-aws"}` and a connector named `prod-aws` with `type=aws, enabled=true` exists
- **THEN** that connector's credentials are resolved into the run environment

### Requirement: Disabled Connectors Not Resolved
The system SHALL exclude connectors with `enabled = false` from default
selection and from `targets:` resolution; a disabled connector named in
`targets:` SHALL fail the run synchronously with HTTP 400.

#### Scenario: Disabled target connector
- **WHEN** a scenario references a disabled connector by name
- **THEN** the run is rejected before any run row is created

### Requirement: Consistent Credential Resolution Across Call Sites
The system SHALL resolve a connector's credentials to the same set of environment variables regardless of whether the resolution is triggered by `POST /api/connectors/test` (test-connection) or by scenario-run execution. There SHALL be exactly one implementation of per-type credential resolution in the codebase; both call sites SHALL invoke it.

#### Scenario: Test-connection and scenario-run agree on credentials
- **WHEN** a connector C of type `aws` is resolved by the test-connection handler, producing env-var map M1
- **AND** the same connector C is resolved during scenario-run execution, producing env-var map M2
- **THEN** M1 and M2 contain the same keys with the same values (modulo time-limited STS session tokens, which may differ between calls but SHALL be produced by the same code path)

#### Scenario: Adding a new connector type updates both call sites at once
- **WHEN** a new connector type is added (e.g., a new cloud provider)
- **THEN** the per-type resolution logic is added in exactly one place
- **AND** both test-connection and scenario-run pick up the new type without requiring per-call-site changes

### Requirement: UI–Backend Connector Type Parity
The connector-administration UI SHALL provide create, edit, and delete capabilities for every connector type recognized by the backend. The per-type configuration form for each recognized type SHALL be implemented in exactly one frontend component file, so that adding a new backend-recognized type requires exactly one frontend file addition plus a registration in the create-dialog and edit-dialog dispatch tables — no other frontend changes SHALL be required for type parity.

#### Scenario: Every recognized type is configurable in the UI
- **WHEN** the backend recognizes connector types `elastic`, `datadog`, `aws`, `gcp`, `azure`, `kubernetes`, `ssh`
- **THEN** the create dialog presents each as a selectable type
- **AND** the edit dialog renders the appropriate form fields when a connector of that type is opened

#### Scenario: Adding a new type is a one-component addition
- **WHEN** a contributor adds a new connector type `X` to the backend
- **THEN** adding UI support requires creating exactly one new component file `web/frontend/src/lib/components/connectors/XConnectorForm.svelte`
- **AND** registering it in the create-dialog dispatch and the edit-dialog dispatch
- **AND** no other frontend changes are required for the type to be fully configurable
