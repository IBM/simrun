## 1. Runner behavior

- [x] 1.1 In `simrun/internal/runner/runner.go` `runExploreMode`, after the polling loop exits and the optional cleanup runs, return a non-nil error when `len(scenario.DiscoveredAlerts) == 0`. Message: `"explore mode: no matching alerts discovered within timeout"`.
- [x] 1.2 Verify by reading the function that returning the error AFTER the cleanup branch still allows discovered-alert cleanup to run when applicable (it won't in the zero-alerts case, but the branch order should make that obvious).

## 2. Tests

- [x] 2.1 Search for existing tests that exercise `runExploreMode` or assert `Success == true` for an empty discovery set (`grep -rn "ExploreMode" simrun/internal/runner` and tests). Update any that assume the old behavior.
- [x] 2.2 Add a unit test in `simrun/internal/runner/` that runs an explore-mode scenario against a fake Elastic API returning no matching alerts and asserts:
  - `runScenario` returns a non-nil error
  - The error message contains "no matching alerts discovered"
  - `scenario.DiscoveredAlerts` is empty
- [x] 2.3 Add a unit test for the positive case: explore mode with the fake returning at least one matching alert produces a `nil` error and `len(DiscoveredAlerts) >= 1`.

## 3. Verification

- [x] 3.1 `go test ./simrun/...` passes.
- [x] 3.2 Manually replay (or describe replay of) the bug-report run shape: a 4-scenario explore run where one scenario has empty `discoveredAlerts` now produces `succeeded: 3, failed: 1`.

## 4. Spec sync

- [x] 4.1 After implementation lands, run `/opsx:archive` to merge the matchers spec delta into `openspec/specs/matchers/spec.md`.
