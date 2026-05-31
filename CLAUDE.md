# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Core principles

Rule 1 — Think Before Coding.
- No silent assumptions.
- State what you're assuming.
- Surface tradeoffs.
- Ask before guessing.
- Push back when a simpler approach exists.

Rule 2 — Simplicity First.
- Minimum code that solves the problem.
- No speculative features.
- No abstractions for single-use code.
- If a senior engineer would call it overcomplicated — simplify.

Rule 3 — Surgical Changes.
- Touch only what you must.
- Don't "improve" adjacent code, comments, or formatting.
- Don't refactor what isn't broken.
- Match existing style.

Rule 4 — Goal-Driven Execution.
- Define success criteria.
- Loop until verified.
- Don't tell Claude what steps to follow, tell it what success looks like and let it iterate.

Rule 5 — Use the model only for judgment calls
- Use Claude for: classification, drafting, summarization, extraction from unstructured text.
- Do NOT use Claude for: routing, retries, status-code handling, deterministic transforms.
- If a status code already answers the question, plain code answers the question.

Rule 6 — Surface conflicts, don't average them
- If two existing patterns in the codebase contradict, don't blend them.
- Pick one (the more recent / more tested), explain why, and flag the other for cleanup.
- "Average" code that satisfies both rules is the worst code.

Rule 7 — Read before you write
- Before adding code in a file, read the file's exports, the immediate caller, and any obvious shared utilities.
- If you don't understand why existing code is structured the way it is, ask before adding to it.
- "Looks orthogonal to me" is the most dangerous phrase in this codebase.

Rule 8 — Tests verify intent, not just behavior
- Every test must encode WHY the behavior matters, not just WHAT it does.
- A test like `expect(getUserName()).toBe('John')` is worthless if the function takes a hardcoded ID.
- If you can't write a test that would fail when business logic changes, the function is wrong.

Rule 9 — Checkpoint after every significant step
- After completing each step in a multi-step task: summarize what was done, what's verified, what's left.
- Don't continue from a state you can't describe back to me.
- If you lose track, stop and restate.

Rule 10 — Match the codebase's conventions, even if you disagree
- If the codebase uses snake_case and you'd prefer camelCase: snake_case.
- If the codebase uses class-based components and you'd prefer hooks: class-based.
- Disagreement is a separate conversation. Inside the codebase, conformance > taste.
- If you genuinely think the convention is harmful, surface it. Don't fork it silently.

Rule 11 — Fail loud
- If you can't be sure something worked, say so explicitly.
- "Migration completed" is wrong if 30 records were skipped silently.
- "Tests pass" is wrong if you skipped any.
- "Feature works" is wrong if you didn't verify the edge case I asked about.
- Default to surfacing uncertainty, not hiding it.

## Project Overview

ASP (Attack Simulation Platform) is a detection testing framework. It detonates attack simulations and verifies that expected security alerts are generated in target platforms (Elastic Security, Datadog). The single shipped binary is `simrun`, a web server with a SvelteKit frontend backed by Postgres.

## Build and Development Commands

