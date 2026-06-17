## Context

Run logs are one JSONL file per run at `<DataDir>/run-logs/<runID>.jsonl`
(`internal/web/runlog.go`), removed only by `DeleteRunLog` when a run is
deleted. Whole assessments — the `runs` row, its `scenario_results`, and the
collected `.ndjson` files at `scenario_results.collected_log_path` — are never
cleaned up automatically. Both accumulate and filled the host's shared volume.

Deployment is single-instance on a shared volume. Bounding growth — not
relocating bytes to Postgres/object storage — is what fixes the out-of-space
condition, so retention stays on disk + Postgres as-is.

Two distinct lifetimes are wanted: verbose logs are disposable after a few days,
while assessment summaries may be kept longer before the whole run is purged.
Hence two independent retention windows.

Existing patterns reused:
- `AppConfig` is a key/value model in `app_config`, parsed by `parseAppConfig`
  and written by `appConfigKVs` (`internal/db/config.go`), served by
  `GET/PUT /api/config`. New settings follow `pack_logs_enabled` exactly.
- A periodic background cleanup goroutine already exists for auth sessions
  (`cmd/simrun/main.go:103`): ticker + `ctx.Done()`. Both sweepers copy it.
- `GET /api/runs/{runId}/logs` already returns `200 []` for a missing file, so
  log-swept runs need no endpoint change.
- `RunStore.Delete` already cascades `scenario_results` via FK; the manual
  delete handler (`HandleDeleteRun`) calls `DeleteRunLog` after.

## Goals / Non-Goals

**Goals:**
- Bound run-log disk usage by age (log sweeper, keeps the run record).
- Bound whole-assessment growth by age (assessment sweeper, removes row +
  results + JSONL + collected `.ndjson`).
- Two independent, admin-tunable windows from the assessments page; sane defaults.
- Fix the existing leak where deleting a run leaves collected `.ndjson` behind.
- Minimal, surgical change; no new storage backend or infra.

**Non-Goals:**
- A per-run log size cap. Measured: ~45 sims running for an hour produce only a
  few MB of JSONL, so a single run cannot realistically exhaust the disk.
- Moving logs to Postgres or object storage (rejected in brainstorm: no benefit
  on a shared volume, adds write amplification / infra).
- Retention for terraform work dirs or pack cache.
- Horizontal-scale / cross-instance access.

## Decisions

### Two independent windows, both enforced by background sweepers
- **Log retention** deletes `run-logs/*.jsonl` by file `mtime` older than
  `assessment_log_retention_days`; keeps the `runs` row. Default **on, 7 days**.
- **Assessment retention** deletes whole runs by `created_at` older than
  `assessment_retention_days`; removes the row (cascading results), the JSONL,
  and the collected `.ndjson`. Default **off, 30 days** — opt-in because it
  destroys results history.

Both run as `go func()`s in `main.go` mirroring the session cleaner: once at
startup, then on a 1h ticker, reading `AppConfig` each tick so changes apply
without restart, exiting on `ctx.Done()`. The 1h interval is fixed (matches the
session cleaner), not a config knob.

- *Why mtime for logs but created_at for assessments:* the log file's mtime is
  the cheapest truth for "last touched" and also reclaims orphaned files; an
  assessment's age is defined by when the run started, which is `created_at`.
- *Why skip `status = running`:* avoids deleting a run that is still actively
  writing results/logs even if it has been running absurdly long.

### Shared run-deletion helper (fixes the collected-`.ndjson` leak)
`HandleDeleteRun` today removes the row + JSONL but **not** the collected
`.ndjson` files, so those large SIEM-log artifacts leak on manual delete. The
assessment sweeper needs to remove them too. Extract a single helper that, given
a run ID: reads its `scenario_results` collected paths, calls `RunStore.Delete`,
then best-effort removes the JSONL and each `.ndjson` (rejecting paths not
ending in `.ndjson`, matching the existing collected-logs handler guard). Both
`HandleDeleteRun` and the assessment sweeper call it, so behavior is identical
and the leak is fixed once.

- *Alternative considered — sweeper deletes rows then globs orphaned files:*
  fragile (can't reliably tie an arbitrary `.ndjson` to a run); using the stored
  `collected_log_path` is precise.

### RunStore: query expired runs
Add a `RunStore` method to list runs with `created_at < cutoff` and
`status <> 'running'` (returning IDs; the helper then fetches each run's
collected paths via the existing `GetScenarioResults`). The existing `List`
filters support `Since` (>=); this adds the complementary cutoff for the
sweeper. Deletion reuses `RunStore.Delete`.

### Validation in the PUT handler
`PUT /api/config` is a generic key/value setter today. The handler SHALL reject
`assessment_log_retention_days < 1` and `assessment_retention_days < 1` with HTTP 400.
Validation is keyed on the config `key` being set; other keys keep today's
permissive behavior.

### Config surface: one dialog, both windows
An "Assessment retention" button on `web/frontend/src/routes/assessments` opens
a dialog (shadcn `Dialog` + `Label`/`Input`/`Switch`) with two enable toggles
and two day-counts, saving via `PUT /api/config` (one call per changed key).
Config is loaded lazily when the dialog opens. 400s surface in the dialog
without discarding input.

## Risks / Trade-offs

- **Assessment retention is destructive (deletes results history)** → shipped
  off by default; admins opt in deliberately; validation floors at 1 day.
- **Mtime drift / clock skew on the host** → mtime/created_at are local to the
  writing host, so relative age is consistent for a multi-day window.
- **Log retention deletes logs an operator still wanted** → default 7 days
  matches stated "disposable after a few days"; the run summary persists.
- **Best-effort file removal can leave a stray file if a path is malformed** →
  the `.ndjson` guard plus logging a warning on failure keeps it visible
  without failing the sweep.
- **Two sweepers read AppConfig hourly (2 DB hits/hour)** → negligible, same
  order as the session cleaner.

## Migration Plan

1. Add migration `011_assessment_retention_appconfig` (up: backfill the four
   keys with defaults; down: delete them). Aligns with `DefaultAppConfig()`.
2. Ship code: new `AppConfig` fields, `parseAppConfig`/`appConfigKVs` entries,
   shared run-deletion helper (reused by `HandleDeleteRun`), `RunStore`
   expired-runs query, both sweeper goroutines, PUT validation, config dialog.
3. Rollback: revert the binary and run the migration `down`; on-disk files are
   untouched by rollback. Accumulated logs are reclaimed on the first log sweep;
   accumulated old assessments are reclaimed only after an admin opts in.

## Open Questions

- None outstanding. Sweep interval fixed at 1h; size cap dropped; assessment
  retention default-off confirmed.
