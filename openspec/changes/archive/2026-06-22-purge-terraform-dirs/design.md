## Context

`deleteRunWithArtifacts` (`internal/web/run_deletion.go:26`) is the single shared
deletion path used by both the manual delete handler (`HandleDeleteRun`) and the
assessment-retention sweeper (`SweepAssessments`). Today it loads the run's
scenario results, deletes the `runs` row (cascading to `scenario_results`), then
best-effort removes the JSONL run log and each collected `.ndjson` file. It never
references `<DataDir>/terraform/`.

Each detonation creates a Terraform working directory at
`<DataDir>/terraform/<executionID>` — see `terraform.Manager.Setup` building
`filepath.Join(m.baseDir, executionID)` (`internal/packs/terraform/manager.go:106`)
with `baseDir = <dataDir>/terraform`. The `executionID` is persisted per scenario
result as `scenario_results.execution_id` (`db.ScenarioResult.ExecutionID`,
`internal/db/runs.go:73`). The deletion function already loads these results into
`results`, so the execution IDs are in hand at exactly the right point — before the
cascade removes the rows.

## Goals / Non-Goals

**Goals:**
- Reclaim `<DataDir>/terraform/<executionID>/` for every scenario result of a
  deleted run, via the shared deletion path so both manual delete and the sweeper
  benefit with no duplicated logic.
- Keep removal best-effort and non-fatal, matching the existing JSONL/`.ndjson`
  cleanup contract.
- Make the removal path-safe so it can never escape or wipe the `terraform/` base.

**Non-Goals:**
- No change to how/where Terraform dirs are created during detonation.
- No backfill/orphan-sweep of pre-existing Terraform dirs whose runs are already
  gone (those have no `execution_id` to key on anymore). Out of scope.
- No edits to the unarchived `assessment-retention` change's spec (flagged in the
  proposal for sync at archive time, not modified here).

## Decisions

**Key the cleanup on `execution_id` from the already-loaded results.**
The function already calls `GetScenarioResults` to gather collected paths before
the cascade. Reuse that same loop to collect `<dataDir>/terraform/<executionID>`
targets. No second query, no signature change — `dataDir` is already a parameter.
*Alternative considered:* glob `<DataDir>/terraform/` and match against run state.
Rejected — slower, racy with concurrent runs, and the rows are about to be deleted.

**Use `os.RemoveAll`, guarded against unsafe `execution_id`.**
The work dir is a directory tree (state, `.terraform/` plugins), so `os.Remove`
is insufficient; `os.RemoveAll` is required. Before removing, skip any
`execution_id` that is empty/whitespace (`strings.TrimSpace`) or contains a path
separator (`strings.ContainsRune(id, os.PathSeparator)` / `filepath.Separator`,
also reject `/`). This prevents both `RemoveAll("<dataDir>/terraform/")` (which a
blank id would produce) and traversal. Execution IDs are UUIDs in practice, so the
guard rejects only malformed data.
*Alternative considered:* trust the value since it is server-generated. Rejected —
`fail loud / defense in depth`; a blank id wiping the whole base dir is too costly
to leave unguarded.

**Best-effort, log-and-continue on error.**
Mirror the existing `.ndjson` block: on `os.RemoveAll` error that is not
`os.IsNotExist`, log a warning with `run_id` and the path, and continue. A leftover
or unremovable dir must never block reclaiming the DB row.

## Risks / Trade-offs

- [A scenario result carries an `execution_id` that points outside `terraform/`]
  → The path-separator/blank guard rejects it; only `<dataDir>/terraform/<id>` with
  a clean single-segment id is ever removed.
- [Concurrent run reuses the same dir while deletion runs] → Execution IDs are
  unique UUIDs per detonation, so a deleted run's dir is never an in-flight run's
  dir; no locking needed.
- [Pre-existing orphaned dirs from before this change] → Not reclaimed by this
  change (no row/id to key on). Acceptable; explicitly a non-goal. Operators can
  remove them manually if desired.

## Migration Plan

Pure code change, no schema migration. Deploys with the binary. Rollback is
reverting the commit — no persisted state depends on the new behavior. After
deploy, deleting a run (or a sweeper tick) removes the Terraform dirs going
forward.
