# Scenarios Specification

## Purpose
Manages saved scenario YAML and orchestrates scenario runs. The
`/api/scenarios/*` endpoints back the UI's scenario library and editor, and
`POST /api/scenarios/run` is the single ingress for executing scenarios
(used by both manual UI runs and the in-process scheduler). Scenario
execution is asynchronous: the run record is created and the response
returns immediately; results land in `runs` and `scenario_results` and stream
over the WebSocket.

## Requirements

### Requirement: Assessment Resource
The system SHALL persist saved assessment definitions in the `assessments` table
with fields `id` (UUID), `name` (UNIQUE), `yaml` (raw text), `type`
(`standard` | `explore` | `collect`, default `standard`), `created_by`,
`updated_by`, `created_at`, `updated_at`. An **assessment** is the saved
definition; its `yaml` body contains a `scenarios:` array of individual scenarios
(the per-case vocabulary and YAML schema are unchanged). Because an assessment
serializes to `<name>.yaml`, `name` is unique and serves as a human-addressable
slug.

#### Scenario: Create with default type
- **WHEN** a client posts an assessment without specifying `type`
- **THEN** the inserted row has `type = "standard"`

#### Scenario: Duplicate name rejected
- **WHEN** a client posts an assessment whose `name` already exists
- **THEN** the response is HTTP 409 and no row is inserted

#### Scenario: Invalid type rejected
- **WHEN** a client posts an assessment with `type: "fast"`
- **THEN** the response is HTTP 400 with `"type must be 'standard', 'explore', or 'collect'"`

### Requirement: YAML Stored Verbatim
The system SHALL store the YAML body unchanged on create and update, without
re-formatting, parsing, or validating it on the save path. Validation occurs
on lint and on run.

#### Scenario: Round-trip preservation
- **WHEN** a client posts YAML body `Y1` and later GETs the same scenario
- **THEN** the returned `yaml` field is byte-identical to `Y1`

### Requirement: Lint Endpoint
The system SHALL parse YAML supplied to `POST /api/scenarios/lint` against
the current pack list (loaded from the DB), without persisting anything,
and return either `{valid: true, scenarios: [...]}` or `{valid: false, error}`.

#### Scenario: Valid YAML lints
- **WHEN** a client posts well-formed YAML with parsable scenarios and the referenced packs are installed
- **THEN** the response is `{valid: true, scenarios: [{name, executorType, executorName, assertions}, ...]}`

#### Scenario: Lint surfaces parse error
- **WHEN** a client posts YAML that the parser rejects (e.g., unknown pack)
- **THEN** the response is `{valid: false, error: "<parser message>"}`

### Requirement: List Assessments
The system SHALL serve `GET /api/assessments` as a paginated, filterable list
ordered by `updated_at DESC`, returning `{assessments, total, page, perPage}`
where `assessments` is the page slice (possibly empty, never `null`). The system
SHALL additionally serve a single assessment by its unique name via
`GET /api/assessments/by-name/{name}`, returning its JSON (which already includes the
raw `yaml` field). Query parameters (`page`, `per_page`, `name`, `type`, `since`)
behave as before against `assessments.name`/`.type`.

#### Scenario: Most-recently-updated first
- **WHEN** a client requests `/api/assessments` with no parameters
- **THEN** the response is HTTP 200 with `{assessments, total, page: 1, perPage: 50}` ordered with the most recently updated assessment first

#### Scenario: Fetch by name
- **WHEN** a client requests `/api/assessments/by-name/aws-privesc`
- **THEN** the response is the JSON for the assessment named `aws-privesc`, including its raw `yaml` field

#### Scenario: Invalid type rejected
- **WHEN** a client requests `/api/assessments?type=bogus`
- **THEN** the response is HTTP 400 and no rows are returned

### Requirement: Update Without Re-Validation
The system SHALL accept `PUT /api/scenarios/{id}` updates that replace
`name`, `type`, and `yaml` atomically. The new YAML SHALL NOT be re-linted
or re-parsed on save; a broken YAML may be saved and will only fail at
run time.

#### Scenario: Save broken YAML
- **WHEN** a client PUTs a scenario with malformed YAML
- **THEN** the response is 204 (or success) and the row is updated

### Requirement: Delete Assessment Cascades and Reloads Scheduler
The system SHALL delete the scenario row on `DELETE /api/scenarios/{id}`,
which cascades to remove any associated schedule via the FK constraint.
The handler SHALL trigger an in-process scheduler reload after the DB write
to evict orphaned cron entries.

