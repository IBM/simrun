## MODIFIED Requirements

### Requirement: Run Record Resource
The system SHALL persist runs in the `runs` table with fields: `id` (UUID),
`status`, `start_time`, `end_time` (nullable), `total`, `succeeded`, `failed`,
`assessment_id` (nullable FK to `assessments`), `schedule_id` (nullable FK),
`schedule_name` (nullable), `created_by`, `created_at`. A **run** is one
execution of an **assessment** (the saved definition). The `runs` table name is
unchanged; only the definition FK is renamed from `scenario_id` to
`assessment_id`.

#### Scenario: Run created on start
- **WHEN** a run is started
- **THEN** a row is inserted into `runs` with `status = "running"`, `start_time = now()`, `total` set to the number of scenarios to execute, and `succeeded = failed = 0`

### Requirement: Scenario Result Lifecycle
The system SHALL track each scenario as a row in `scenario_results` with
`status` transitioning `pending` → `running` → `completed`, and `phase`
populated during `running` (e.g., `"warmup"`, `"detonating"`, `"matching"`,
`"collecting"`, `"cleanup"`, `"queued"`).

The system SHALL populate the row's executor identity — `executor_name`,
`executor_type`, `execution_id`, and `simulation_id` — as soon as detonation
returns these values, while the row is still `running`. When a detonator does
not produce a `simulation_id`, that field SHALL remain empty without blocking
the other identity fields.

While in the `matching` phase, the system SHALL persist per-expectation results
incrementally as the matcher resolves them, so the row's `expectations` reflect
the current passed/pending state before the scenario completes. An expectation
not yet matched SHALL be represented as not-yet-passed (no terminal failure is
recorded until completion). Each per-expectation outcome is an `ExpectationResult`
(formerly `AssertionResult`).

These incremental writes SHALL NOT alter the terminal completion write: when a
scenario completes, `status` becomes `completed`, `phase` is cleared, and the
final `is_success`, `expectations`, durations, and `discovered_alerts` are written.

#### Scenario: Phase transitions
- **WHEN** a scenario enters the matching phase
- **THEN** its `scenario_results` row has `status = "running"` and `phase = "matching"`

#### Scenario: Expectation progress exposed during matching
- **WHEN** a scenario expecting 3 expectations has matched 2 of them and is still matching
- **THEN** the scenario's `expectations` in `GET /api/runs/{id}` show 2 passed and 1 not-yet-passed while `status = "running"`

#### Scenario: Completion write unchanged
- **WHEN** a scenario finishes after its identity and partial expectations were written mid-run
- **THEN** the final row has `status = "completed"`, `phase = null`, and `is_success` plus the full `expectations` and `discovered_alerts` set

### Requirement: Get Run Returns Composite Object
The system SHALL respond to `GET /api/runs/{id}` with the envelope
`{run, scenarios}` where `run` includes a LEFT JOIN of `assessments.name` and
`.type`, and `scenarios` is the list of `scenario_results` rows for the run.

#### Scenario: Assessment still exists
- **WHEN** a client GETs a run whose `assessment_id` references an existing assessment
- **THEN** `run.assessmentName` and `run.assessmentType` are populated

#### Scenario: Assessment deleted
- **WHEN** the run's source assessment has been deleted
- **THEN** `run.assessmentName` and `run.assessmentType` are null but the run is still returned 200

### Requirement: List Runs Unpaginated
The system SHALL return all runs from `GET /api/runs` ordered by `created_at`
descending, and SHALL serve the runs of a single assessment via the nested
collection `GET /api/assessments/{id}/runs` in the same order (the nested handler
applies the existing `ListRunsFilters` with `assessment_id` set). Runs are
created only at `POST /api/runs`; the nested route is read-only. **Note:** there
is no pagination today; flagged as a known scaling gap.

#### Scenario: List ordering
- **WHEN** a client requests `/api/runs`
- **THEN** the most recent run is first

#### Scenario: Runs scoped to one assessment
- **WHEN** a client requests `/api/assessments/{id}/runs`
- **THEN** only runs whose `assessment_id` equals `{id}` are returned, most recent first

### Requirement: Delete Run Cascades to Results and Log File
The system SHALL delete the `runs` row on `DELETE /api/runs/{id}`, cascade-delete
all `scenario_results` rows via the FK, and best-effort remove the run's on-disk
artifacts: the run's JSONL log file, every collected `.ndjson` file referenced by
the run's `scenario_results.collected_log_path`, and, for each scenario result
with a non-empty `execution_id`, the run's Terraform working directory at
`<DataDir>/terraform/<execution_id>/`. The system SHALL skip Terraform-directory
removal for any `execution_id` that does not resolve to a direct child of
`<DataDir>/terraform/`. Failure to remove any on-disk artifact SHALL be logged
and SHALL NOT fail the request.

#### Scenario: Successful delete
- **WHEN** a client deletes a run with 3 results
- **THEN** the `runs` row, all 3 `scenario_results` rows, and the JSONL log file are removed

#### Scenario: Unsafe execution id skipped
- **WHEN** a scenario result has a blank `execution_id` or one containing a path separator
- **THEN** no Terraform directory is removed for that result and the `<DataDir>/terraform/` base directory is left intact

### Requirement: Get Run Logs
The system SHALL respond to `GET /api/runs/{id}/logs` by reading the JSONL file
from disk and returning the array of entries. A missing file SHALL produce an
empty array `[]`, not an error.

#### Scenario: Missing log file
- **WHEN** a run's log file does not exist on disk
- **THEN** the response is HTTP 200 with body `[]`

### Requirement: Schedule Attribution on Triggered Runs
The system SHALL set `schedule_id` and `schedule_name` on runs created by the
in-process scheduler, and `created_by = "system"`. Manual runs SHALL have
`schedule_id = null`.

#### Scenario: Scheduled run
- **WHEN** the scheduler fires for an assessment
- **THEN** the new `runs` row has `schedule_id` set and `created_by = "system"`

## ADDED Requirements

### Requirement: Single-Scenario Runner With Consolidated Result
The runner SHALL execute exactly one scenario per `Runner` instance and return a
single consolidated `ScenarioResult` value rather than mutating the scenario
input to carry outputs. Fan-out across multiple scenarios SHALL be the sole
responsibility of the parallel executor (worker pool). There SHALL be exactly one
in-memory `ScenarioResult` type shared across the runner, executor, and web
layers (replacing the two former near-identical in-memory types
`runner.ScenarioResult` and `results.ScenarioRunResult`), and one `RunResult`
aggregate (formerly `SimrunRunResult`). The persistence layer keeps its own
column-shaped row DTO (`db.ScenarioResult`, with `json.RawMessage` fields and
DB-only columns) so `db` stays decoupled from the domain packages; the single
marshal boundary that projects the in-memory result onto the row DTO is retained.

#### Scenario: Runner returns a result
- **WHEN** the runner finishes executing a scenario
- **THEN** it returns a populated `ScenarioResult` and the scenario input struct is not used as the output carrier

#### Scenario: Fan-out lives in the executor
- **WHEN** a run contains N scenarios
- **THEN** the parallel executor schedules N single-scenario runner executions and there is no second multi-scenario loop inside `Runner`
