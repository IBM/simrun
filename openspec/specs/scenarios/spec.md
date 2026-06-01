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

### Requirement: Saved Scenario Resource
The system SHALL persist saved scenarios in `saved_scenarios` with fields
`id` (UUID), `name`, `yaml` (raw text), `type` (`standard` | `explore` | `collect`,
default `standard`), `created_by`, `updated_by`, `created_at`, `updated_at`.

#### Scenario: Create with default type
- **WHEN** a client posts a scenario without specifying `type`
- **THEN** the inserted row has `type = "standard"`

#### Scenario: Invalid type rejected
- **WHEN** a client posts a scenario with `type: "fast"`
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

### Requirement: List Saved Scenarios
The system SHALL serve `GET /api/scenarios` as a paginated, filterable
list ordered by `updated_at DESC`. The response SHALL be a JSON object
`{scenarios, total, page, perPage}` where `scenarios` is the page slice
(possibly empty array, never `null`) and `total` is the row count after
filters but before `LIMIT/OFFSET`.

Query parameters:
- `page` (integer, default `1`, must be `>= 1`).
- `per_page` (integer, default `50`, clamped to `[1, 100]`).
- `name` (string, optional) — case-insensitive substring match against
  `saved_scenarios.name` (`ILIKE %name%`).
- `type` (string, repeatable) — restricts `saved_scenarios.type` to
  the listed values. Allowed values: `standard`, `explore`, `collect`.
  An unrecognized value SHALL return HTTP 400.
- `since` (Go duration string, optional, e.g. `24h`, `168h`) —
  restricts results to `updated_at >= now() - since`. A malformed or
  non-positive duration SHALL return HTTP 400.

#### Scenario: Most-recently-updated first
- **WHEN** a client requests `/api/scenarios` with no parameters
- **THEN** the response is HTTP 200 with `{scenarios, total, page: 1, perPage: 50}` and `scenarios` is ordered with the most recently updated scenario first

#### Scenario: Pagination slice
- **WHEN** a client requests `/api/scenarios?page=2&per_page=25` and there are 60 saved scenarios matching no filters
- **THEN** `total = 60`, `page = 2`, `perPage = 25`, and `scenarios.length` is 25 (rows 26–50 in `updated_at DESC` order)

#### Scenario: Empty page beyond range
- **WHEN** a client requests `page=99` on a table with 10 rows
- **THEN** the response is HTTP 200 with `scenarios: []` and `total: 10`

#### Scenario: Name substring filter
- **WHEN** a client requests `/api/scenarios?name=login`
- **THEN** only scenarios whose `name` contains `"login"` (case-insensitive) are returned, and `total` reflects the filtered count

#### Scenario: Multi-type filter
- **WHEN** a client requests `/api/scenarios?type=standard&type=explore`
- **THEN** the response includes only scenarios whose `type` is `standard` or `explore`

#### Scenario: Invalid type rejected
- **WHEN** a client requests `/api/scenarios?type=bogus`
- **THEN** the response is HTTP 400 and no rows are returned

#### Scenario: Since window filter
- **WHEN** a client requests `/api/scenarios?since=24h`
- **THEN** the response includes only scenarios with `updated_at >= now() - 24h`

#### Scenario: Malformed since rejected
- **WHEN** a client requests `/api/scenarios?since=abc`
- **THEN** the response is HTTP 400

#### Scenario: Combined filters
- **WHEN** a client requests `/api/scenarios?name=ssh&type=explore&since=168h&page=1&per_page=25`
- **THEN** results are scenarios whose name ILIKE `%ssh%` AND type is `explore` AND `updated_at` is within the past week, paginated to the first 25 in `updated_at DESC` order, with `total` reflecting all matches

#### Scenario: per_page clamped to maximum
- **WHEN** a client requests `/api/scenarios?per_page=500`
- **THEN** `perPage` in the response is `100` and at most 100 rows are returned

### Requirement: Update Without Re-Validation
The system SHALL accept `PUT /api/scenarios/{id}` updates that replace
`name`, `type`, and `yaml` atomically. The new YAML SHALL NOT be re-linted
or re-parsed on save; a broken YAML may be saved and will only fail at
run time.

#### Scenario: Save broken YAML
- **WHEN** a client PUTs a scenario with malformed YAML
- **THEN** the response is 204 (or success) and the row is updated

### Requirement: Delete Scenario Cascades and Reloads Scheduler
The system SHALL delete the scenario row on `DELETE /api/scenarios/{id}`,
which cascades to remove any associated schedule via the FK constraint.
The handler SHALL trigger an in-process scheduler reload after the DB write
to evict orphaned cron entries.

#### Scenario: Delete with schedule
- **WHEN** a scenario with a schedule is deleted
- **THEN** the schedule row is removed and the scheduler no longer fires for that scenario

### Requirement: Run Endpoint Is Asynchronous
The system SHALL accept `POST /api/scenarios/run` with a body identifying
either a saved scenario by ID or inline YAML, perform pre-flight setup
(load AppConfig, packs, scenarios; build run env; parse YAML), and start the
run in a detached goroutine. The response SHALL return HTTP 202 with
`{runId: "<uuid>"}` once the `runs` row is inserted.

#### Scenario: Successful run start
- **WHEN** a client posts a valid run request for an existing scenario
- **THEN** the response is HTTP 202 with a `runId`
- **AND** a new `runs` row exists with `status = "running"`

#### Scenario: Pre-flight failure
- **WHEN** the request references a saved scenario that does not exist
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