#### Scenario: Delete with schedule
- **WHEN** a scenario with a schedule is deleted
- **THEN** the schedule row is removed and the scheduler no longer fires for that scenario

### Requirement: Run Endpoint Is Asynchronous
The system SHALL start a run via `POST /api/runs` with `{assessmentId}` in the
body, performing pre-flight setup (load AppConfig, packs, assessment; build run
env; parse YAML) and starting the run in a detached goroutine. The response SHALL
return HTTP 202 with `{runId: "<uuid>"}` once the `runs` row is inserted. A Run is
a top-level resource: it is created at `POST /api/runs` and read/deleted at
`/api/runs/{id}`. (This replaces `POST /api/scenarios/run` body `{scenarioId}` —
the same shape with a renamed path and field. Runs always reference a saved
assessment; there is no inline-YAML run path today.)

#### Scenario: Run a saved assessment
- **WHEN** a client posts `{assessmentId}` to `/api/runs` for an existing assessment
- **THEN** the response is HTTP 202 with a `runId`
- **AND** a new `runs` row exists with `status = "running"` and `assessment_id` set to the posted id

#### Scenario: Pre-flight failure
- **WHEN** the request references a saved assessment that does not exist
- **THEN** the response is HTTP 400 and no `runs` row is created

### Requirement: Run Environment Construction
The system SHALL build a per-run environment map combining: the first
enabled Elastic connector's URL/cloud-id/API-key fields; all decrypted
secret-group entries (flat merge); SSH log directory if
`ssh_logging_enabled`; and target-connector credentials resolved from the
YAML's `targets:` block. The map SHALL NOT mutate process environment
variables; concurrent runs SHALL each have an independent map.

#### Scenario: Concurrent runs are isolated
- **WHEN** two scenarios are running in parallel against different AWS connectors
- **THEN** each run's environment contains only the credentials for its own target

### Requirement: Targets Block Resolution
The system SHALL resolve target-connector references from the YAML
`targets:` block by name. Each named connector MUST exist, MUST be
enabled, and MUST be of a type compatible with the target slot (e.g.,
`targets.aws` requires an AWS connector). A missing, disabled, or
type-mismatched reference SHALL fail the request synchronously with HTTP
400 before any run row is created.

#### Scenario: Missing target connector
- **WHEN** the YAML has `targets: {aws: "nonexistent"}`
- **THEN** the response is HTTP 400 and no run is created

#### Scenario: Disabled target connector
- **WHEN** the named connector exists but `enabled = false`
- **THEN** the response is HTTP 400 and no run is created

### Requirement: Parallelism Override Hierarchy
The system SHALL select per-run parallelism in priority order:
`RunRequest.parallelism` (if > 0), then `AppConfig.parallelism`, then the
hard default of 5.

#### Scenario: AppConfig used by default
- **WHEN** a run request omits `parallelism` and `AppConfig.parallelism = 8`
- **THEN** the run executes with parallelism 8

#### Scenario: Per-run override takes precedence
- **WHEN** a run request has `parallelism: 2` and `AppConfig.parallelism = 8`
- **THEN** the run executes with parallelism 2

### Requirement: Per-Run Timeout Override
The system SHALL accept an optional `timeout` (Go duration string) on the
run request. When set, it overrides each scenario's per-expectation timeout
parsed from YAML.

#### Scenario: Override applies
- **WHEN** the run request has `timeout: "1m"` and the YAML expectation has `timeout: "10m"`
- **THEN** the runner uses 1 minute as the timeout

#### Scenario: Malformed timeout rejected
- **WHEN** the run request has `timeout: "abc"`
- **THEN** the response is HTTP 400

### Requirement: Explore and Cleanup Flags
The system SHALL accept boolean per-run flags `exploreMode` and
`cleanupAlerts` on the run request, overriding any YAML-level defaults.

#### Scenario: Explore mode enabled per run
- **WHEN** the run request has `exploreMode: true`
- **THEN** the runner enters explore mode for the duration of the run regardless of YAML

### Requirement: Default Elastic Connector Selection
The system SHALL use the first enabled Elastic-type connector (ordered by
name) to populate `SR_KIBANA_URL`, `SR_ELASTIC_CLOUD_ID`, `SR_ELASTIC_URL`,
and `SR_ELASTIC_API_KEY` in the run environment. **Note:** there is no
explicit `is_default` flag for Elastic connectors; selection is by name
order. Operators with multiple enabled Elastic connectors should expect
deterministic but possibly surprising selection.

#### Scenario: First-enabled selection
- **WHEN** two enabled Elastic connectors named `prod` and `staging` exist
- **THEN** the run environment is populated from `prod` (alphabetically first)
