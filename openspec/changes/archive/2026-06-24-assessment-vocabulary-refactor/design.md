## Context

simrun is architecturally a test runner for detection rules, but its vocabulary obscures that and collides across layers. A brainstorm established two intertwined problems and one strong analogy:

1. **Vocabulary** — "scenario" names both the container and the case; "assertion"/"matcher"/"expectation" are three words for two concepts; the execution entity is `run` in the backend but shipped as "assessment" in the UI.
2. **Duplicated runner machinery** — `Runner.Run()` carries a vestigial multi-scenario loop, three modes are encoded three different ways, the poll loops are duplicated, and three near-identical result types coexist.
3. **The GitHub Actions analogy** — simrun's domain hierarchy maps almost exactly onto Actions, and the bottom two levels are already simrun's unchangeable YAML keys.

Constraints:
- **User-authored YAML is immovable** (`scenarios:`, `expectations:`, `targets:` stay).
- **The component interfaces are good** (`Detonator`/`Injector`/`Collector`/`Matcher`) — out of scope.
- **The `runs` table should not be renamed.** An earlier draft renamed it to `assessment_runs`; that jammed the parent's name into the child and read badly. GitHub keeps the child as plain `run` and lets the URL namespace disambiguate.

## Goals / Non-Goals

**Goals:**
- A GitHub-Actions-style resource model: `Assessment` → `Run` → `Scenario` → `Expectation`.
- Keep `scenario` and `expectation` as the case/check nouns everywhere so code and YAML agree.
- Keep the `runs` and `scenario_results` tables in place; disambiguate runs by URL nesting, not by renaming.
- Collapse duplicated runner machinery: one fan-out layer, one `Mode`, one poll primitive, one result type.
- Backward-compatible config migration (no operator-set retention values lost).

**Non-Goals:**
- Changing user-authored YAML keys.
- Touching the component interfaces or the worker-pool concurrency model.
- Adding dependent scenario chains / DAG orchestration — explicitly deferred (see Open Questions).
- Renaming `scenario` → `case` (rejected: would re-create a code-vs-YAML mismatch).

## Decisions

### Decision 1: Model the resource hierarchy on GitHub Actions
```
   GitHub Actions          simrun
   ──────────────          ──────
   Workflow (.yml)     ↔   Assessment   (saved definition, addressable by name)
   Workflow Run        ↔   Run          (execution; table stays `runs`)
     Job (parallel)    ↔   Scenario     (parallel worker-pool unit) — YAML key
       Step            ↔   Expectation  (checked by a Matcher)      — YAML key
```
The bottom two levels need no renaming. Only the top two get names: `Assessment` (definition) and `Run` (execution).

- **Why `Run` stays `Run`** (not `AssessmentRun`): GitHub addresses runs as `/actions/runs/{id}` and `/actions/workflows/{id}/runs` — the namespace disambiguates, so the child noun stays short. simrun mirrors this. Bonus: the `runs` table and retention keys (`run_retention_*`) stay clean, and the migration shrinks.
- **Run routing:** a Run is a top-level resource. Create is singular at `POST /api/runs` (body `{assessmentId, …opts}`) — *not* duplicated as a nested create — so there is one way to mint a run. Reads may live in multiple places (GitHub-style): `GET /api/runs`, `GET /api/runs/{id}`, and the nested read collection `GET /api/assessments/{id}/runs` (which internally applies `ListRunsFilters` with `assessment_id`). Writes singular, reads plural.

### Decision 2: `Assessment` is the definition, addressable by unique name
The saved definition becomes `Assessment`. Because each assessment serializes to `<name>.yaml` (GitHub addresses workflows by filename), `name` becomes UNIQUE and serves as the slug: `GET /api/assessments/{name}` returns the JSON, which already includes the raw `yaml` field. (A separate `.yaml` raw endpoint was considered and dropped — it would only re-serve a string already present in the JSON; Rule 2.)

