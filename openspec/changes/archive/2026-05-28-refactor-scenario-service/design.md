## Context

`simrun/internal/web/scenarios.go` is 940 lines covering six distinct concerns:

1. HTTP-facing Run/Lint orchestration (`ScenarioService.Run`, `Lint`)
2. Per-cloud credential resolution (`buildConnectorCredentials`, the AWS/GCP/Azure/K8s/SSH switch, target-credentials, kubeconfig generation, `writeTempSSHKey`, `strVal`)
3. Connector env-var loading (`resolveElasticConnectorEnv`, `loadSecrets`)
4. Scenario-result DTO construction (the ~100-line inline block inside the goroutine that turns `results.ScenarioRunResult` into `db.ScenarioResult`)
5. Async run lifecycle (the goroutine spawned at line 261)
6. Elasticsearch export (`exportResults`, `getElasticAPIKey`, `indexResults`)

Concern (2) is also duplicated in `simrun/internal/web/connector_handlers.go::testConnection` (lines 493-577) — both call sites implement the same per-cloud switch over `connector.Type`. Any change to credential handling for a given cloud must today be applied in both places, and the only thing preventing drift is reviewer attention.

The file is the literal entry point for the product's core verb (`POST /api/scenarios/run`), so it shapes every new contributor's mental model of the architecture. This was tolerable in a closed codebase; for OSS this is the first file outside reviewers will open.

## Goals / Non-Goals

**Goals:**
- Reduce `scenarios.go` to ~250 lines covering only Run/Lint orchestration.
- Have exactly one implementation of per-cloud credential resolution in the codebase, shared by `ScenarioService.Run` and `connector_handlers.go::testConnection`.
- Keep the file structure obvious for adding a future export backend (e.g., Splunk) without needing a speculative interface today.
- Preserve all existing test coverage; add unit tests for the extracted credential resolver per cloud type.

**Non-Goals:**
- No behavior changes. No HTTP contract changes. No DB schema changes. No env-var changes.
- No new Go module dependencies; no dependency removals (those belong in separate changes).
- No introduction of an `Exporter` interface. With one impl, an interface would be speculative; we name the seam (`ResultExporter` + internal switch on `connector.Type`) so adding a second backend later is mechanical, but defer the interface until two real impls exist.
- No fix for the unrelated audit findings in this file (context-cancellation on the run goroutine, hardcoded `asp.results` datastream, panic-on-pack-author-error). Those are tracked as separate changes.
- No subagent extraction of `Lint`, `loadPacksFromDB`, or other 20-40 line helpers — extracting them would be cosmetic.

## Decisions

### D1. Credential resolution moves to a new `simrun/internal/credentials` package

A new package `simrun/internal/credentials` exposes a `Resolver` type:

```go
type Resolver struct {
    connectorStore db.ConnectorStore
    secretStore    db.SecretStore
    encryptor      *crypto.Encryptor
}

func NewResolver(connectorStore db.ConnectorStore, secretStore db.SecretStore, encryptor *crypto.Encryptor) *Resolver

// Build returns the env-var map for a single connector.
// Equivalent to today's buildConnectorCredentials.
func (r *Resolver) Build(ctx context.Context, connector *db.Connector) (map[string]string, error)

// BuildTargets resolves a YAML targets map (cloud-type → connector name)
// to merged env vars. Equivalent to today's resolveTargetCredentials.
func (r *Resolver) BuildTargets(ctx context.Context, targets map[string]string) (map[string]string, error)

// LoadAllSecrets decrypts every secret group and returns a flat map.
// Equivalent to today's loadSecrets.
func (r *Resolver) LoadAllSecrets(ctx context.Context) map[string]string

// ResolveElasticEnv returns SR_KIBANA_URL/SR_ELASTIC_*  env vars from the
// first enabled elastic connector. Equivalent to today's resolveElasticConnectorEnv.
func (r *Resolver) ResolveElasticEnv(ctx context.Context) map[string]string
```

Kubernetes-specific logic (`resolveKubeconfigPath`, currently parameterised by a `buildCreds` function so it can be called from both `ScenarioService` and `connector_handlers.go`) becomes a regular method `(r *Resolver) buildKubernetesCredentials(...)` that calls back to `r.Build` directly — the function-parameter indirection was only there because the implementation lived on `ScenarioService`.

**Alternatives considered:**
- *Keep credentials in `web/` as another file in the same package.* Rejected because `connector_handlers.go::testConnection` is the duplicate caller and lives in `web/` too — extracting to a sibling file would clean up `scenarios.go` but not solve the duplication. A separate package forces both callers to import the same symbol.
- *Put credentials in `simrun/internal/connectors/`*. Tempting — there's already an analogous `simrun/internal/cloud/{aws,gcp,azure,k8s}` package set. Rejected because `internal/connectors/` doesn't exist yet (connector logic currently lives in `web/`), and creating it would expand the scope of this change to "all connector logic moves out of web/". Defer that to a follow-up.

