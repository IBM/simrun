<p align="center">
  <img src="docs/assets/simrun-icon-rounded.png" alt="SimRun" width="120">
</p>

<h1 align="center">SimRun</h1>

<p align="center">
  An Attack Simulation Platform (ASP) for detection testing — detonate attacks, verify the alerts fire.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://github.com/IBM/simrun/releases"><img src="https://img.shields.io/github/v/release/IBM/simrun" alt="Release"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/IBM/simrun/actions/workflows/docker-publish.yml"><img src="https://github.com/IBM/simrun/actions/workflows/docker-publish.yml/badge.svg" alt="Docker"></a>
</p>

## What is SimRun

SimRun is an **Attack Simulation Platform (ASP)** for detection testing. It **detonates** attack simulations and verifies that the security alerts you expect fire in your SIEM (Elastic Security and Datadog are supported).

It ships as a single Go binary serving a REST API + WebSocket interface backed by PostgreSQL, with an embedded SvelteKit frontend.

## Screenshot

![SimRun dashboard](docs/images/dashboard.png)

## Quickstart (60 seconds)

**Prerequisites:** [mise](https://mise.jdx.dev/) (manages Go 1.25 and Node 22 toolchains), PostgreSQL.

```bash
# Build
mise run build          # builds frontend + binary → dist/simrun

# Run
export SR_DATABASE_URL="postgres://user:pass@localhost:5432/simrun?sslmode=disable"
./dist/simrun           # serves UI + API on http://localhost:8080
```

Schema migrations run automatically on startup. Authentication is optional — without Google OAuth credentials the app runs unauthenticated.

Full guide → [docs/getting-started.md](docs/getting-started.md)

## How it works

An **Assessment** defines scenarios. Running it creates a **Run**. The run executes each scenario (in parallel, like jobs), and each scenario checks its expectations via **matchers**.

Read more → [docs/concepts.md](docs/concepts.md)

## Documentation

- [Getting Started](docs/getting-started.md) — prerequisites, build, first run
- [Concepts](docs/concepts.md) — assessments, runs, detonators, matchers, collectors
- [Walkthrough](docs/walkthrough.md) — end-to-end tutorial
- [Scenarios](docs/scenarios.md) — YAML schema reference
- [Connectors & Secrets](docs/connectors-and-secrets.md) — SIEM and cloud integrations
- [Packs](docs/packs.md) — simulation packs: install, configure, author
- [Configuration](docs/configuration.md) — environment variables reference
- [Deployment](docs/deployment.md) — Docker, production notes, OAuth setup

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
