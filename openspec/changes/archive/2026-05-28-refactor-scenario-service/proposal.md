## Why

`simrun/internal/web/scenarios.go` has grown to 940 lines and mixes six unrelated concerns: HTTP-facing run/lint orchestration, per-cloud credential resolution, secret/env-var loading, scenario-result row construction, and Elasticsearch export. The per-cloud credential switch is also duplicated in `connector_handlers.go::testConnection` (lines 493-577), so any new connector type or auth mode must be edited in two places that silently drift. We're preparing the codebase for an OSS release where the first file an outside contributor opens to understand "what happens when you run a scenario" should not be a 940-line god object.

## What Changes

- Extract per-cloud credential resolution into a new `simrun/internal/credentials` package (`Resolver` type), wired into both `ScenarioService.Run` and `connector_handlers.go::testConnection` so the per-cloud switch lives in exactly one place.
- Extract Elasticsearch result export into a new `ResultExporter` type in `simrun/internal/web/scenario_export.go`. Internal dispatch keyed on `connector.Type` (only `"elastic"` today) so a future `splunk` or `datadog` case is mechanical — no speculative interface introduced yet.
- Extract the scenario-result DTO construction (~100 lines inside the async goroutine) into a `buildScenarioResultRow` helper in `simrun/internal/web/scenario_results.go`.
- Shrink `ScenarioService` to its true responsibility: HTTP-surface Run/Lint orchestration. Constructor argument list drops from 10 to ~8.
- No behavior changes. No API changes. No spec-level requirement changes. Pure refactor.

## Capabilities

### New Capabilities
<!-- None. This is a pure refactor; no user-visible behavior changes. -->

### Modified Capabilities
- `connectors`: ADD a requirement codifying the previously-implicit invariant that credential resolution for any connector is identical between the test-connection endpoint and scenario-run execution. Today this is "true by convention" because two code paths re-implement the same per-cloud switch; the refactor makes it structurally guaranteed.

## Impact

- **Code touched**:
  - `simrun/internal/web/scenarios.go` — shrinks from 940 to ~250 lines
  - `simrun/internal/web/connector_handlers.go` — `testConnection` switches to using the shared resolver (loses duplicated per-cloud switch)
  - New: `simrun/internal/credentials/resolver.go`, `simrun/internal/credentials/kubernetes.go`
  - New: `simrun/internal/web/scenario_export.go`, `simrun/internal/web/scenario_results.go`
  - `simrun/cmd/simrun/main.go` — constructor wiring update (one new `credentials.NewResolver(...)` call + adjusted `NewScenarioService(...)` args)
- **Tests**: existing tests under `simrun/internal/web/` continue to pass unchanged. New unit tests for `credentials.Resolver` per cloud type (currently untestable in isolation).
- **APIs**: no HTTP request/response shape changes, no DB schema changes, no env-var changes.
- **Dependencies**: no new Go module deps; no removed deps.
- **Risk**: low — purely structural. The main risk is forgetting a call site of the duplicated credential logic; mitigated by the task list requiring a grep for `buildConnectorCredentials` after extraction.