### D2. Elastic export moves to `simrun/internal/web/scenario_export.go`, struct named `ResultExporter`

The struct is named for the action (`ResultExporter`), not the backend (`ElasticExporter`). Its `Export` method takes `(ctx, runID, results)` — a signature that works for any future backend without change. Internal dispatch is a switch on `connector.Type`, currently with only an `"elastic"` case.

```go
type ResultExporter struct {
    connectorStore db.ConnectorStore
    encryptor      *crypto.Encryptor
    secretStore    db.SecretStore
}

func (e *ResultExporter) Export(ctx context.Context, runID uuid.UUID, results []results.ScenarioRunResult) {
    connectors, _ := e.connectorStore.List(ctx)
    for _, c := range connectors {
        if !c.Enabled { continue }
        switch c.Type {
        case "elastic":
            e.exportToElastic(ctx, c, runID, results)
        }
    }
}
```

**Alternatives considered:**
- *Introduce `BackendExporter` interface now.* Rejected per CLAUDE.md Rule 2 (no abstractions for single-use code) and because the right interface shape is unknown without a second implementation. Splunk HEC, Datadog Logs API, and S3 file-write all have different batching/error/auth semantics; designing the interface from Elastic alone would almost certainly be wrong.
- *Keep export inline in `ScenarioService`.* Rejected — it's the single largest concern in the file (~160 lines) and has zero coupling to run orchestration beyond the final `Export(ctx, runID, results)` call.

### D3. Scenario-result DTO construction moves to `simrun/internal/web/scenario_results.go`

The ~100-line block inside the run-result callback (marshaling assertions, indicators, metadata, discovered-alerts; building the `db.ScenarioResult`) becomes a pure function:

```go
func buildScenarioResultRow(runID uuid.UUID, result *results.ScenarioRunResult) *db.ScenarioResult
```

The callback in `Run()` then reduces to:
```go
row := buildScenarioResultRow(runID, result)
if dbID, ok := scenarioDBIDs[result.Name]; ok {
    s.runStore.CompleteScenarioResult(context.Background(), dbID, row)
} else {
    s.runStore.AddScenarioResult(context.Background(), runID, row)
}
s.runStore.IncrementRunCounters(...)
```

Pure function (no receiver, no I/O), trivially unit-testable.

### D4. `ScenarioService` constructor signature

After extraction, the struct holds:

```go
type ScenarioService struct {
    runStore       db.RunStore
    scenarioStore  db.ScenarioStore
    packStore      db.PackStore
    configStore    db.ConfigStore
    creds          *credentials.Resolver
    exporter       *ResultExporter
    hub            *Hub
    runLogRegistry *RunLogRegistry
    dataDir        string
}
```

`secretStore`, `connectorStore`, and `encryptor` move out of `ScenarioService`'s direct deps into `credentials.Resolver` and `ResultExporter`. Wiring in `cmd/simrun/main.go` adds two constructor calls (`credentials.NewResolver(...)`, `NewResultExporter(...)`) and passes them into `NewScenarioService(...)`.

### D5. Caller of credential resolution in `connector_handlers.go`

`testConnection` (currently lines 493-577 of `connector_handlers.go`) today re-implements the per-cloud switch. After the refactor it becomes:

```go
func (s *Server) testConnection(...) {
    // ... parse request, look up connector ...
    creds, err := s.credResolver.Build(ctx, connector)
    if err != nil { /* HTTP error */ }
    // ... existing test logic that uses the env vars ...
}
```

The `Server` struct (or wherever `testConnection` is defined — currently it's a free function with a closure over stores) gains a `credResolver *credentials.Resolver` field.

## Risks / Trade-offs

- [**Risk**: Forgetting a caller of the duplicated credential logic.] → Mitigation: tasks include an explicit grep for `buildConnectorCredentials`, `resolveTargetCredentials`, `resolveKubernetesCredentials`, `loadSecrets`, and `resolveElasticConnectorEnv` after extraction; all hits must resolve to either the new `credentials` package or callers of it.
- [**Risk**: Subtle behavior drift in the credentials switch when moving code.] → Mitigation: extraction is mechanical — copy the switch body verbatim into `Resolver.Build`. New unit tests per cloud type assert env-var keys and basic values to lock the contract before any further changes.
- [**Risk**: Constructor wiring in `main.go` is fiddly.] → Mitigation: small, single commit just for wiring; integration test (`go run ./simrun/cmd/simrun --help` plus a smoke run) confirms startup.
- [**Trade-off**: New package `simrun/internal/credentials` adds an import edge.] → Acceptable. It's a leaf package with three deps (`db`, `crypto`, the `cloud/*` packages) and no consumers outside `web/`.
- [**Trade-off**: `ResultExporter` is not behind an interface.] → Acceptable per D2 and CLAUDE.md Rule 2. Cost of introducing the interface later (when a second backend lands) is trivial because the call site already has a single-method API.

## Migration Plan

No data migration; no API contract change. The deploy path is a single binary swap. Rollback is the previous binary. No feature flag needed.
