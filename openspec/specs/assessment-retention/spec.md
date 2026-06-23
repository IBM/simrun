# Assessment Retention Specification

## Purpose
Bounds the storage footprint of assessment data by automatically deleting
aged run artifacts. Two independent, admin-configurable retention policies run
as background sweepers: a log-retention policy that prunes per-run JSONL log
files while keeping the `runs` row, and an assessment-retention policy that
purges whole runs (rows, results, log files, and collected NDJSON) once they
age out. Settings live in `AppConfig` and are editable from the assessments
page.

## Requirements

### Requirement: Retention Settings In AppConfig
The system SHALL extend `AppConfig` with retention settings persisted in the
`app_config` table and served by `GET /api/config` / `PUT /api/config`:
`assessment_log_retention_enabled` (bool), `assessment_log_retention_days` (int),
`assessment_retention_enabled` (bool), and `assessment_retention_days` (int).
Defaults SHALL be `assessment_log_retention_enabled = true`,
`assessment_log_retention_days = 7`, `assessment_retention_enabled = false`,
`assessment_retention_days = 30`, backfilled by a migration aligned with
`DefaultAppConfig()`.

#### Scenario: Defaults when unset
- **WHEN** no `app_config` row exists for the retention keys
- **THEN** `GET /api/config` returns `assessment_log_retention_enabled = true`, `assessment_log_retention_days = 7`, `assessment_retention_enabled = false`, and `assessment_retention_days = 30`

#### Scenario: Admin updates retention
- **WHEN** a client sends `PUT /api/config` with `assessment_retention_enabled = true` and `assessment_retention_days = 14`
- **THEN** both values are persisted and returned by a subsequent `GET /api/config`

### Requirement: Retention Settings Validation
The system SHALL reject `PUT /api/config` with HTTP 400 when
`assessment_log_retention_days` or `assessment_retention_days` is less than 1, so
retention cannot be set to a value that deletes data immediately.

#### Scenario: Zero log retention rejected
- **WHEN** a client sends `PUT /api/config` with `assessment_log_retention_days = 0`
- **THEN** the response is HTTP 400 and the stored value is unchanged

#### Scenario: Zero assessment retention rejected
- **WHEN** a client sends `PUT /api/config` with `assessment_retention_days = 0`
- **THEN** the response is HTTP 400 and the stored value is unchanged

### Requirement: Log-Retention Sweeper
The system SHALL run a background sweeper that periodically scans
`<DataDir>/run-logs/` and deletes any `<runID>.jsonl` file whose last
modification time is older than `assessment_log_retention_days`. The sweeper SHALL run
once at startup and then on a fixed 1-hour interval, SHALL re-read `AppConfig`
each tick, and SHALL be a no-op when `assessment_log_retention_enabled = false`.
Deleting a log file SHALL NOT delete or modify the corresponding `runs` row.

#### Scenario: Old log swept
- **WHEN** a run's JSONL file was last modified longer ago than `assessment_log_retention_days` and log retention is enabled
- **THEN** the sweeper deletes the file and leaves the `runs` row intact

#### Scenario: Recent log retained
- **WHEN** a run's JSONL file is newer than `assessment_log_retention_days`
- **THEN** the sweeper leaves the file in place

#### Scenario: Log retention disabled
- **WHEN** `assessment_log_retention_enabled = false`
- **THEN** the log sweeper deletes no files regardless of age

### Requirement: Assessment-Retention Sweeper
The system SHALL run a background sweeper that periodically deletes whole runs
whose `created_at` is older than `assessment_retention_days`. For each expired
run the system SHALL delete the `runs` row (cascading to `scenario_results`),
the run's JSONL log file, and every collected `.ndjson` file referenced by that
run's `scenario_results.collected_log_path`. The sweeper SHALL run once at
startup and then on a fixed 1-hour interval, SHALL re-read `AppConfig` each
tick, SHALL be a no-op when `assessment_retention_enabled = false`, and SHALL
skip runs whose `status` is still `running`.

#### Scenario: Old assessment purged
- **WHEN** a completed run's `created_at` is older than `assessment_retention_days` and assessment retention is enabled
- **THEN** the `runs` row, its `scenario_results`, its JSONL log file, and all of its collected `.ndjson` files are deleted

#### Scenario: Recent assessment retained
- **WHEN** a run's `created_at` is newer than `assessment_retention_days`
- **THEN** the sweeper leaves the run and all its artifacts in place

#### Scenario: Assessment retention disabled
- **WHEN** `assessment_retention_enabled = false`
- **THEN** the assessment sweeper deletes no runs regardless of age

#### Scenario: Running assessment skipped
- **WHEN** a run older than `assessment_retention_days` still has `status = "running"`
- **THEN** the sweeper does not delete it

### Requirement: Swept Logs Surface As Empty
The system SHALL serve `GET /api/runs/{runId}/logs` with HTTP 200 and body `[]`
when the run's JSONL file has been swept by log retention, reusing the existing
missing-file behavior so an expired-log run is indistinguishable from one that
never logged.

#### Scenario: Logs requested after sweep
- **WHEN** a client GETs logs for a run whose JSONL file was deleted by the log sweeper
- **THEN** the response is HTTP 200 with body `[]`

### Requirement: Configure Retention From Assessments Page
The system SHALL present an "Assessment retention" control on the assessments
page that opens a dialog for editing `assessment_log_retention_enabled`,
`assessment_log_retention_days`, `assessment_retention_enabled`, and
`assessment_retention_days`, and SHALL persist changes via `PUT /api/config`.

#### Scenario: Open and save
- **WHEN** an admin opens the dialog, enables assessment retention with a 14-day window, and saves
- **THEN** the new values are sent to `PUT /api/config` and reflected on the page after save

#### Scenario: Invalid input surfaced
- **WHEN** an admin enters a retention period below 1 and saves
- **THEN** the API returns HTTP 400 and the dialog surfaces the error without losing the entered values