- **Why "Assessment"** over Plan/Playbook/Suite: it is the security-domain-native word ("a security assessment" *is* a defined set of attack scenarios). `Plan` is disqualified by the `terraform plan` collision pervasive in this codebase. `Playbook` connotes an ordered procedure, contradicting the parallel/independent scenario model. `Suite` is accurate but flavorless and smuggles back a "test" prefix.

### Decision 3: Keep `scenario` and `expectation`; drop "assertion"
The two YAML-locked words map onto GitHub's job/step and stay. Following the Gomega split (`Expect(x).To(Equal(y))`): **Expectation** = the declared check (YAML key) *and* its outcome, **Matcher** = the pluggable mechanism that performs the check. "Assertion" sat redundantly between them and collided with both; it is removed.
- `scenario.Assertions` → `scenario.Matchers`; `FailedAssertions` → `UnmetExpectations`.
- `ElasticSecurityAlertGeneratedAssertion` → `ElasticSecurityAlertMatcher`.
- `AssertionResult` / `LatestAssertionResult` → `ExpectationResult`.
- "Assertion" is currently spread across **three** wire/storage identifiers that must all converge on `expectations`: the `scenario_results.assertions` JSONB column, the run-result JSON key `matchers` (`results/types.go`), and the lint JSON count key `assertions` (`web/types.go`). "Matcher" is retained only for the mechanism (the `AlertGeneratedMatcher` interface and its impls), not for results.

### Decision 4: One fan-out layer, one result type, no input mutation
`Runner` executes exactly one scenario and **returns** a consolidated `ScenarioResult`; it no longer mutates the `Scenario` input. The worker pool in `internal/results` is the sole fan-out. The two near-identical **in-memory** result types (`runner.ScenarioResult`, `results.ScenarioRunResult`) collapse into one `ScenarioResult`; `SimrunRunResult` → `RunResult`. The persistence row `db.ScenarioResult` (raw-JSON columns + DB-only fields like `Status`/`Phase`/`CreatedAt`) is **kept as a separate DTO** so `db` stays decoupled from `runner`/`matchers`; the existing `web.buildScenarioResultRow` remains the single marshal boundary. (Folding the row DTO into the in-memory type would invert the layering — `db` importing the domain — and force one struct to carry fields half its callers ignore; rejected per Rule 2/Rule 6.)

### Decision 5: Extract `Eventually(fn, interval, deadline)`
One poll-until-satisfied primitive shared by the assert and explore paths.

### Deferred: mode unification (separate change)
A clean-up of how execution mode is selected — today three disconnected mechanisms: the `type` column (storage/list-filter only, never read at run time), the per-run `exploreMode` bool, and the `" - collect mode"` expectation-name suffix — into one authoritative `Mode` enum is **out of scope here**. Verified that `type` does not drive runtime behavior today, so unifying it is a *behavior change* (it would wire `type` to execution and require migrating existing collect scenarios). It is tracked as its own change. This change leaves `exploreMode`/`cleanupAlerts` and the collect-mode suffix untouched.

### Canonical rename map

| Concept | Old | New |
|---|---|---|
| Saved definition | `SavedScenario` / `saved_scenarios` / `/api/scenarios` | `Assessment` / `assessments` / `/api/assessments` |
| Definition address | by UUID | by unique `name` |
| Execution | `Run` / `runs` / `/api/runs` | **unchanged** (`/api/runs` + `/api/assessments/{id}/runs`) |
| Run ingress | `POST /api/scenarios/run` body `{scenarioId}` | `POST /api/runs` body `{assessmentId}` |
| Runs of one assessment | (none) | `GET /api/assessments/{id}/runs` (nested read) |
| Run→definition FK | `runs.scenario_id` | `runs.assessment_id` |
| Per-expectation outcome column | `scenario_results.assertions` (JSONB) | `scenario_results.expectations` |
| Run-result JSON key | `matchers` | `expectations` |
| Lint JSON count key | `assertions` | `expectations` |
| Per-case outcome (in-memory ×2) | `runner.ScenarioResult`, `results.ScenarioRunResult` | one in-memory `ScenarioResult` (`db.ScenarioResult` row DTO kept separate) |
| Run aggregate | `SimrunRunResult` | `RunResult` |
| Per-expectation outcome | `AssertionResult` / `LatestAssertionResult` | `ExpectationResult` |
| Runtime checks | `scenario.Assertions []AlertGeneratedMatcher` | `scenario.Matchers` |
| Failed checks | `FailedAssertions` | `UnmetExpectations` |
| Concrete matcher struct | `ElasticSecurityAlertGeneratedAssertion` | `ElasticSecurityAlertMatcher` |
| Retention keys | `assessment_(log_)retention_*` | `run_(log_)retention_*` |
| Mode encoding (`ExploreMode`/`CleanupAlerts`/suffix) | — | **unchanged** (deferred to a separate change) |
| Case noun (UNCHANGED) | `Scenario`, `scenario_results`, YAML `scenarios:` | unchanged |
| Declared check (UNCHANGED) | `Expectation`, YAML `expectations:` | unchanged |

