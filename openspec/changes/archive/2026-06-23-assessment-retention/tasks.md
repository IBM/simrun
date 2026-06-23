## 1. Config model + persistence

- [x] 1.1 Add `AssessmentLogRetentionEnabled bool`, `AssessmentLogRetentionDays int`, `AssessmentRetentionEnabled bool`, `AssessmentRetentionDays int` to `config.AppConfig` and set defaults (`true`, `7`, `false`, `30`) in `DefaultAppConfig()` (`internal/config/appconfig.go`)
- [x] 1.2 Add the four keys to `parseAppConfig` (`> 0` guard for the day fields, like `parallelism`) and `appConfigKVs` (`internal/db/config.go`)
- [x] 1.3 Add migration `011_assessment_retention_appconfig` up/down backfilling the four keys with defaults, aligned with `DefaultAppConfig()` (`internal/db/migrations/`)
- [x] 1.4 Unit-test `parseAppConfig`/`appConfigKVs` round-trip and defaults for the new keys

## 2. PUT /api/config validation

- [x] 2.1 In `HandleUpdateConfig`, reject `assessment_log_retention_days < 1` and `assessment_retention_days < 1` with HTTP 400, leaving the stored value unchanged (`internal/web/handlers.go`)
- [x] 2.2 Test: PUT with either day field `= 0` → 400; valid values → persisted

## 3. Shared run-deletion helper (fix collected-.ndjson leak)

- [x] 3.1 Add a helper that, given a run ID, fetches its `scenario_results` collected paths, calls `RunStore.Delete`, then best-effort removes the JSONL log and each `collected_log_path` `.ndjson` (reject paths not ending in `.ndjson`, log warnings on failure) (`internal/web/handlers.go` or a small helper file)
- [x] 3.2 Refactor `HandleDeleteRun` to call the helper so manual delete also removes collected `.ndjson`
- [x] 3.3 Test: deleting a run with collected logs removes row, results, JSONL, and `.ndjson`; a missing `.ndjson` still returns 204

## 4. RunStore expired-runs query

- [x] 4.1 Add a `RunStore` method returning IDs of runs with `created_at < cutoff` and `status <> 'running'` (`internal/db/runs.go`)
- [x] 4.2 Test: returns only runs older than the cutoff and excludes `running` runs

## 5. Background sweepers

- [x] 5.1 Add a log-retention sweep helper in `internal/web/runlog.go` deleting `<DataDir>/run-logs/*.jsonl` older than N days by `ModTime`; no-op when disabled
- [x] 5.2 Add an assessment-retention sweep that queries expired run IDs and deletes each via the shared helper from task 3; no-op when disabled
- [x] 5.3 Start both sweepers as `go func()`s in `cmd/simrun/main.go` mirroring the session-cleanup goroutine: run once at startup, then on a 1h ticker, reading `AppConfig` each tick, exiting on `ctx.Done()`
- [x] 5.4 Test: log sweeper deletes aged JSONL and leaves the `runs` row; assessment sweeper deletes aged completed runs + all artifacts, skips `running` runs, and both are no-ops when disabled

## 6. Config page UI

- [x] 6.1 Add an "Assessment retention" button + shadcn `Dialog` on `web/frontend/src/routes/assessments` with two toggles + two day-counts (`Label`/`Input`/`Switch`)
- [x] 6.2 Wire save to `PUT /api/config` (one call per changed key, matching existing config writes); reflect saved values on the page
- [x] 6.3 Surface a 400 from the API in the dialog without discarding entered values

## 7. Verification

- [x] 7.1 `go test ./...`, `mise run lint`, `mise run fmt` pass
- [ ] 7.2 Manual: set log retention to 1 day, confirm an aged JSONL is swept on next tick and `GET /api/runs/{id}/logs` returns `200 []` while the run record remains
- [ ] 7.3 Manual: enable assessment retention at 1 day, confirm an aged completed run is fully removed (row, results, JSONL, collected `.ndjson`) and a `running` run is left untouched
