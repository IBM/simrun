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
`run_log_retention_enabled` (bool), `run_log_retention_days` (int),
`run_retention_enabled` (bool), and `run_retention_days` (int). These keys gate
the deletion of **runs** (executions) and their logs; they are renamed from the
former `assessment_log_retention_*` / `assessment_retention_*` keys (which the
new vocabulary makes misleading, since "assessment" now denotes the definition).
Defaults SHALL be `run_log_retention_enabled = true`, `run_log_retention_days = 7`,
`run_retention_enabled = false`, `run_retention_days = 30`, backfilled by a
migration that also carries forward any operator-set values from the previous key
names.

#### Scenario: Defaults when unset
- **WHEN** no `app_config` row exists for the retention keys
- **THEN** `GET /api/config` returns `run_log_retention_enabled = true`, `run_log_retention_days = 7`, `run_retention_enabled = false`, and `run_retention_days = 30`

#### Scenario: Prior values migrated
- **WHEN** the database held `assessment_retention_days = 14` under the old key name before this change
- **THEN** after migration `run_retention_days = 14` and the old key is removed

#### Scenario: Admin updates retention
- **WHEN** a client sends `PUT /api/config` with `run_retention_enabled = true` and `run_retention_days = 14`
- **THEN** both values are persisted and returned by a subsequent `GET /api/config`

### Requirement: Retention Settings Validation
The system SHALL reject `PUT /api/config` with HTTP 400 when
`run_log_retention_days` or `run_retention_days` is less than 1, so retention
cannot be set to a value that deletes data immediately.

#### Scenario: Zero log retention rejected
- **WHEN** a client sends `PUT /api/config` with `run_log_retention_days = 0`
- **THEN** the response is HTTP 400 and the stored value is unchanged

#### Scenario: Zero run retention rejected
- **WHEN** a client sends `PUT /api/config` with `run_retention_days = 0`
- **THEN** the response is HTTP 400 and the stored value is unchanged

### Requirement: Log-Retention Sweeper
The system SHALL run a background sweeper that periodically scans
`<DataDir>/run-logs/` and deletes any `<id>.jsonl` file whose last modification
time is older than `run_log_retention_days`. The sweeper SHALL run once at
startup and then on a fixed 1-hour interval, SHALL re-read `AppConfig` each tick,
and SHALL be a no-op when `run_log_retention_enabled = false`. Deleting a log file
SHALL NOT delete or modify the corresponding `runs` row.

#### Scenario: Old log swept
- **WHEN** a run's JSONL file was last modified longer ago than `run_log_retention_days` and log retention is enabled
- **THEN** the sweeper deletes the file and leaves the `runs` row intact

#### Scenario: Log retention disabled
- **WHEN** `run_log_retention_enabled = false`
- **THEN** the log sweeper deletes no files regardless of age

### Requirement: Assessment-Retention Sweeper
The system SHALL run a background sweeper that periodically deletes whole runs
whose `created_at` is older than `run_retention_days`. For each expired run the
system SHALL delete the `runs` row (cascading to `scenario_results`), the run's
JSONL log file, and every collected `.ndjson` file referenced by that run's
`scenario_results.collected_log_path`. The sweeper SHALL run once at startup and
then on a fixed 1-hour interval, SHALL re-read `AppConfig` each tick, SHALL be a
no-op when `run_retention_enabled = false`, and SHALL skip runs whose `status` is
still `running`.

#### Scenario: Old run purged
- **WHEN** a completed run's `created_at` is older than `run_retention_days` and run retention is enabled
- **THEN** the `runs` row, its `scenario_results`, its JSONL log file, and all of its collected `.ndjson` files are deleted

#### Scenario: Run retention disabled
- **WHEN** `run_retention_enabled = false`
- **THEN** the sweeper deletes no runs regardless of age

#### Scenario: Running run skipped
- **WHEN** a run older than `run_retention_days` still has `status = "running"`
- **THEN** the sweeper does not delete it

### Requirement: Swept Logs Surface As Empty
The system SHALL serve `GET /api/runs/{id}/logs` with HTTP 200 and body `[]` when
the run's JSONL file has been swept by log retention, reusing the existing
missing-file behavior so an expired-log run is indistinguishable from one that
never logged.

#### Scenario: Logs requested after sweep
- **WHEN** a client GETs logs for a run whose JSONL file was deleted by the log sweeper
- **THEN** the response is HTTP 200 with body `[]`

### Requirement: Configure Retention From Assessments Page
The system SHALL present a "Run retention" control on the runs page that opens a
dialog for editing `run_log_retention_enabled`, `run_log_retention_days`,
`run_retention_enabled`, and `run_retention_days`, and SHALL persist changes via
`PUT /api/config`.

#### Scenario: Open and save
- **WHEN** an admin opens the dialog, enables run retention with a 14-day window, and saves
- **THEN** the new values are sent to `PUT /api/config` and reflected on the page after save