## Risks / Trade-offs

- **Large breaking API surface in one change** → Stage by tranche (Migration Plan); the rename is mechanical and compiler-enforced in Go (wide but shallow).
- **Frontend/backend drift during rollout** → Land the API path/key renames together as one coordinated tranche, not piecemeal.
- **Retention key migration could drop operator values** → Migration copies old key values to new names before dropping old keys; covered by a spec scenario.
- **Unique `name` constraint breaks existing duplicate-named definitions** → Migration must detect collisions and disambiguate (e.g., suffix) before adding the UNIQUE index; surface any renames in migration logs (fail loud).
- **Spec deltas cover contract-changing requirements, not every incidental "run" mention** → Unchanged-behavior requirements (WebSocket keep-alive, backpressure, export) keep their semantics; their cosmetic wording is handled by the mechanical sweep in tasks.

## Migration Plan

Staged tranches, each independently shippable:

- **Tranche A (zero user impact):** consolidate the three result types into one `ScenarioResult` (+ `RunResult`); runner returns instead of mutating; drop "assertion" → `Matcher`/`UnmetExpectations`/`ExpectationResult`; extract `Eventually`. Pure Go internal, behavior-preserving refactor.
- **Tranche B (lean runner):** collapse the vestigial `Runner.Run()` multi-scenario loop so `Runner` is single-scenario; the worker pool is the only fan-out.
- **Tranche C (the rename, coordinated):** DB migration `saved_scenarios` → `assessments` (+ UNIQUE `name`, with collision disambiguation), FK `runs.scenario_id` → `assessment_id`, column `scenario_results.assertions` → `expectations`; API `/api/scenarios*` → `/api/assessments*`, addressable by name; run ingress `POST /api/runs` body `{assessmentId}` plus nested read `GET /api/assessments/{id}/runs`; retention config-key rename to `run_(log_)retention_*` with value-carrying backfill; frontend surfaces **Assessments** (library) and **Runs** (history) and updates API-client paths/keys — landed together.

Rollback: tranches A/B are behavior-preserving refactors revertable via VCS. Tranche C's DB migrations ship with `down` migrations restoring the prior table/column/key names.

## Open Questions

- Slug form for `name`: raw string or slugified (lowercase, hyphenated)? Default: store `name` verbatim, require uniqueness, URL-encode in paths. Revisit if names with spaces/slashes prove awkward.
- Keep a temporary alias for `POST /api/scenarios/run` during transition? Default: replace; no known external API consumers.
- Inline-YAML (unsaved) runs do not exist today (every run references a saved definition). If ad-hoc runs are ever wanted, that is a *new feature* (`POST /api/runs` with a YAML body) — deliberately out of scope here so this stays a rename.
- `RunResponse` stays `{runId}` — renaming to `{id}` is cosmetic churn with frontend coupling and is not adopted.
- Dependent scenario chains (scenario B consumes scenario A's output) — the one place that would justify a DAG/workflow engine. Deferred to a separate proposal.
