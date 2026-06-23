## 1. DB layer

- [ ] 1.1 Add `UpdateScenarioIdentity(ctx, id uuid.UUID, executorName, executorType, executionID, simulationID string) error` to the `RunStore` interface in `internal/db/runs.go`, implementing a column-scoped `UPDATE scenario_results SET executor_name=…, executor_type=…, execution_id=…, simulation_id=… WHERE id=$1` that leaves `status`/`phase` untouched.
- [ ] 1.2 Add `UpdateScenarioAssertions(ctx, id uuid.UUID, assertionsJSON []byte) error` to `RunStore`, implementing `UPDATE scenario_results SET assertions=$2 WHERE id=$1` (status/phase untouched).
- [ ] 1.3 Regenerate mocks (`go generate ./...`) so the `RunStore` mock includes the two new methods.

## 2. Runner callbacks

- [ ] 2.1 Add `IdentityCallback func(name string, id ScenarioIdentity)` and `AssertionsCallback func(name string, results []AssertionResult)` (with their small value types) to `runner.Scenario` in `internal/runner/scenario.go`, alongside `StatusCallback`.
- [ ] 2.2 In `internal/runner/runner.go`, immediately after `executionId`/`simulation_id` are resolved post-detonation, invoke `IdentityCallback` (nil-guarded like `reportStatus`) with executor name/type (derived as in `results/executor.go`), `execution_id`, and `simulation_id`.
- [ ] 2.3 In `runAssertions`, after each assertion newly matches, invoke `AssertionsCallback` with the current assertion set: matched → `passed=true`, not-yet-matched → `passed` omitted (pending). Fire only on a state change, not every poll tick.

## 3. Web wiring

- [ ] 3.1 In `internal/web/scenarios.go`, wire `sc.IdentityCallback` to `runStore.UpdateScenarioIdentity(scenarioDBIDs[name], …)`, mirroring the existing `StatusCallback`→`UpdateScenarioPhase` block (nil/missing-id safe, log-on-error).
- [ ] 3.2 Wire `sc.AssertionsCallback` to marshal the partial assertions and call `runStore.UpdateScenarioAssertions(scenarioDBIDs[name], json)`, reusing the same assertion JSON shape produced by `buildScenarioResultRow`/`scenario_results.go`.
- [ ] 3.3 Confirm the terminal `CompleteScenarioResult` write still sets the authoritative `is_success`, full `assertions` (with terminal `passed=false` for unmatched), durations, and `discovered_alerts`, overwriting any mid-run partial values.

## 4. Frontend

- [ ] 4.1 In `web/frontend/src/lib/components/ScenarioResult.svelte`, surface executor identity (executor name, simulation id, matcher) on `running` entries, not only `completed` ones.
- [ ] 4.2 Extend the assertion mini-bar to a tri-state: `passed===true` → success, `passed===false` → error, `passed` undefined/null → muted (pending); show the running n/m matched count.
- [ ] 4.3 Verify the `scenario-tracker` carries partial `assertions`/identity from the poll for `running` rows (entries currently keep `result` only for completed — adjust so a running entry can expose the partial `ScenarioResult` fields).

## 5. Verification

- [ ] 5.1 Add/extend Go tests: `UpdateScenarioIdentity` and `UpdateScenarioAssertions` write only their columns and preserve `status='running'`; the runner fires `IdentityCallback` once post-detonation and `AssertionsCallback` on each new match.
- [ ] 5.2 `go test ./...`, `mise run lint`, and frontend `npm run check` + `npm run build` all pass.
- [ ] 5.3 Manually verify against a running assessment: a `matching` scenario shows executor/IDs and a live "k/n matched" assertion bar that fills in before completion, and the final state matches today's completed view.
