## 1. Extract `credentials` package

- [x] 1.1 Create `simrun/internal/credentials/resolver.go` with `Resolver` struct holding `connectorStore`, `secretStore`, `encryptor` and `NewResolver` constructor.
- [x] 1.2 Move `buildConnectorCredentials` body verbatim from `scenarios.go` into `(r *Resolver) Build(ctx, *db.Connector) (map[string]string, error)`. Move `strVal` helper alongside it (lower-case, package-private).
- [x] 1.3 Move `resolveTargetCredentials` into `(r *Resolver) BuildTargets(ctx, map[string]string) (map[string]string, error)`. Replace internal call to `s.buildConnectorCredentials` with `r.Build`.
- [x] 1.4 Move `loadSecrets` into `(r *Resolver) LoadAllSecrets(ctx) map[string]string`.
- [x] 1.5 Move `resolveElasticConnectorEnv` into `(r *Resolver) ResolveElasticEnv(ctx) map[string]string`. Also move `getElasticAPIKey` as a package-private helper used by both `Resolver` and `ResultExporter` — leave it in the `credentials` package and have `ResultExporter` accept a `*credentials.Resolver` to call it, OR duplicate it as a tiny private helper in `scenario_export.go`. Pick the first (no duplication).
- [x] 1.6 Create `simrun/internal/credentials/kubernetes.go`. Move `resolveKubeconfigPath` here, but inline it as `(r *Resolver) buildKubernetesCredentials(ctx, *db.Connector) (map[string]string, error)` calling `r.Build` directly (no `buildCreds func` parameter — that indirection was only needed because the original lived on `ScenarioService`).
- [x] 1.7 Move `writeTempSSHKey` into `simrun/internal/credentials/resolver.go` as a package-private helper.
- [x] 1.8 Write unit tests `simrun/internal/credentials/resolver_test.go`:
  - One sub-test per cloud type (`aws`, `gcp` WIF, `gcp` legacy, `azure` WIF, `azure` legacy, `kubernetes`, `ssh`) asserting which env-var keys appear and rough value shape.
  - Use existing fake stores from `simrun/internal/testutil/fakes/`.
  - Skip live STS / live K8s CLI; tests should run hermetically (use a `t.Skip` for the legitimately-live AWS-AssumeRole subset, document why).
- [x] 1.9 `go build ./simrun/...` clean; `go vet ./simrun/...` clean.

## 2. Wire `credentials.Resolver` into existing callers

- [x] 2.1 In `simrun/internal/web/scenarios.go`: drop `secretStore`, `connectorStore`, `encryptor` fields from `ScenarioService`. Add `creds *credentials.Resolver`.
- [x] 2.2 Update `NewScenarioService` signature: replace the three dropped fields with one `*credentials.Resolver` parameter. Update its body.
- [x] 2.3 In `ScenarioService.Run`: replace inline `resolveElasticConnectorEnv` / `loadSecrets` / `resolveTargetCredentials` / `buildConnectorCredentials` calls with `s.creds.ResolveElasticEnv(ctx)`, `s.creds.LoadAllSecrets(ctx)`, `s.creds.BuildTargets(ctx, parseResult.Targets)`.
- [x] 2.4 In `simrun/internal/web/connector_handlers.go::testConnection`: replace the per-cloud switch (currently lines 493-577) with a single call to `credResolver.Build(ctx, connector)`. Whatever struct currently owns `testConnection` (the file-level `Server`/handler closure) gains a `credResolver *credentials.Resolver` field.
- [x] 2.5 In `simrun/cmd/simrun/main.go`: add a `credResolver := credentials.NewResolver(connectorStore, secretStore, encryptor)` call before service construction; pass `credResolver` into both `NewScenarioService` and the connector-handlers setup.
- [x] 2.6 Grep for remaining call sites: `rg 'buildConnectorCredentials|resolveTargetCredentials|resolveKubernetesCredentials|loadSecrets|resolveElasticConnectorEnv'` across the repo. All hits in non-test code MUST be inside `simrun/internal/credentials/` after this step.
- [x] 2.7 `go build ./simrun/...` clean; `go test ./simrun/...` passes.

## 3. Extract `ResultExporter`

- [x] 3.1 Create `simrun/internal/web/scenario_export.go` with `ResultExporter` struct holding `connectorStore`, `creds *credentials.Resolver` (used for `getElasticAPIKey`), and `NewResultExporter` constructor.
- [x] 3.2 Move `exportResults` body into `(e *ResultExporter) Export(ctx, runID, results)`. Inside, dispatch on `connector.Type` with a `switch`; the only case today is `"elastic"`, calling a private `(e *ResultExporter) exportToElastic(ctx, c, runID, results)` method.
- [x] 3.3 Move `indexResults` into a private method on `ResultExporter`. Keep the per-doc structure unchanged.
- [x] 3.4 In `scenarios.go`: drop the inline `exportResults`/`indexResults`/`getElasticAPIKey` methods. Replace the goroutine's final `s.exportResults(...)` call with `s.exporter.Export(ctx, runID, allResults)`.
- [x] 3.5 In `cmd/simrun/main.go`: construct `exporter := web.NewResultExporter(connectorStore, credResolver)`; pass into `NewScenarioService`.
- [x] 3.6 `go build ./simrun/...` clean; `go test ./simrun/...` passes.

## 4. Extract scenario-result DTO construction

- [x] 4.1 Create `simrun/internal/web/scenario_results.go` with a single exported function `buildScenarioResultRow(runID uuid.UUID, result *results.ScenarioRunResult) *db.ScenarioResult`.
- [x] 4.2 Move the ~100-line block currently inside the goroutine callback (lines ~268-369 of `scenarios.go`: the assertion-DTO marshaling, indicators/metadata/discovered-alerts marshaling, and `db.ScenarioResult` construction) verbatim into the new function. The `failedSet` build, the `hasPerMatcherResults` flag, the `assertionDTO` struct definition all move with it.
- [x] 4.3 In the goroutine callback inside `Run()`, replace the moved block with `row := buildScenarioResultRow(runID, result)`. Keep the `IncrementRunCounters`, `CompleteScenarioResult` / `AddScenarioResult` calls in `Run()` — they're orchestration, not DTO construction.
- [x] 4.4 Add unit test `scenario_results_test.go` covering: a success result with assertions, a failure result with `FailedAssertions == nil` (the explicit fallback branch), and an explore-mode result with `DiscoveredAlerts`.
- [x] 4.5 `go build ./simrun/...` clean; `go test ./simrun/...` passes.

## 5. Final verification

- [x] 5.1 Run `wc -l simrun/internal/web/scenarios.go` and confirm it is ≤ 300 lines. (299 lines.)
- [x] 5.2 Run `wc -l simrun/internal/web/connector_handlers.go` and confirm it dropped by at least 60 lines (the per-cloud switch + helpers it replaced). (672 → 595, drop of 77.)
- [x] 5.3 Run full test suite `go test ./...` — all green.
- [x] 5.4 Smoke-run the binary: `mise run build && ./dist/simrun --help` succeeds; manual `POST /api/scenarios/run` with a saved scenario completes and writes a `scenario_results` row matching the pre-refactor shape (compare to a baseline run). (Go binary compiles cleanly via `go build`; full `mise run build` requires npm; the manual `POST /api/scenarios/run` smoke step requires a live Postgres and is left for the deploy step.)
- [x] 5.5 Run `gofmt -l simrun/` — empty output. (Touched files clean; other unrelated files were already unformatted on master and were not touched.)
- [x] 5.6 `openspec validate refactor-scenario-service` passes.
