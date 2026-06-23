## Context

A scenario run is orchestrated in `web/scenarios.go`: a `scenario_results` row is pre-created as `pending` (`CreateScenarioStatus`), a `name → dbID` map is built, and each scenario's `StatusCallback(name, phase)` is wired to `runStore.UpdateScenarioPhase(dbID, phase)`. The runner (`internal/runner/runner.go`) drives detonation → matching → cleanup, calling `reportStatus` at each phase. Only at completion does `RunScenariosParallel`'s callback build the full row and call `CompleteScenarioResult(dbID, row)`.

Consequently, between `pending` and `completed` the row carries only `status` + `phase`. Yet the runner already holds everything else mid-run:
- Executor identity — `executor_name`/`executor_type`/`simulation_id` are derivable from `scenario.Detonator`/`Injector` with no detonation at all (`String()`, `SimulationId()`); `execution_id` is available the instant detonation returns (`runner.go` ~L95).
- Assertion progress — `runAssertions` (`runner.go:217-259`) resolves expected alerts one-by-one off a `remainingAssertions` channel; only the *failed* remainder is recorded, and only at completion.

The frontend already polls `GET /api/runs/{runId}` every 5s and renders from `scenario_results`, so any column written earlier is surfaced with no new transport.

## Goals / Non-Goals

**Goals:**
- A `running` scenario row exposes executor identity (`executor_name`, `executor_type`, `execution_id`, `simulation_id`) once detonation has occurred.
- A `matching` scenario row exposes per-assertion pass/pending state as the matcher resolves each expected alert.
- Reuse the existing poll path and the existing `name → dbID` callback pattern; no new transport.

**Non-Goals:**
- No new WebSocket message types or server push (the `assertion_update`/`scenario_started` types in the frontend `types.ts` stay unwired).
- No incremental `discovered_alerts` for explore mode (future work).
- No DB schema migration — target columns already exist on `scenario_results`.
- No change to the terminal completion write or to run-counter logic.

## Decisions

**1. Two new optional callbacks on `Scenario`, wired exactly like `StatusCallback`.**
Add `IdentityCallback func(name string, id ScenarioIdentity)` and `AssertionsCallback func(name string, results []AssertionResult)` to `runner.Scenario`. `web/scenarios.go` wires them to new `RunStore` methods keyed by the same `scenarioDBIDs[name]` map.
- *Why:* mirrors the proven phase-callback flow; keeps the runner transport-agnostic and the web layer the single place that knows about `dbID`.
- *Alternatives:* a single generic `ProgressCallback` (rejected — `phase` already owns status transitions; separate concerns read clearer); WebSocket push (rejected — adds plumbing, diverges from the poll model `phase` already uses).

**2. Fire identity once, right after detonation returns.**
In `runner.go` immediately after `executionId` (and `simulation_id`) are resolved, emit the identity callback carrying all four fields. Executor name/type are computed the same way `results/executor.go` already does.
- *Why:* a single write; `execution_id` is the highest-value "where" field (correlates to the Terraform working dir). Detonation is fast relative to the matching window, so waiting for it costs little.
- *Alternative:* emit name/type at queue time, `execution_id` later (two writes) — rejected as needless chatter.

**3. Incremental assertions use a tri-state, written on change only.**
After each successful match in the `runAssertions` loop, emit the current assertion set: matched → `passed = true`, not-yet-matched → `passed` omitted (pending). The terminal completion write continues to set `passed = true/false` definitively (a still-unmatched assertion becomes `false` only then). The callback fires only when an assertion newly resolves, not on every poll tick.
- *Why:* avoids rendering an unmatched-but-still-polling assertion as a red failure; the optional `passed?: boolean` field already models pending. Write-on-change keeps UPDATE volume to roughly one per matched assertion.
- *Alternative:* write on every poll iteration (rejected — write amplification for no signal change).

**4. New DB methods do column-scoped partial UPDATEs.**
`UpdateScenarioIdentity(ctx, id, name, typ, execID, simID)` and `UpdateScenarioAssertions(ctx, id, assertionsJSON)` update only their columns and leave `status='running'`/`phase` untouched. Add to the `RunStore` interface and regenerate mocks.
- *Why:* preserves the lifecycle state machine; each scenario writes only its own row by `dbID`, so parallel scenarios never contend.

## Risks / Trade-offs

- **Frontend miscolors pending assertions as failed** → tri-state: only the completion write sets `passed=false`; mid-run unmatched assertions carry `passed` undefined and render muted. The assertion mini-bar gains a third (pending) color.
- **Write amplification from the matcher loop** → fire the assertions callback only on a newly-resolved assertion, not per poll tick; identity is a single write.
- **Failed/aborted detonation** → identity callback may carry an empty `execution_id`; this is acceptable (mirrors today's tolerance for empty `simulation_id`) and the terminal completion write still produces the authoritative row.
- **Callback nil-safety** → both new callbacks are optional and nil-guarded like `reportStatus`, so non-web callers (tests, CLI paths) are unaffected.

## Migration Plan

No schema migration. Ship backend + frontend together. Rollback is a plain revert: running rows simply return to `status` + `phase` only, and an older frontend harmlessly ignores the earlier-populated fields (backward compatible).

## Open Questions

- Should explore mode surface `discovered_alerts` incrementally the same way? Deferred to a follow-up; this change scopes to assertion-based scenarios + identity.
