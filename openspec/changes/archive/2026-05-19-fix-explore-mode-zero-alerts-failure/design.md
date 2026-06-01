## Context

Explore mode is the "discovery" sibling of standard assertion mode: instead of asserting that a named rule fires, the runner polls Elasticsearch for any open alerts whose serialized form contains one of the configured indicators (typically the execution ID or other terraform-output values). Discovered alerts go into `scenario.DiscoveredAlerts`.

Today `TestRunner.runExploreMode` (`simrun/internal/runner/runner.go:341`) always returns `nil` on a clean polling loop, regardless of how many alerts were discovered. In `TestRunner.Run` (`runner.go:48`) a `nil` error from `runScenario` flips `result.Success = true`. The executor wrapper (`simrun/internal/results/executor.go`) propagates that to `ScenarioRunResult.Success` and ultimately to the `isSuccess` JSON field. There is no other place where explore-mode success is set, so the entire pass/fail signal hinges on the return value of `runExploreMode`.

The sample run in the bug report shows `discoveredAlerts: null` for the last scenario, yet `isSuccess: true`. The run counters (`succeeded: 4, failed: 0`) are derived from the per-scenario `isSuccess`, so they propagate the same lie up to the run row.

## Goals / Non-Goals

**Goals:**
- An explore-mode scenario that completes with zero discovered alerts is reported as failed at the scenario level.
- An explore-mode scenario that discovers ≥ 1 alert continues to be reported as successful.
- The error message surfaced for the zero-alerts case is clear enough for a user to understand it from the UI without needing logs.
- Run-level `succeeded`/`failed` counters reflect the corrected scenario outcomes automatically through existing counter-increment logic.

**Non-Goals:**
- No new spec for partial success (e.g., "found some but not enough"). One or more is success; zero is failure.
- No retroactive correction of historical run rows.
- No change to how indicators are built or how the elastic query is constructed.
- No UI work — failure styling already exists for non-explore scenarios and applies equally.
- No change to the per-assertion record format; explore mode still does not produce `assertions` entries (the `assertions` in the sample payload are a synthetic frontend artifact, not produced by the matcher).

## Decisions

### Decision 1: Decide pass/fail inside `runExploreMode`, not at the call site

`runExploreMode` already owns the `scenario.DiscoveredAlerts` slice and the polling loop. The cleanest fix is for it to return an explicit error when the loop exits with an empty slice, mirroring how `runAssertions` returns an error when assertions remain unmatched at deadline.

**Alternative considered:** Set a new `scenario.ExploreFailed` field and have the executor read it. Rejected — it duplicates state that the existing `error` return already encodes, and would require new plumbing through `TestRunner.Run` and `ScenarioRunResult`.

**Alternative considered:** Decide in `executor.go` by inspecting `len(DiscoveredAlerts)` after the run. Rejected — split the success decision across two packages and would leave the `runScenario` `error` path lying about the actual outcome.

### Decision 2: Error message wording

Return `fmt.Errorf("explore mode: no matching alerts discovered within timeout")` (or similar). Including the word "timeout" tells the user the scenario didn't crash — it ran to completion without finding anything, which is the actionable distinction from infrastructure errors. The message becomes `scenario_results.error` and is what the UI surfaces.

### Decision 3: Don't change the matchers spec for per-assertion outcomes

The current spec language "without recording pass/fail outcomes for assertions" is still correct at the assertion level — explore mode never produced per-assertion rows. The modification is strictly at the scenario-level `isSuccess` derivation. The MODIFIED requirement keeps the assertion-level disclaimer and adds the scenario-level rule.

### Decision 4: Tests

Add a runner test that constructs an explore-mode scenario, runs against a fake Elastic API that returns zero matching alerts, and asserts `runScenario` returns a non-nil error and the resulting `ScenarioResult.Success` is `false`. Update any existing explore-mode tests that asserted `Success == true` with an empty discovery set (search first; the change set should be small).

## Risks / Trade-offs

- **Risk:** Users who today rely on the "always green" explore-mode behavior to inspect `discoveredAlerts` regardless of outcome may be surprised when scenarios start showing as red.
  - **Mitigation:** This is the intended behavior change. The `discoveredAlerts` field is still populated and visible on the failed scenario, so the inspection workflow is unaffected.

- **Risk:** A flaky Elastic API (e.g., transient query errors) that returns zero results during the entire timeout window will now produce a failed scenario rather than a misleadingly-green one.
  - **Mitigation:** This is also the intended behavior — surfacing the problem is the point. Infrastructure errors that surface as `error` from `ExploreAlerts` already cause the scenario to fail today, so this only changes the silent-zero case.

- **Trade-off:** No threshold knob (e.g., "fail unless ≥ N alerts"). Keeping it binary keeps the change surgical; a threshold can be added later if the data demands it.
