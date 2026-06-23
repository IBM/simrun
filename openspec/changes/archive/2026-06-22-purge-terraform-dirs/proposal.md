## Why

Deleting a run — whether manually via `DELETE /api/runs/{runId}` or automatically
via the assessment-retention sweeper — leaves the run's Terraform working
directories on disk forever. The shared `deleteRunWithArtifacts` path removes only
the DB row, the JSONL run log, and the collected `.ndjson` files; it never touches
`<DataDir>/terraform/<executionID>/`. These directories accumulate unbounded (one
per detonation, each holding providers, state, and plugins). The capability to
reclaim that disk simply does not exist in the code today.

## What Changes

- Extend the shared `deleteRunWithArtifacts` path so that, for every scenario
  result of the run, it best-effort removes the run's Terraform working directory
  at `<DataDir>/terraform/<executionID>/`, keyed by `scenario_results.execution_id`.
- Removal is best-effort and bounded: a missing or unremovable directory logs a
  warning and never fails the delete (matching how JSONL/`.ndjson` cleanup behaves
  today). A blank/whitespace `execution_id` or one containing a path separator is
  skipped so the cleanup can never escape or wipe the `terraform/` base directory.
- Because both the manual delete handler and the retention sweeper call
  `deleteRunWithArtifacts`, both reclaim Terraform directories identically with no
  separate code path.

## Capabilities

### New Capabilities
<!-- None. This extends existing deletion behavior. -->

### Modified Capabilities
- `runs`: The "Delete Run Cascades to Results and Log File" requirement gains
  best-effort removal of the run's per-execution Terraform working directories, so
  the enumerated artifact set removed on delete now includes
  `<DataDir>/terraform/<executionID>/`.

## Impact

- **Code**: `internal/web/run_deletion.go` (`deleteRunWithArtifacts`) — adds the
  Terraform-dir removal step using `dataDir` and each result's `ExecutionID`
  (already loaded via `GetScenarioResults`). No new dependencies, no signature
  change; the sweeper (`SweepAssessments`) and manual delete (`HandleDeleteRun`)
  inherit the behavior automatically.
- **On-disk layout**: Relies on the detonator/terraform-manager convention that
  the work dir is `<DataDir>/terraform/<executionID>` (`internal/packs/terraform/manager.go`).
- **Flag for follow-up (not in scope here)**: the pending `assessment-retention`
  change's "Assessment-Retention Sweeper" requirement enumerates the artifacts the
  sweeper removes (row, JSONL, `.ndjson`) and should be kept in sync to mention
  Terraform directories when that change is archived. Surfaced here rather than
  edited, to avoid blending with an unarchived change's deltas.
