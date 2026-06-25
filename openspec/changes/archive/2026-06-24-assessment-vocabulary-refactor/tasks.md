## 1. Tranche A — Internal refactor (zero user impact)

- [x] 1.1 Extract `Eventually(fn func() (bool, error), interval, deadline) error` and reimplement `runAssertions` and `runExploreMode` polling on top of it (behavior-preserving)
- [x] 1.2 Rename matcher vocabulary: `scenario.Assertions` → `Matchers`, `FailedAssertions` → `UnmetExpectations`; concrete `*Assertion` structs (e.g. `ElasticSecurityAlertGeneratedAssertion`) → `*Matcher`
- [x] 1.3 Rename `AssertionResult`/`LatestAssertionResult` → `ExpectationResult` across `internal/db`, `internal/web`; converge the three result identifiers on `expectations` — run-result JSON key `matchers` (`results/types.go`) → `expectations`, lint count key `assertions` (`web/types.go`) → `expectations` (DB column handled in 3.x)
- [x] 1.4 Consolidate the two in-memory types `runner.ScenarioResult` and `results.ScenarioRunResult` into one `ScenarioResult` (persistence row `db.ScenarioResult` kept as a separate column-shaped DTO so `db` stays decoupled); rename `SimrunRunResult` → `RunResult`
- [x] 1.5 Make `runScenario` return the populated `ScenarioResult` instead of mutating the `Scenario` input; update the executor to consume the returned value
- [x] 1.6 Update unit tests in `internal/runner`, `internal/results`, and matchers to the new names/shapes; `go build ./...` and `go test ./...` green

> Note: unifying mode selection (`type`/`exploreMode`/`" - collect mode"` suffix → one authoritative `Mode` enum) is a behavior change handled in a separate change; `ExploreMode`/`CleanupAlerts` and the collect-mode suffix are left untouched here.

## 2. Tranche B — Lean runner (single-scenario)

- [x] 2.1 Remove `Runner.Scenarios []` and the multi-scenario `for` loop + `failedScenarios` aggregate-error builder; make `Runner` execute exactly one scenario
- [x] 2.2 Confirm `RunScenariosParallel` is the sole fan-out path; adjust `runSingleScenario` to call the single-scenario runner and map its `ScenarioResult`
- [x] 2.3 Update/retire tests that exercised the multi-scenario loop; `go test ./...` green

## 3. Tranche C — Rename to the GitHub-Actions model (coordinated)

- [x] 3.1 DB migration: rename `saved_scenarios` → `assessments`; add a `UNIQUE` constraint on `assessments.name`, disambiguating any pre-existing duplicate names first and logging each rename
- [x] 3.2 DB migration: rename FK column `runs.scenario_id` → `runs.assessment_id` (preserve the FK/index)
- [x] 3.3 DB migration: rename column `scenario_results.assertions` (JSONB) → `expectations`; add matching `down` migration
- [x] 3.4 DB migration: rename retention `AppConfig` keys `assessment_(log_)retention_*` → `run_(log_)retention_*`, carrying forward operator-set values, then remove the old keys; add matching `down` migrations
- [x] 3.5 Rename Go types/stores: `SavedScenario` → `Assessment`, `ScenarioStore` saved-definition methods, `RunStore` FK field, config keys in `AppConfig`/`DefaultAppConfig()`
- [x] 3.6 API: move `/api/scenarios*` → `/api/assessments*`; add `GET /api/assessments/{name}` (JSON, includes raw `yaml` field); enforce unique-name 409 on create
- [x] 3.7 API: move run ingress to `POST /api/runs` body `{assessmentId}` (replacing `POST /api/scenarios/run` body `{scenarioId}`); add nested read `GET /api/assessments/{id}/runs` (delegates to `ListRunsFilters` with `assessment_id`); update `GET /api/runs/{id}` composite to join `assessments` and return `assessmentName`/`assessmentType` (response stays `{runId}`)
- [x] 3.8 API: retention handlers read/write the new `run_(log_)retention_*` keys; validation messages updated
- [x] 3.9 Frontend: rename routes/components/state from scenario-library → **Assessments** and run-history → **Runs**; update API-client paths and JSON keys (result `matchers`/lint `assertions` → `expectations`, assessment by name)
- [x] 3.10 Frontend: relabel the retention dialog to "Run retention" and bind to the new config keys

## 4. Verification

- [x] 4.1 `mise run build` (frontend + server) succeeds; `mise run lint` clean
- [x] 4.2 `go test ./...` green; add/adjust handler tests for the new endpoints (run a saved assessment, fetch assessment by name, duplicate-name 409)
- [~] 4.3 Migration round-trip — **up verified on a live operator DB** (schema_migrations=15 clean; `assessments` + UNIQUE `name`, `runs.assessment_id`, `scenario_results.expectations`; retention keys carried operator-set values `run_log_retention_days=3`/`run_retention_days=60` forward, no `assessment_*` keys left). `.down.sql` counterparts ship but the down round-trip was **not** run against the live DB to avoid disrupting real data
- [x] 4.4 Manual smoke — **verified on live system**: created/listed assessments, ran one via `POST /api/runs {assessmentId}`, `runs` rows carry `assessment_id`, `GET /api/runs/{id}` returns `assessmentName`/`assessmentType` + `scenarios[].expectations`, run appears under `GET /api/assessments/{id}/runs`, duplicate name → 409, `/api/rules/coverage` 200 (jsonb `expectations` query OK)
- [x] 4.5 Confirm no `assertion`/`SavedScenario`/`assessment_run`/old-retention-key identifiers remain (grep), and YAML `scenarios:`/`expectations:` parsing is unchanged
