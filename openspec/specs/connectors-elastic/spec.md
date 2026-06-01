# Elastic Connector Specification

## Purpose
Type-specific behavior of `elastic`-typed connectors: configuration
shape, run-time env injection, test connection, and result-export
configuration. Shared CRUD behavior is in `connectors`.

## Requirements

### Requirement: Required Fields
The system SHALL require `config.kibana_url` to be a non-empty string at
create and update time. `cloud_id`, `elasticsearch_url`, `export_enabled`,
and `export_datastream` SHALL be optional.

#### Scenario: Missing kibana_url rejected
- **WHEN** a client creates an elastic connector with empty `kibana_url`
- **THEN** the response is HTTP 400

### Requirement: Linked Secret Group Provides API Key
The system SHALL expect the linked secret group to contain entry
`SR_ELASTIC_API_KEY` carrying a Kibana API key. Other entries in the
secret group SHALL pass through to the run environment via the standard
flat decryption merge.

#### Scenario: API key present
- **WHEN** the linked secret group has `SR_ELASTIC_API_KEY = "abc"`
- **THEN** at run time the run environment contains `SR_ELASTIC_API_KEY = "abc"`

### Requirement: Run-Time Env Injection
The system SHALL inject the connector's `kibana_url`, `cloud_id`, and
`elasticsearch_url` (when populated) as `SR_KIBANA_URL`,
`SR_ELASTIC_CLOUD_ID`, and `SR_ELASTIC_URL` respectively into the
per-run environment. The decrypted API key SHALL be injected as
`SR_ELASTIC_API_KEY`.

#### Scenario: Full env injection
- **WHEN** an elastic connector with `kibana_url=K`, `cloud_id=C`, `elasticsearch_url=U` and a secret group with API key `A` is selected for a run
- **THEN** the run environment contains `SR_KIBANA_URL=K, SR_ELASTIC_CLOUD_ID=C, SR_ELASTIC_URL=U, SR_ELASTIC_API_KEY=A`

### Requirement: Selection by Name Order
The system SHALL select the first enabled `elastic` connector by
alphabetical name order at run time. **Note:** there is no
`is_default` for elastic; multiple enabled elastic connectors result in
deterministic-but-implicit selection. Flagged as a known UX gap.

#### Scenario: Two elastic connectors
- **WHEN** enabled elastic connectors named `prod-cluster` and `staging-cluster` exist
- **THEN** `prod-cluster` is used at run time

### Requirement: is_default Forbidden
The system SHALL reject any create or update that sets `is_default: true`
on an elastic connector with HTTP 400.

#### Scenario: Attempt to set default
- **WHEN** a client posts an elastic connector with `is_default: true`
- **THEN** the response is HTTP 400

### Requirement: Test Connection
The system SHALL implement `POST /api/connectors/test` for `type: "elastic"`
by attempting an authenticated request against `kibana_url` using the
decrypted API key. Failures SHALL be returned in the test response
payload as `{success: false, error}`.

#### Scenario: Successful test
- **WHEN** kibana_url is reachable and the API key authenticates
- **THEN** the response is `{success: true}`

#### Scenario: Bad API key
- **WHEN** the API key is rejected by Kibana
- **THEN** the response is `{success: false, error: "<message>"}`

### Requirement: Optional Result Export
The system SHALL export run results to Elasticsearch only when
`config.export_enabled = true` AND `config.cloud_id` is non-empty. The
target index SHALL be `logs-<datastream>-default`, where `<datastream>`
is `config.export_datastream` if non-empty, otherwise `"asp.results"`.

#### Scenario: Export enabled
- **WHEN** an elastic connector has `export_enabled: true, cloud_id: "c", export_datastream: "myds"`
- **THEN** completed scenario results are indexed into `logs-myds-default`

#### Scenario: Export blocked by missing cloud_id
- **WHEN** `export_enabled: true` but `cloud_id` is empty
- **THEN** export is skipped silently and the run is unaffected

### Requirement: Elastic Rules Endpoints
The system SHALL serve `GET /api/connectors/{id}/elastic/rules` and
`GET /api/connectors/{id}/elastic/rules/{ruleId}` only when the connector
is of type `elastic`, returning HTTP 400 otherwise. The list endpoint
SHALL accept `page` (default 1) and `per_page` (default 100, max 100).
A convenience `GET /api/elastic/rules` SHALL auto-detect the first
enabled elastic connector and return HTTP 404 when none exists.

#### Scenario: Type mismatch on rules
- **WHEN** a client requests `/api/connectors/<aws-id>/elastic/rules`
- **THEN** the response is HTTP 400

#### Scenario: Auto-detect with no elastic connector
- **WHEN** a client requests `/api/elastic/rules` and no enabled elastic connector exists
- **THEN** the response is HTTP 404
