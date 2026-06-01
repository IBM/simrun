# Schedules Specification

## Purpose
Triggers saved scenarios on a recurring cron schedule via an in-process
scheduler started at server boot. Schedules are 1:1 with saved scenarios
(at most one schedule per scenario), persisted in the `schedules` table,
and use standard Unix cron expressions.

## Requirements

### Requirement: Schedule Resource
The system SHALL persist schedules in the `schedules` table with fields:
`id` (UUID), `scenario_id` (UUID, unique, FK to `saved_scenarios.id`,
`ON DELETE CASCADE`), `cron_expression`, `enabled` (default true),
`parallelism`, `last_run_at` (nullable), `created_at`, `updated_at`.

#### Scenario: One schedule per scenario
- **WHEN** a scenario already has a schedule and a client posts a second `POST /api/schedules` for the same `scenario_id`
- **THEN** the response is HTTP 409 with `"schedule already exists for this scenario"`

#### Scenario: Cascade on scenario delete
- **WHEN** a saved scenario is deleted
- **THEN** its schedule row is removed atomically by the database

### Requirement: Cron Expression Format
The system SHALL accept 5-field standard Unix cron expressions
(minute, hour, day-of-month, month, day-of-week). Predefined keywords
(`@daily`, `@hourly`, etc.) and 6-field formats with seconds SHALL NOT be
accepted. Invalid expressions SHALL be rejected at create or update time
with HTTP 400.

#### Scenario: Valid cron accepted
- **WHEN** a client posts a schedule with `cron_expression="*/15 * * * *"`
- **THEN** the schedule is created

#### Scenario: Invalid cron rejected
- **WHEN** a client posts a schedule with `cron_expression="* * * *"` (4 fields)
- **THEN** the response is HTTP 400 with `"invalid cron expression"`

### Requirement: In-Process Scheduler
The system SHALL run a single in-process cron engine, started before
`ListenAndServe`. On startup it SHALL load all enabled schedules from the DB
and register them. The engine SHALL run in a background goroutine and tick
according to the wall clock of the simrun process.

#### Scenario: Scheduler starts on server boot
- **WHEN** the server starts and the `schedules` table contains 3 enabled rows
- **THEN** the cron engine has 3 registered jobs

### Requirement: Schedule Mutation Triggers Reload
The system SHALL fully rebuild the cron engine's job set on every schedule
create, update, delete, and on every saved-scenario delete. Reload SHALL
happen synchronously after the DB write before the HTTP response.

#### Scenario: Reload on update
- **WHEN** a client PUTs a new `cron_expression` to a schedule
- **THEN** before the response is returned, the cron engine has been reloaded so the next firing uses the new expression

#### Scenario: Reload on scenario delete
- **WHEN** a saved scenario with a schedule is deleted
- **THEN** the cron engine no longer fires for that scenario

### Requirement: Disabled Schedules Are Inactive
The system SHALL load only schedules with `enabled = true` into the cron
engine. Disabling a schedule via update SHALL stop further firings after
the next reload.

#### Scenario: Disabled at boot
- **WHEN** a schedule has `enabled = false` and the server starts
- **THEN** no cron job is registered for it

#### Scenario: Toggle to disabled
- **WHEN** an enabled schedule is updated to `enabled = false`
- **THEN** the reload removes it from the cron engine

### Requirement: Triggered Run Attribution
When a cron job fires, the system SHALL invoke the same scenario-run code
path used for manual runs, with `created_by = "system"` and
`schedule_name = "<scenario.name> (scheduled)"` recorded on the run row.

#### Scenario: Scheduled run row
- **WHEN** a schedule for scenario named `"detect-iam-pivot"` fires
- **THEN** a new `runs` row is inserted with `created_by = "system"` and `schedule_name = "detect-iam-pivot (scheduled)"`

### Requirement: last_run_at Update
After a scheduled run is successfully started, the system SHALL update
`schedules.last_run_at = now()`. If the run cannot be started (scenario not
found, parse error), `last_run_at` SHALL NOT be updated.

#### Scenario: Successful start updates timestamp
- **WHEN** a scheduled run is started and a `runs` row is created
- **THEN** the schedule's `last_run_at` is set to approximately the current time

#### Scenario: Pre-flight failure leaves timestamp unchanged
- **WHEN** a scheduled run fails before any `runs` row is created (e.g., the YAML no longer parses)
- **THEN** `last_run_at` is unchanged

### Requirement: Parallelism Default for Schedules
The system SHALL default schedule `parallelism` to 10 when the field is set
to 0 or a negative value at create or update time. **Note:** this default
differs from `RunRequest` parallelism (which falls through to
`AppConfig.parallelism`); flagged as a known divergence in the current
implementation.

#### Scenario: Default parallelism on create
- **WHEN** a schedule is created with `parallelism: 0`
- **THEN** the persisted row has `parallelism = 10`

### Requirement: Get Schedule by Scenario
`GET /api/scenarios/{scenarioId}/schedule` SHALL return the schedule row for
the given scenario, or HTTP 404 if no schedule exists.

#### Scenario: Schedule exists
- **WHEN** scenario `S` has a schedule and a client requests `/api/scenarios/<S>/schedule`
- **THEN** the response is 200 with the schedule object

#### Scenario: No schedule
- **WHEN** scenario `S` has no schedule
- **THEN** the response is HTTP 404

### Requirement: Listing Schedules
`GET /api/schedules` SHALL return all schedules (enabled and disabled) ordered
by `created_at` descending, always as a JSON array.

#### Scenario: Empty list
- **WHEN** there are no schedules
- **THEN** the response is `[]`

### Requirement: Scheduled Run Uses Background Context
The cron callback SHALL use `context.Background()` for both DB lookups and
the run kick-off, so an in-progress scheduled run is not cancelled by
scheduler shutdown.

#### Scenario: Server shutdown mid-run
- **WHEN** a scheduled run is in progress and the server begins graceful shutdown
- **THEN** the in-flight run continues until completion (subject to its own timeout)

### Requirement: Stale Job on Race
The system SHALL log an error and skip a cron firing whose target scenario
has been deleted between the last reload and the firing, without updating
`last_run_at`. **Note:** the orphaned cron entry remains registered until
the next mutation triggers a reload.

#### Scenario: Fire after scenario deleted
- **WHEN** a cron job fires and `scenarioStore.Get` returns "not found"
- **THEN** no run row is created and `last_run_at` is unchanged
