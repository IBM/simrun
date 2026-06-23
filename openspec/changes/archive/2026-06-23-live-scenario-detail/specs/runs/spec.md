## MODIFIED Requirements

### Requirement: Scenario Result Lifecycle
The system SHALL track each scenario as a row in `scenario_results` with
`status` transitioning `pending` → `running` → `completed`, and `phase`
populated during `running` (e.g., `"warmup"`, `"detonating"`, `"matching"`,
`"collecting"`, `"cleanup"`, `"queued"`).

The system SHALL populate the row's executor identity — `executor_name`,
`executor_type`, `execution_id`, and `simulation_id` — as soon as detonation
returns these values, while the row is still `running`, rather than only at
completion. When a detonator does not produce a `simulation_id`, that field
SHALL remain empty without blocking the other identity fields.

While in the `matching` phase, the system SHALL persist per-assertion results
incrementally as the matcher resolves them, so the row's `assertions` reflect
the current passed/pending state before the scenario completes. An assertion
not yet matched SHALL be represented as not-yet-passed (no terminal failure is
recorded until completion).

These incremental writes SHALL NOT alter the terminal completion write: when a
scenario completes, `status` becomes `completed`, `phase` is cleared, and the
final `is_success`, `assertions`, durations, and `discovered_alerts` are
written as they are today.

#### Scenario: Phase transitions
- **WHEN** a scenario enters the matching phase
- **THEN** its `scenario_results` row has `status = "running"` and `phase = "matching"`

#### Scenario: Executor identity exposed during run
- **WHEN** a scenario has detonated and is in the `matching` phase
- **THEN** `GET /api/runs/{runId}` returns that scenario with `status = "running"` and non-empty `executor_name`, `executor_type`, and `execution_id`

#### Scenario: Assertion progress exposed during matching
- **WHEN** a scenario expecting 3 assertions has matched 2 of them and is still matching
- **THEN** the scenario's `assertions` in `GET /api/runs/{runId}` show 2 passed and 1 not-yet-passed while `status = "running"`

#### Scenario: Completion write unchanged
- **WHEN** a scenario finishes after its identity and partial assertions were written mid-run
- **THEN** the final row has `status = "completed"`, `phase = null`, and `is_success` plus the full `assertions` and `discovered_alerts` set, with no stale `running`-phase values
