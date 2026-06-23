## Why

While a scenario is running, the only per-scenario data we expose is `status` + `phase`, so the UI can show nothing but a phase badge and the streamed log lines. The executor identity, the resource IDs, and per-assertion match results are all known to the runner mid-run but are flushed to `scenario_results` in a single write at completion — so a running row reads "in progress" with no sense of *what* is executing, *where*, or *how close* detection is to passing. The data already exists in the runner; it just isn't persisted until the end.

## What Changes

- Persist executor identity to the `scenario_results` row **right after detonation** (before matching begins): `executor_name`, `executor_type`, `execution_id`, and `simulation_id`. A running row then shows what is executing and where, surfaced through the existing `GET /api/runs/{runId}` poll.
- Flush per-assertion results **incrementally as the matcher resolves them**, rather than only at completion. As each expected alert matches (or the matching window advances), the row's `assertions` reflect current pass/pending state, enabling a live "2/3 matched" view.
- No new WebSocket message types or backend push plumbing — both rely on incremental `UPDATE`s to `scenario_results` picked up by the frontend's existing 5s poll, consistent with how `phase` is surfaced today.
- Frontend renders the now-available executor identity and partial assertion progress on `running` scenario rows (today these are gated to `completed` rows).

## Capabilities

### New Capabilities

_None._

### Modified Capabilities

- `runs`: The **Scenario Result Lifecycle** requirement is extended so that a `scenario_results` row in the `running` state exposes executor identity (after detonation) and incrementally-updated assertion results (during matching), not just `status` + `phase`.

## Impact

- **DB layer** (`internal/db/runs.go`): new/extended scenario-result update methods — one to record executor identity post-detonation, one to upsert incremental assertion results during matching. New `RunStore` interface methods + mock regeneration.
- **Runner** (`internal/runner/runner.go`): invoke the post-detonation persistence hook once the executor returns identity, and persist each assertion outcome inside the existing `runAssertions` poll loop as it resolves.
- **Runner↔DB wiring**: extend the per-scenario callback/store currently used for `UpdateScenarioPhase` to carry the new writes.
- **Frontend** (`web/frontend/src/lib/components/ScenarioResult.svelte`, `scenario-tracker`): show executor identity and partial assertion ticks for `running` entries.
- No schema migration expected — the target columns (`executor_name`, `executor_type`, `execution_id`, `simulation_id`, `assertions`) already exist on `scenario_results`; this change writes them earlier.
