## Why

simrun is, architecturally, a test runner for detections — but its vocabulary doesn't say so, and the words it uses collide across layers. "Scenario" names both the saved file *and* each case inside it ("a scenario of scenarios"). "Assertion", "matcher", and "expectation" are three words for two concepts. The execution entity is `run` in the backend but already shipped as "assessment" in the UI, so the two halves of the product disagree. The runner also carries a vestigial multi-scenario loop and three near-identical result types.

The fix is to model the domain on **GitHub Actions**, whose 4-level hierarchy maps almost exactly onto simrun — and whose bottom two levels are already simrun's (unchangeable) YAML keys.

## What Changes

Adopt a GitHub-Actions-style resource model and one consistent vocabulary end to end:

```
   Assessment   (definition — the saved YAML; GitHub "workflow")
      └── Run        (execution; GitHub "workflow run")
            └── Scenario     (parallel unit; GitHub "job")  — YAML key, unchanged
                  └── Expectation   (check via Matcher; GitHub "step") — YAML key, unchanged
```

- **BREAKING** The saved definition (`SavedScenario` / `saved_scenarios` / `/api/scenarios`) becomes **Assessment** (`assessments` / `/api/assessments`), addressable by its unique `name` (e.g. `GET /api/assessments/aws-privesc`) — mirroring GitHub addressing a workflow by filename. `name` becomes unique; the raw YAML is already returned in the JSON `yaml` field.
- The execution entity stays **Run** (`runs` table unchanged), GitHub-style: created at `POST /api/runs`, item at `GET/DELETE /api/runs/{id}`, all runs at `GET /api/runs`, and a nested read collection `GET /api/assessments/{id}/runs` for one assessment's history. Create is singular (`POST /api/runs`); reads may be nested. No `assessment_run`.
- **BREAKING** Run ingress moves from `POST /api/scenarios/run` body `{scenarioId}` to `POST /api/runs` body `{assessmentId}` — same shape, renamed path and field. Runs always reference a saved assessment; there is no inline-YAML run path today.
- **BREAKING** The run→definition FK `runs.scenario_id` becomes `runs.assessment_id`.
- The per-case noun stays **Scenario** at every layer (the YAML key `scenarios:` is unchanged, so code and YAML finally agree).
- **BREAKING** Drop **assertion** from the vocabulary. Keep **Expectation** (declared check = the YAML key, and the *outcome*) and **Matcher** (the *mechanism*). This spans three identifiers today: DB column `scenario_results.assertions` → `expectations`; run-result JSON key `matchers` → `expectations`; lint JSON count key `assertions` → `expectations`. Also `AssertionResult`/`LatestAssertionResult` → `ExpectationResult`; concrete `…Assertion` structs → `…Matcher`; `scenario.Assertions` → `scenario.Matchers`.
- **BREAKING** Retention config keys gate runs, so they shorten to `run_retention_*` / `run_log_retention_*` (from `assessment_retention_*` / `assessment_log_retention_*`), with operator-set values carried forward.

**Lean-runner cleanup (internal, behavior-preserving):**
- Remove the vestigial multi-scenario loop in `Runner.Run()`; `Runner` executes exactly one scenario and the worker pool is the sole fan-out.
- Extract one `Eventually(fn, interval, deadline)` poll primitive shared by the assert and explore paths.
- Consolidate the two near-identical in-memory result types (`runner.ScenarioResult`, `results.ScenarioRunResult`) into one `ScenarioResult`; the persistence row `db.ScenarioResult` stays a separate column-shaped DTO so `db` stays decoupled from the domain. Stop mutating the `Scenario` input to carry outputs. `SimrunRunResult` → `RunResult`.

> **Out of scope (deferred to a separate change):** unifying the three disconnected mode mechanisms (`type` column, per-run `exploreMode` bool, `" - collect mode"` suffix) into one authoritative `Mode` enum. That is a behavior change (it would wire `type` to execution for the first time and require migrating existing collect scenarios), so it is tracked separately. This change leaves `exploreMode`/`cleanupAlerts` and the collect-mode suffix exactly as they are.

## Capabilities

### New Capabilities
<!-- None — this change renames and refactors existing capabilities; it introduces no new behavior. -->

### Modified Capabilities
- `runs`: the `runs` table is retained, but the run→definition FK becomes `assessment_id`, the composite GET joins `assessments`, per-expectation outcomes rename `AssertionResult` → `ExpectationResult` (DB column `assertions` → `expectations`, result JSON key `matchers` → `expectations`), and the runner contract is clarified (single-scenario execution, one consolidated `ScenarioResult`).
- `scenarios`: the **saved definition** becomes **Assessment** (`saved_scenarios` → `assessments`, `/api/scenarios*` → `/api/assessments*`, addressable by unique name), and run ingress moves to `POST /api/runs` body `{assessmentId}` with a nested read `GET /api/assessments/{id}/runs`. The per-case "scenario" vocabulary and the YAML `scenarios:`/`expectations:` schema are unchanged.
- `assessment-retention`: retention config keys rename to `run_retention_*` / `run_log_retention_*` (they gate `runs`), with prior operator values migrated.

## Impact

- **DB**: migrations renaming `saved_scenarios` → `assessments` (+ unique `name`), FK `runs.scenario_id` → `assessment_id`, column `scenario_results.assertions` → `expectations`; `AppConfig` retention key rename with value-carrying backfill. The `runs` and `scenario_results` table names are unchanged (only the `assertions` column renames).
- **API/WS**: `/api/scenarios*` → `/api/assessments*`; run ingress at `POST /api/runs` body `{assessmentId}`; nested read `GET /api/assessments/{id}/runs`; result JSON key `matchers` → `expectations`; coordinated frontend update.
- **Go packages**: `internal/runner`, `internal/results`, `internal/db`, `internal/web`, `internal/parser`, `internal/matchers/*` (type/field renames + result consolidation + lean-runner refactor).
- **Frontend**: `web/frontend` — surface **Assessments** (the library of saved definitions) and **Runs** / **Run History** (executions); update API-client paths and keys.
- **Not changed**: user-authored YAML (`scenarios:`, `expectations:`, `targets:` keys), the component interfaces (`Detonator`/`Injector`/`Collector`/`Matcher`), the worker-pool executor, and the `runs` / `scenario_results` tables.
