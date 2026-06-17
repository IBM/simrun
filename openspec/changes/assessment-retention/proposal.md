## Why

Run-log JSONL files (`<DataDir>/run-logs/<runID>.jsonl`) are deleted only when
their parent run is deleted, and whole assessments (the `runs` row, its
`scenario_results`, and the collected `.ndjson` log files) are never cleaned up
automatically. Both pile up without bound and recently filled the host's shared
volume. Operators treat run logs as disposable a few days after a run finishes,
and want a way to bound how long whole assessments are kept too.

## What Changes

- Add a background **log-retention sweeper** that deletes run-log JSONL files
  older than a configurable age, keeping the assessment record. Run summaries
  persist; their verbose logs expire.
- Add a background **assessment-retention sweeper** that deletes whole runs
  older than a configurable age — the `runs` row (cascading to
  `scenario_results`), the JSONL log file, **and** the collected `.ndjson`
  files referenced by `collected_log_path` (the large SIEM-log artifacts), so
  the disk is actually reclaimed. Disabled by default (opt-in), since it
  destroys results history.
- Add a shared run-deletion helper that removes a run's row plus all of its
  on-disk artifacts (JSONL + collected `.ndjson`). The manual
  `DELETE /api/runs/{id}` is updated to use it so it no longer leaks collected
  `.ndjson` files.
- Extend `AppConfig` with admin-tunable retention settings (log-retention
  enable + days; assessment-retention enable + days), stored in `app_config`
  and served by the existing `GET/PUT /api/config`.
- Add an **"Assessment retention" button + dialog** on the assessments page so
  admins set both cleanup cadences from the UI.

Relies on the existing `runs` behavior that a missing log file yields `200 []`
from `GET /api/runs/{runId}/logs`, so swept logs surface gracefully as "no
logs" rather than an error.

## Capabilities

### New Capabilities
- `assessment-retention`: Time-bounded lifecycle for run logs and whole
  assessments — the two background sweepers, the shared run-deletion helper,
  the admin-configurable retention settings, and the UI to tune them.

### Modified Capabilities
- `runs`: The "Delete Run Cascades to Results and Log File" requirement is
  extended so deleting a run also removes the collected `.ndjson` files, not
  just the JSONL log.

## Impact

- **Code:** `internal/web/runlog.go` (log sweep helper),
  `internal/web/handlers.go` (shared run-deletion helper; reused by
  `HandleDeleteRun`), `internal/db/runs.go` (method to list/delete runs older
  than a cutoff and return their artifact paths), `internal/config/appconfig.go`
  (new fields + defaults), server startup (`cmd/simrun/main.go`) to start both
  sweeper goroutines, a new `app_config` migration to backfill defaults.
- **Frontend:** `web/frontend/src/routes/assessments` — new button + dialog
  (`web/frontend/src/lib/components/RetentionDialog.svelte`) wired to
  `PUT /api/config`.
- **APIs:** `GET/PUT /api/config` payload gains four retention fields. No new
  endpoints.
- **Dependencies / infra:** none. Single-host, shared-volume deployment; no new
  storage backend.
