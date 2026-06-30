<p align="center">
  <img src="docs/assets/simrun-icon-rounded.png" alt="SimRun" width="120">
</p>

<h1 align="center">SimRun</h1>

<p align="center">
  <strong>An Attack Simulation Platform for detection engineering.</strong><br>
  Detonate attacks, verify the alerts fire, and measure exactly which detections you're testing — across multiple clouds.
</p>

<p align="center">
  <a href="LICENSE"><img src="https://img.shields.io/badge/License-Apache%202.0-blue.svg" alt="License"></a>
  <a href="https://github.com/IBM/simrun/releases"><img src="https://img.shields.io/github/v/release/IBM/simrun" alt="Release"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Go-1.25-00ADD8?logo=go&logoColor=white" alt="Go"></a>
  <a href="https://github.com/IBM/simrun/actions/workflows/docker-publish.yml"><img src="https://github.com/IBM/simrun/actions/workflows/docker-publish.yml/badge.svg" alt="Docker"></a>
</p>

---

## Why SimRun

Detection rules rot silently. A field gets renamed, a log source drifts, an index mapping changes — and the alert you were counting on simply stops firing. You don't find out until the breach.

SimRun closes that gap. It runs real attack techniques against your environment, confirms the detections you expect actually fire in your SIEM, and tracks **which of your rules are tested and which are blind spots**.

It's not a script you wire into CI and forget. It's a platform:

- **🎯 Coverage you can see.** SimRun joins your live Elastic detection rules against your scenarios and their latest pass/fail result, then reports a coverage percentage. You learn what you *aren't* testing — not just whether one test passed. → [Rule Coverage](#rule-coverage)
- **☁️ Multiple clouds, multiple accounts.** First-class connectors for AWS, GCP, Azure, and Kubernetes, each backed by managed, encrypted secret groups and selectable per scenario.
- **🧪 Three ways to exercise a detection.** **Detonate** a real attack, **inject** a crafted log to confirm a rule is wired up without touching infrastructure, or **collect** the raw logs an attack produced to build the next rule.
- **📦 A pack ecosystem.** Simulations ship as versioned, shareable bundles with a clean Go SDK. Use the first-party packs, wrap [Stratus Red Team](https://github.com/DataDog/stratus-red-team), or author your own. → [Ecosystem](docs/ecosystem.md)
- **🖥️ Built to operate.** Web UI, REST API, and WebSocket interface; PostgreSQL persistence; scheduled runs; optional multi-user OAuth. It all ships as a single Go binary with the SvelteKit UI embedded.

> SimRun is **primarily focused on Elastic Security** — that's where coverage analysis, injection, and log collection are richest. Datadog security signals are supported as a matching backend.

## How it works

An **Assessment** is a saved set of **scenarios**. Running it creates a **Run**, which executes every scenario in parallel. Each scenario triggers some activity and then asserts that an expected alert appears in your SIEM before a timeout.

```mermaid
flowchart LR
    A[Assessment<br/>scenarios] -->|run| B(Run)
    B --> S1[Scenario 1]
    B --> S2[Scenario 2]
    B --> S3[Scenario N]
    S1 --> D{Trigger}
    D -->|detonate| E[Real attack<br/> from SimRun pack]
    D -->|inject| F[Crafted log<br/>into Elasticsearch]
    E --> M[Matcher polls SIEM<br/>until alert fires or timeout]
    F --> M
    M --> R[(Results + Coverage<br/>Postgres)]
    E -.optional.-> C[Collector<br/>gathers related logs]
    C --> R
```

Every detonation gets a UUID that SimRun reflects into the generated activity wherever possible (user-agent strings), so an alert maps unambiguously back to the exact attack that caused it.

→ Full vocabulary in [Concepts](docs/concepts.md).

## A scenario, end to end

Build scenarios visually in the assessment editor, or write the YAML directly — the editor toggles between a forms-based **Builder** and raw **YAML**. This scenario detonates a pack simulation in AWS and asserts the matching Elastic Security rule fires:

```yaml
targets:
  aws: prod-aws            # an AWS connector you configured in the UI

scenarios:
  - name: S3 public access block disabled
    detonate:
      simrunDetonator:
        pack: simrun-base-pack
        simulation: aws.s3-disable-public-access-block
    expectations:
      - timeout: 5m
        elasticSecurityAlert:
          name: "S3 Public Access Block Disabled"
```

Swap `detonate` for `inject` to test a rule without running an attack, or add a `collect` block to capture the raw logs the attack produced. 

→ [Scenarios reference](docs/scenarios.md).

## Quickstart (60 seconds)

**Prerequisites:** [mise](https://mise.jdx.dev/) (manages Go 1.25 and Node 22), PostgreSQL.

```bash
# Build the frontend + binary → dist/simrun
mise run build

# Point it at Postgres and run
export SR_DATABASE_URL="postgres://user:pass@localhost:5432/simrun?sslmode=disable"
./dist/simrun           # serves UI + API on http://localhost:8080
```

Schema migrations run automatically on startup. Authentication is optional — without Google OAuth credentials, SimRun runs unauthenticated.

→ Then follow the [Walkthrough](docs/walkthrough.md) to run your first detection test.

## Rule Coverage

The Coverage view (`/rules/coverage`) pulls every detection rule from your Elastic deployment and answers the question CI never could: **which of these rules does a SimRun scenario actually exercise, and did it pass last time?** Rules with no scenario are flagged as blind spots; the summary reports an overall covered-rules percentage.

## The pack ecosystem

Simulations are distributed as **packs** — versioned bundles of Terraform modules, scenario definitions, and a manifest. Two first-party packs are maintained alongside SimRun:

| Pack | What it is |
|---|---|
| [**simrun-pack**](https://github.com/confluentinc/simrun-pack) | The reference pack and a worked example of authoring simulations in Go (AWS, Kubernetes, Okta injections). |
| [**simrun-stratus-adapter**](https://github.com/confluentinc/simrun-stratus-adapter) | Plug-and-play wrapper that exposes the entire [Stratus Red Team](https://github.com/DataDog/stratus-red-team) technique registry as a SimRun pack. |


→ [Ecosystem & authoring guide](docs/ecosystem.md).

## Documentation

- [Getting Started](docs/getting-started.md) — prerequisites, build, first run
- [Concepts](docs/concepts.md) — assessments, runs, detonators, matchers, collectors
- [Walkthrough](docs/walkthrough.md) — end-to-end tutorial
- [Scenarios](docs/scenarios.md) — YAML schema reference
- [Connectors & Secrets](docs/connectors-and-secrets.md) — SIEM and cloud integrations
- [Packs](docs/packs.md) — install and configure simulation packs
- [Ecosystem](docs/ecosystem.md) — the pack ecosystem and writing your own
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

## Acknowledgments

SimRun stands on excellent prior work in the detection-engineering community:

- [**Threatest**](https://github.com/DataDog/threatest) by Datadog pioneered the detonate-and-verify model for detection testing and shaped how we think about correlating attacks to alerts.
- [**Stratus Red Team**](https://github.com/DataDog/stratus-red-team), also by Datadog, provides the MITRE ATT&CK–mapped cloud attack techniques that SimRun exposes through [simrun-stratus-adapter](https://github.com/confluentinc/simrun-stratus-adapter).

## Contributing

Issues and pull requests are welcome.

## License

Licensed under the Apache License 2.0. See [LICENSE](LICENSE).