This project uses [mise](https://mise.jdx.dev/) for tool management (Go 1.25.8, mockery 3.5.5).

```bash
# Build the simrun binary (frontend + server)
mise run build

# Build only frontend
mise run build-frontend

# Regenerate parser from JSON schemas
mise run parser

# Generate mocks (uses mockery)
go generate ./...

# Run all tests
go test ./...

# Run a single test
go test -v ./simrun/internal/... -run TestName
```

## Architecture

### Core Components

**Detonators** (`simrun/internal/detonators/`) - Execute attack simulations:
- `SimrunDetonator` - Detonate using simulation packs (Terraform-based)
- `AWSCLIDetonator` - Execute AWS CLI commands
- `AWSDetonator` - Programmatic AWS SDK operations

**Injectors** (`simrun/internal/injectors/`) - Inject logs directly into SIEM:
- `ElasticInjector` - Inject documents into Elasticsearch

**Alert Matchers** (`simrun/internal/matchers/`) - Verify expected alerts:
- `elastic/` - Match Elastic Security Detection alerts
- `datadog/` - Match Datadog security signals

**Collectors** (`simrun/internal/collectors/`) - Collect logs after detonation:
- `ElasticCollector` - Collect related logs from Elasticsearch

**Parser** (`simrun/internal/parser/`) - Parse YAML scenario files into Scenario objects. Code is generated from JSON schemas via `mise run parser`. ParseOptions takes `Packs []config.PackConfig` plus run-scoped `EnvVars`, `DataDir`, `TerraformVersion`, `PackLogsEnabled`.

**Config** (`simrun/internal/config/`) - Type-only package with `Bootstrap` (env-only deploy config), `AppConfig` (DB-backed admin defaults), and `PackConfig`/`PackType` (in-memory parser/runner shapes). No more singleton.

**Connectors** (managed via `simrun/internal/web/connector_handlers.go`) - DB-backed connectors for Elastic, Datadog, AWS, GCP, Azure, Kubernetes, SSH; credentials live in linked secret groups.

**Packs** (`simrun/internal/packs/`) - Pack lifecycle management: install, upload, manifest parsing, parameter configuration.

**Pack-level parameters** (`simrun/pack/`):
- Built-in pack params (`default_tags`, `aws_region`, `gcp_region`, `azure_location`) ship with the SDK and are always present in every pack's manifest `params_schema`. Pack authors declare additional ones via `pack.RegisterPackParams(...pack.PackParam)` in `main()` — `gcp_project` is intentionally not a built-in because projects are org-specific.
- At manifest build time the SDK rewrites each sim's Terraform: for built-ins it ensures a `variable "<name>" {}` block is present (with the schema's type/default) and rewrites the matching provider/resource block to reference `var.<name>` (no-op when the cloud's provider block is absent). Custom params are author-declared `variable` blocks; the SDK does not modify them.
- At apply time the detonator passes every key from `packs.parameters` and every per-sim scenario param as `TF_VAR_<key>=...`. Precedence: TF variable default < pack-level value < per-sim scenario value. Map/array values are JSON-encoded.
- `PUT /api/packs/{name}/parameters` strict-validates declared keys (type, enum, required) against `params_schema`; unknown keys are kept and surfaced in the response's `unknown_keys` field as a soft warning. Manifest fetch failure or empty schema falls back to today's permissive storage.

### Scenario Configuration

Scenarios are defined in YAML files validated against JSON schemas in `simrun/schemas/`:
- `simrun.schema.json` - Main schema with `scenarios` array
- Each scenario has: `name`, `detonate`/`inject`, `expectations`, optional `indicators` and `collect`

### Execution Flow

1. UI submits a scenario run via `POST /api/scenarios/run`
2. `ScenarioService.Run()` loads `AppConfig` from DB, resolves the default Elastic connector + linked secret group into runEnv, and parses the YAML
3. Scenarios execute in parallel (parallelism comes from AppConfig or per-run override)
4. For each scenario:
   - Detonator/Injector executes the attack
   - Execution ID (UUID) is generated for correlation
   - Alert matchers poll for expected alerts until timeout
   - Optional collector gathers related logs
5. Results persisted to Postgres and broadcast over WebSocket; optionally exported to Elasticsearch

## Web Architecture

### Web Server

The web server provides a REST API + WebSocket interface for the SvelteKit frontend.

**Entry point:** `simrun/cmd/simrun/main.go`

**Packages:**
- `simrun/internal/web/` - HTTP server, handlers, WebSocket hub, frontend embedding
- `simrun/internal/db/` - PostgreSQL database layer (pgx + golang-migrate)
- `simrun/internal/results/` - Shared result types and parallel executor

### Database

PostgreSQL is used for persistence. Connection via `SR_DATABASE_URL` environment variable.

Tables: `runs`, `scenario_results`, `saved_scenarios`, `packs`, `app_config`, `secret_groups`, `schedules`, `connectors`, `auth_sessions`

Migrations run automatically on server startup (embedded SQL files in `simrun/internal/db/migrations/`).


### Frontend

SvelteKit app at `web/frontend/` with shadcn-svelte components, dark mode via `mode-watcher`. Built to static assets and served by a separate nginx container that also proxies `/api/` requests to the Go backend.

**Component library:** Before implementing any UI element, check https://shadcn-svelte.com/docs/components.md for available pre-built components. Install new components with `npx shadcn-svelte@latest add <component> --yes`. Use `@lucide/svelte` for icons instead of inline SVGs. Use the shadcn `Alert` component for error messages, `Label` for form labels, and `Select` for dropdowns.
