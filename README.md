# simrun

simrun is an **Attack Simulation Platform (ASP)** for detection testing. It **detonates**
attack simulations and verifies that the security alerts you expect fire in your SIEM
(Elastic Security, Datadog).

It ships as a single Go binary serving a REST API + WebSocket interface backed by
PostgreSQL, with an embedded SvelteKit frontend.

## Getting Started

### Prerequisites

- [mise](https://mise.jdx.dev/) — manages the Go 1.25 and Node 22 toolchains (or install them yourself)
- PostgreSQL

### Build

```bash
mise run build   # builds the SvelteKit frontend and the simrun binary into dist/simrun
```

### Run

simrun requires a PostgreSQL database; schema migrations run automatically on startup.

```bash
export SR_DATABASE_URL="postgres://user:pass@localhost:5432/simrun?sslmode=disable"
./dist/simrun
```

The UI and API are then served on http://localhost:8080.

> Authentication is optional. Without `SR_GOOGLE_CLIENT_ID`/`SR_GOOGLE_CLIENT_SECRET`,
> login is disabled and the app runs unauthenticated — fine for local use, not for a
> shared deployment.

## Configuration

Deploy-time configuration is read from environment variables — the only `SR_*` env
surface. Everything else (connectors, secrets, packs, schedules, scenarios, app
defaults) lives in the database and is managed through the web UI.

| Variable | Required | Default | Description |
|---|---|---|---|
| `SR_DATABASE_URL` | yes | — | PostgreSQL connection string |
| `SR_WEB_PORT` | no | `8080` | HTTP listen port |
| `SR_DATA_DIR` | no | `~/.simrun` | Local data dir (encryption key, SSH logs) |
| `SR_ENCRYPTION_KEY_FILE` | no | `$SR_DATA_DIR/encryption.key` | Key file for encrypting stored secrets |
| `SR_DEBUG` | no | off | Verbose logging when set to a non-zero value |
| `SR_WEB_URL` | no | — | External base URL (used for OAuth redirects) |
| `SR_GOOGLE_CLIENT_ID` / `SR_GOOGLE_CLIENT_SECRET` | no | — | Google OAuth credentials (enables login) |
| `SR_GOOGLE_ALLOWED_DOMAIN` | no | — | Restrict OAuth login to a Google Workspace domain |
| `SR_AUTH_SESSION_TTL_HOURS` | no | `168` | Session lifetime in hours |

### Run with Docker

```bash
docker build -t simrun .
docker run -p 8080:8080 \
  -e SR_DATABASE_URL="postgres://..." \
  -v simrun-data:/home/nonroot/.simrun \
  simrun
```

The image bundles the `aws`, `gcloud`, and `az` CLIs used by detonators. Persist
`SR_DATA_DIR` (the volume above) so the secret-encryption key survives restarts.

## Architecture

A single Go binary handles:
- Simulation detonation and orchestration
- Alert matching and verification
- Log collection from security platforms
- Scenario parsing and execution

### Simulation Packs

Simulations are distributed as external packs, installed and managed via the web UI:

- simrun-base-pack — custom simulations (AWS, Azure, GCP)
- simrun-stratus-pack — [Stratus Red Team](https://github.com/DataDog/stratus-red-team) simulations

## Concepts

### Detonators
A **detonator** describes how and where an attack technique is executed.
* Simrun detonator — runs a simulation pack (Terraform-based; packs can themselves
  execute locally or over SSH)
* AWS CLI detonator — runs AWS CLI commands

### Injectors
An **injector** is an alternative to detonators: instead of executing the end-to-end
attack it takes a generated log message and injects it directly into the SIEM. This
covers cases where end-to-end simulation isn't feasible but you still want to confirm
the detection is operational.
* Elastic Injector

### Alert Matchers
An **alert matcher** is a platform-specific integration that checks whether an expected
alert was triggered.
* Elastic Security alerts
* Datadog security signals

### Collectors
A **collector** retrieves logs from security platforms after detonation for analysis and
rule generation.
* Elastic Collector — collects related logs from Elasticsearch by execution ID or
  user-agent correlation

### Detonation and Alert Correlation
Each detonation is assigned a UUID, reflected in the detonation where possible and used
to ensure the matched alert corresponds exactly to that detonation. If the detonator
cannot reflect the UUID, the matcher can correlate using indicators the user provides
(static indicators) or terraform output (dynamic indicators).

### Simulations
A **simulation** is a reusable module describing how to perform a specific attack.
Simulations are distributed as [packs](#simulation-packs) and installed via the web UI.

## Development

```bash
mise run build-frontend   # build just the SvelteKit frontend
go test ./...             # run the test suite
mise run lint             # run golangci-lint
go generate ./...         # regenerate mocks (mockery)
mise run parser           # regenerate parser from JSON schemas
```

## Contributing

Issues and pull requests are welcome.

## License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE).
