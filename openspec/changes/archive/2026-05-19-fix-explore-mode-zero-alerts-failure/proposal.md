## Why

Explore-mode scenario runs report `isSuccess: true` even when zero alerts were discovered. A run with `discoveredAlerts: null` is indistinguishable from a successful run at the run-counters level (`succeeded`/`failed`), which masks broken detections and defeats the point of running an explore-mode assessment. Users running detection-engineering experiments cannot tell from the dashboard which scenarios actually produced alerts.

## What Changes

- Treat an explore-mode scenario that completes with zero `DiscoveredAlerts` as a **failed** scenario (`isSuccess = false`) with an explanatory error message.
- An explore-mode scenario that discovered at least one alert continues to report `isSuccess = true`.
- Run-level counters (`succeeded`, `failed`) reflect the corrected per-scenario outcome automatically (no separate change needed).

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `matchers`: the "Explore Mode Bypasses Match Logic" requirement is amended so that the runner records pass/fail at the scenario level based on whether any alerts were discovered, while still not recording per-assertion outcomes.

## Impact

- Code: `simrun/internal/runner/runner.go` (`runExploreMode` — return a non-nil error when `len(scenario.DiscoveredAlerts) == 0`).
- API: no schema changes. The `isSuccess` field on `scenario_results` and the `succeeded`/`failed` counters on `runs` will now flip for explore runs that find no alerts.
- UI: existing failure styling in the assessments/run views applies; no frontend code changes required.
- Tests: runner unit test for the zero-alerts case; existing tests that assume explore mode always succeeds will need to be updated.
- No DB migration; no breaking API change. Historical run rows are not rewritten.
