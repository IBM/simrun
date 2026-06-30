# Concepts

An **Assessment** defines scenarios. Running it creates a **Run**. The run **executes each scenario** (in parallel, like jobs), and each scenario checks its **expectations** via **matchers**.

## Vocabulary

**Assessment**
: A saved collection of scenarios that describes what to detonate and what alerts to expect. Managed on the Assessments page (`/assessments`).

**Run**
: A single execution of an Assessment. Persists results to the database and is viewable on the Runs page (`/runs`).

**Scenario**
: One unit of work inside an Assessment: a detonation or injection step, plus one or more expectations. Scenarios in a run execute in parallel.

**Expectation**
: A declared assertion that a specific alert or signal must appear in the target platform after detonation. Expectations are defined per-scenario in the YAML file.

**Matcher**
: The component that checks whether an expectation was satisfied by polling the target platform until the alert appears or the timeout expires.

## Detonators

Detonators execute the attack simulation.

**SimRun detonator** — runs a pack simulation using Terraform. The pack can execute locally (on the SimRun host) or over SSH on a remote target. This is the primary detonation method and requires a pack to be installed.

## Injectors

Injectors skip the attack execution and push a document directly into the SIEM.

**Elastic Injector** — writes a log document into Elasticsearch. Use this to confirm that a detection rule is operational without actually running an attack: if the matcher finds the expected alert after injection, the rule is wired up correctly.

## Matchers

Matchers verify that an expectation fired after detonation or injection.

**Elastic Security alert matcher** — polls Kibana for a Detection Engine alert whose `kibana.alert.rule.name` matches the expected rule name.

## Collectors

Collectors retrieve related logs after detonation for post-hoc analysis and rule development.

**Elastic Collector** — queries Elasticsearch for log events correlated to the detonation, either by the execution UUID (when it was reflected into the activity) or by a user-agent string. The collected logs are stored with the run result and can be used to understand what raw data a detection rule would have to match.

## Correlation

Every detonation is assigned a UUID (nanoid) at execution time. Where possible, SimRun reflects that UUID into the generated activity by injecting it into the user-agent string — so that the resulting alert maps unambiguously to exactly one detonation event.

When the UUID cannot be reflected into the activity (for example, in a managed AWS API call), SimRun uses **indicators** for correlation:

- `static` — a fixed string you provide (e.g. a known username or resource name) that will appear in the alert or log.
- `terraformOutput` — a value extracted dynamically from a Terraform output after the simulation completes (e.g. a generated resource ARN).

## Packs, Connectors, Secrets

**Packs** distribute simulations as versioned bundles. A pack contains Terraform modules, scenario definitions, and a manifest. Install packs from the UI and reference them by name in your scenario YAML. See [packs.md](packs.md) for installation and configuration.

**Connectors** point SimRun at an external platform — Elastic, AWS, GCP, Azure, Kubernetes, or SSH. Each connector stores the endpoint and non-secret configuration needed to reach that platform. Connectors are managed on the Connectors page (`/connectors`). See [connectors-and-secrets.md](connectors-and-secrets.md) for setup.

**Secret groups** hold the credentials (API keys, passwords, certificates) that a connector uses. A connector links to at most one secret group. Secrets are managed on the Secrets page (`/secrets`) and are never returned in plaintext after saving.

## See also

- [walkthrough.md](walkthrough.md) — run your first detection test end to end
- [scenarios.md](scenarios.md) — full YAML reference for scenario files
