## MODIFIED Requirements

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

### Requirement: List Assessments
The system SHALL serve `GET /api/assessments` as a paginated, filterable list
ordered by `updated_at DESC`, returning `{assessments, total, page, perPage}`
where `assessments` is the page slice (possibly empty, never `null`). The system
SHALL additionally serve a single assessment by its unique name via
`GET /api/assessments/{name}`, returning its JSON (which already includes the
raw `yaml` field). Query parameters (`page`, `per_page`, `name`, `type`, `since`)
behave as before against `assessments.name`/`.type`.

#### Scenario: Most-recently-updated first
- **WHEN** a client requests `/api/assessments` with no parameters
- **THEN** the response is HTTP 200 with `{assessments, total, page: 1, perPage: 50}` ordered with the most recently updated assessment first

#### Scenario: Fetch by name
- **WHEN** a client requests `/api/assessments/aws-privesc`
- **THEN** the response is the JSON for the assessment named `aws-privesc`, including its raw `yaml` field

#### Scenario: Invalid type rejected
- **WHEN** a client requests `/api/assessments?type=bogus`
- **THEN** the response is HTTP 400 and no rows are returned

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

## RENAMED Requirements

- FROM: `### Requirement: Saved Scenario Resource`
- TO: `### Requirement: Assessment Resource`

- FROM: `### Requirement: List Saved Scenarios`
- TO: `### Requirement: List Assessments`

- FROM: `### Requirement: Delete Scenario Cascades and Reloads Scheduler`
- TO: `### Requirement: Delete Assessment Cascades and Reloads Scheduler`
