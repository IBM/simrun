# SimRun Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a `/docs` Markdown tree (getting-started, concepts, hero walkthrough, and reference pages) and revamp `README.md` into a polished OSS front door with badges.

**Architecture:** Plain Markdown rendered natively by GitHub — no site generator. The README is the front door and links into `/docs`. The concepts page is the vocabulary source of truth; every other page reuses its terms and cross-links to it. Each doc page is one file with a single responsibility and is independently reviewable.

**Tech Stack:** Markdown (GitHub-flavored). No build tooling. Verification is link-existence checks and cross-referencing claims against the repo's code/schemas.

## Global Constraints

- Spec: `docs/superpowers/specs/2026-06-25-simrun-docs-design.md`.
- Vocabulary anchor — used verbatim as the opener of `docs/concepts.md` and consistently elsewhere: "An Assessment defines scenarios. Running it creates a Run. The run executes each scenario (in parallel, like jobs), and each scenario checks its expectations via matchers."
- Term map (verified): Assessment (`/assessments`), Run (`/runs`, `/runs/[id]`), Scenario, Expectation, Matcher, Detonator, Injector, Collector, Pack, Connector, Secret group.
- Real UI navigation (sidebar): Dashboard `/`, Runs `/runs`, Assessments `/assessments`, Packs `/packs`, Rule Coverage `/rules/coverage`, Connectors `/connectors`, Secrets `/secrets`, Config `/config`.
- Repo: `github.com/IBM/simrun`, license Apache-2.0, Go 1.25, Node 22, managed via mise. **Do not `git push`** (push is blocked); commit locally only.
- Deploy-time env surface is EXACTLY these 11 vars (from `internal/config/bootstrap.go`, "the ONLY surface that reads SR_* env vars"): `SR_DATABASE_URL` (required), `SR_DATA_DIR`, `SR_WEB_PORT`, `SR_ENCRYPTION_KEY_FILE`, `SR_AUTH_SESSION_TTL_HOURS`, `SR_DEBUG`, `SR_WEB_DEV`, `SR_WEB_URL`, `SR_GOOGLE_CLIENT_ID`, `SR_GOOGLE_CLIENT_SECRET`, `SR_GOOGLE_ALLOWED_DOMAIN`. Do NOT document any other `SR_*` var (the rest are test-only/non-surface).
- Logo is OUT OF SCOPE — README uses a text/placeholder header only.
- Accuracy over volume: every documented YAML field, env var, route, and pack parameter must correspond to something that exists in the code. Omit anything you cannot verify; never invent options. Fail loud — flag uncertainty in the PR rather than guessing.
- Screenshots: use clearly-marked placeholders `![alt](images/<name>.png)` pointing at `docs/images/`. Do not fabricate images.

### Verified scenario YAML reference (use these EXACT fields in Task 4 — do not add fields not listed)

Top-level (`schemas/simrun.schema.json`): `metadata` (`name`, `description`), `targets` (keys: `aws`, `gcp`, `azure`, `kubernetes`, `ssh` → connector names), `scenarios` (required).

Per scenario (required: `name`, `expectations`): `name`, `enabled` (bool, default true), `indicators` (`terraformOutput`: string[], `static`: string[]), `detonate` (oneOf `awsCliDetonator` | `simrunDetonator`), `inject` (oneOf `elasticInjector`), `collect` (oneOf `elasticCollector`), `expectations` (array).

- `awsCliDetonator` (`schemas/awsCliDetonator.schema.json`): `script` (string).
- `simrunDetonator` (`schemas/simrunDetonator.schema.json`, required `pack`, `simulation`): `pack`, `simulation` (e.g. `aws.exfil.s3`), `params` (object, free-form).
- `elasticInjector` (`schemas/elasticInjector.schema.json`, required `documents`): each document requires `index` and oneOf (`file`) | (`template` + `pack`); optional `vars` (string→string, `{{var}}` substitution).
- `elasticCollector` (`schemas/elasticCollector.schema.json`, required `index`): `index`, `additionalFields` (string→string, supports `{{ indicators.terraformOutput.<key> }}`).
- Expectation item (oneOf `datadogSecuritySignal` | `elasticSecurityAlert`) plus `timeout` (Go duration string, default `5m`):
  - `elasticSecurityAlert` (required `name`): `name` (exact match on `kibana.alert.rule.name.keyword`), `severity` (enum: low/medium/high/critical).
  - `datadogSecuritySignal` (required `name`): `name` (exact match), `severity` (string).

Worked examples to copy from: `internal/parser/testdata/scenarios/aws-cli-detonator.yaml`, `elastic-injector.yaml`, `elastic-collector.yaml`, `targets-all.yaml`.

### Verified pack parameters (use in Task 6)

Built-in pack params (always present, `pack/builtins.go`): `default_tags`, `aws_region`, `gcp_region`, `azure_location`. `gcp_project` is intentionally NOT built-in. Custom params declared via `pack.RegisterPackParams(...)`; reserved built-in names cannot be reused. Apply-time precedence: TF variable default < pack-level value < per-sim scenario value. Map/array values JSON-encoded. `PUT /api/packs/{name}/parameters` strict-validates declared keys; unknown keys kept and surfaced in `unknown_keys`.

### Verified env var table (use in Task 7 — README already has the prose form)

| Variable | Required | Default | Description |
|---|---|---|---|
| `SR_DATABASE_URL` | yes | — | PostgreSQL connection string |
| `SR_WEB_PORT` | no | `8080` | HTTP listen port |
| `SR_DATA_DIR` | no | `~/.simrun` | Local data dir (encryption key, SSH logs) |
| `SR_ENCRYPTION_KEY_FILE` | no | `$SR_DATA_DIR/encryption.key` | Key file for encrypting stored secrets |
| `SR_DEBUG` | no | off | Verbose logging when set to a non-zero/non-`false` value |
| `SR_WEB_DEV` | no | off | Dev mode when set to `1` |
| `SR_WEB_URL` | no | — | External base URL (used for OAuth redirects) |
| `SR_GOOGLE_CLIENT_ID` / `SR_GOOGLE_CLIENT_SECRET` | no | — | Google OAuth credentials (enables login) |
| `SR_GOOGLE_ALLOWED_DOMAIN` | no | — | Restrict OAuth login to a Google Workspace domain |
| `SR_AUTH_SESSION_TTL_HOURS` | no | `168` | Session lifetime in hours |

---

## Conventions for every task

- Each page starts with an `#` H1 title and a one-line summary sentence.
- Each page ends with a "Next steps" or "See also" list of relative links to sibling pages.
- Relative links between docs use bare filenames (e.g. `concepts.md`, `walkthrough.md`); links from README into docs use `docs/<file>.md`.
- After writing each page, run the link check (below) and fix any broken link before committing.
- Commit message style: `docs: <what>` (matches repo conventional-commit history).

**Link check command** (run from repo root; verifies every relative `.md` link target in a file exists):

```bash
f=docs/<page>.md; grep -oE '\]\(([^)]+\.md)(#[^)]*)?\)' "$f" | sed -E 's/\]\(([^)#]+).*/\1/' | while read -r l; do d=$(dirname "$f"); [ -f "$d/$l" ] && echo "OK   $l" || echo "MISS $l"; done
```
Expected: no `MISS` lines.

---

### Task 1: Getting Started page

**Files:**
- Create: `docs/getting-started.md`

**Interfaces:**
- Produces: `docs/getting-started.md` — linked from README quickstart, docs index, and walkthrough prerequisites.

- [ ] **Step 1: Write the page.** Sections, in order:
  1. Title + one-liner ("Install SimRun and reach the dashboard.").
  2. **Prerequisites** — mise (manages Go 1.25 + Node 22) or install them yourself; PostgreSQL. Link to https://mise.jdx.dev/.
  3. **Build** — `mise build` builds the SvelteKit frontend and the `simrun` binary into `dist/simrun`.
  4. **Run** — note migrations run automatically on startup; then:
     ```bash
     export SR_DATABASE_URL="postgres://user:pass@localhost:5432/simrun?sslmode=disable"
     ./dist/simrun
     ```
     UI + API served on http://localhost:8080.
  5. **Authentication is optional** — without `SR_GOOGLE_CLIENT_ID`/`SR_GOOGLE_CLIENT_SECRET`, login is disabled and the app runs unauthenticated. Link to `deployment.md` for OAuth.
  6. **Next steps** — links to `concepts.md`, `walkthrough.md`, `configuration.md`.

- [ ] **Step 2: Verify facts.** Confirm `mise build` exists: `grep -n "build" mise.toml`. Confirm port default `8080`: `grep -n "8080\|SR_WEB_PORT" internal/config/bootstrap.go`. Expected: both found.

- [ ] **Step 3: Run link check** on `docs/getting-started.md`. Expected: no `MISS`.

- [ ] **Step 4: Commit.**
  ```bash
  git add docs/getting-started.md && git commit -m "docs: add getting started guide"
  ```

---

### Task 2: Concepts page

**Files:**
- Create: `docs/concepts.md`

**Interfaces:**
- Produces: `docs/concepts.md` — the vocabulary source of truth; linked from nearly every other page.

- [ ] **Step 1: Write the page.** Sections, in order:
  1. Title + the vocabulary anchor sentence **verbatim** (see Global Constraints) as the opening paragraph.
  2. **Vocabulary** — definition list for: Assessment, Run, Scenario, Expectation, Matcher (each one sentence, with its UI route where applicable).
  3. **Detonators** — Simrun detonator (runs a pack simulation, Terraform-based; packs can run locally or over SSH) and AWS CLI detonator (runs AWS CLI commands).
  4. **Injectors** — Elastic Injector: instead of executing the attack, inject a log/document straight into the SIEM to confirm a detection is operational.
  5. **Matchers** — Elastic Security alerts and Datadog security signals; a matcher verifies an expectation fired.
  6. **Collectors** — Elastic Collector: retrieves related logs after detonation for analysis/rule generation.
  7. **Correlation** — each detonation gets a UUID, reflected into the activity where possible so the matched alert maps exactly to that detonation; when the UUID can't be reflected, correlate via user-provided `static` indicators or `terraformOutput` (dynamic) indicators.
  8. **Packs, Connectors, Secrets** — one paragraph each: packs distribute simulations (link `packs.md`); connectors point at a platform (link `connectors-and-secrets.md`); secret groups hold the credentials a connector uses.
  9. **See also** — links to `walkthrough.md`, `scenarios.md`.

- [ ] **Step 2: Verify the anchor sentence** is present byte-for-byte: `grep -F "each scenario checks its expectations via matchers" docs/concepts.md`. Expected: one match.

- [ ] **Step 3: Run link check** on `docs/concepts.md`. Expected: no `MISS`.

- [ ] **Step 4: Commit.**
  ```bash
  git add docs/concepts.md && git commit -m "docs: add concepts and vocabulary guide"
  ```

---

### Task 3: Connectors and Secrets page

**Files:**
- Create: `docs/connectors-and-secrets.md`

**Interfaces:**
- Produces: `docs/connectors-and-secrets.md` — referenced by walkthrough steps 2 and concepts.

- [ ] **Step 1: Verify supported connector types** before writing: `grep -rhoE 'Type:\s*"(aws|gcp|azure|kubernetes|ssh|elastic|datadog)"' internal | sort -u`. Document only the types that appear. Expected set: aws, gcp, azure, kubernetes, ssh, elastic, datadog.

- [ ] **Step 2: Write the page.** Sections:
  1. Title + one-liner ("Point SimRun at your platforms and store the credentials securely.").
  2. **Connectors vs secret groups** — a connector (`/connectors`) describes a target platform; its credentials live in a linked secret group (`/secrets`). Secrets are encrypted at rest using the key in `SR_DATA_DIR` (link `configuration.md`).
  3. **SIEM connectors** — Elastic (the alert source/SIEM) and Datadog: what each needs (Elastic: URL + auth; Datadog: API/app keys + site). Keep field descriptions high-level and UI-driven; do not invent exact field names you can't confirm in `internal/web/connector_handlers.go` — verify with `grep -in "elastic\|datadog\|api_key\|url" internal/web/connector_handlers.go` and describe only confirmed fields.
  4. **Cloud connectors** — AWS, GCP, Azure: used by detonators for cloud credentials; referenced from a scenario's `targets`.
  5. **Other connectors** — Kubernetes, SSH (SSH used for remote command detonation).
  6. **See also** — `walkthrough.md`, `packs.md`, `scenarios.md`.

- [ ] **Step 3: Run link check** on `docs/connectors-and-secrets.md`. Expected: no `MISS`.

- [ ] **Step 4: Commit.**
  ```bash
  git add docs/connectors-and-secrets.md && git commit -m "docs: add connectors and secrets guide"
  ```

---

### Task 4: Scenarios YAML reference page

**Files:**
- Create: `docs/scenarios.md`

**Interfaces:**
- Produces: `docs/scenarios.md` — referenced by walkthrough and concepts.

- [ ] **Step 1: Write the page** using ONLY the "Verified scenario YAML reference" fields in Global Constraints. Sections:
  1. Title + one-liner ("The YAML shape behind every assessment.").
  2. **Top-level structure** — `metadata`, `targets`, `scenarios` (table: field, type, required, description).
  3. **A scenario** — `name`, `enabled`, `indicators`, `detonate`/`inject`, `collect`, `expectations`.
  4. **Detonators** — `awsCliDetonator` (`script`) and `simrunDetonator` (`pack`, `simulation`, `params`) with a YAML snippet each (copy from `internal/parser/testdata/scenarios/aws-cli-detonator.yaml`).
  5. **Injector** — `elasticInjector` (`documents[]` with `index` + `file` OR `template`+`pack`, optional `vars`); snippet from `elastic-injector.yaml`.
  6. **Collector** — `elasticCollector` (`index`, `additionalFields` with `{{ indicators.terraformOutput.<key> }}`); snippet from `elastic-collector.yaml`.
  7. **Expectations** — `timeout` (Go duration, default `5m`) plus a matcher: `elasticSecurityAlert` (`name`, `severity` enum low/medium/high/critical) or `datadogSecuritySignal` (`name`, `severity`).
  8. **Full example** — paste `internal/parser/testdata/scenarios/targets-all.yaml` and walk through it.
  9. **See also** — `concepts.md`, `connectors-and-secrets.md`.

- [ ] **Step 2: Cross-check no invented fields.** Re-read `schemas/simrun.schema.json` and the referenced sub-schemas; confirm every field documented appears in a schema. List any field in your page not found in a schema and delete it.

- [ ] **Step 3: Verify example snippets match real files:** `cat internal/parser/testdata/scenarios/targets-all.yaml` and confirm the page's pasted copy is identical.

- [ ] **Step 4: Run link check** on `docs/scenarios.md`. Expected: no `MISS`.

- [ ] **Step 5: Commit.**
  ```bash
  git add docs/scenarios.md && git commit -m "docs: add scenario YAML reference"
  ```

---

### Task 5: Packs page

**Files:**
- Create: `docs/packs.md`

**Interfaces:**
- Produces: `docs/packs.md` — referenced by walkthrough step 3 and concepts.

- [ ] **Step 1: Write the page** using the "Verified pack parameters" facts. Sections:
  1. Title + one-liner ("Install simulations and tune their parameters.").
  2. **What a pack is** — external bundle of simulations, installed/managed at `/packs`.
  3. **Shipped packs** — `simrun-base-pack` (custom AWS/Azure/GCP simulations) and `simrun-stratus-pack` ([Stratus Red Team](https://github.com/DataDog/stratus-red-team)).
  4. **Installing a pack** — via the Packs page in the UI.
  5. **Pack parameters** — built-ins (`default_tags`, `aws_region`, `gcp_region`, `azure_location`); note `gcp_project` is org-specific and NOT a built-in; custom params via `RegisterPackParams`.
  6. **Precedence** — TF variable default < pack-level value < per-sim scenario value; map/array values JSON-encoded.
  7. **Validation** — `PUT /api/packs/{name}/parameters` strict-validates declared keys; unknown keys are kept and surfaced under `unknown_keys` as a soft warning.
  8. **See also** — `walkthrough.md`, `scenarios.md`.

- [ ] **Step 2: Verify built-in names:** `grep -nE '"(default_tags|aws_region|gcp_region|azure_location)"' pack/builtins.go`. Expected: all four found. Confirm `gcp_project` is not registered as a built-in.

- [ ] **Step 3: Run link check** on `docs/packs.md`. Expected: no `MISS`.

- [ ] **Step 4: Commit.**
  ```bash
  git add docs/packs.md && git commit -m "docs: add packs guide"
  ```

---

### Task 6: Configuration reference page

**Files:**
- Create: `docs/configuration.md`

**Interfaces:**
- Produces: `docs/configuration.md` — referenced by getting-started, deployment, connectors pages, README.

- [ ] **Step 1: Write the page.** Sections:
  1. Title + one-liner ("Every deploy-time setting SimRun reads.").
  2. Intro: deploy-time config is env-only (the only `SR_*` surface); everything else (connectors, secrets, packs, schedules, assessments, app defaults) lives in the database and is managed in the UI.
  3. **Environment variables** — paste the verified env var table from Global Constraints exactly.
  4. **App config defaults** — DB-backed admin defaults managed at `/config` (e.g. scenario parallelism). Describe at a high level; do not invent specific field names beyond parallelism unless confirmed via `grep -in "parallel\|AppConfig" internal/config/appconfig.go`.
  5. **See also** — `deployment.md`, `getting-started.md`.

- [ ] **Step 2: Verify the env table is complete and correct** against the source: `grep -oE '"SR_[A-Z_]+"' internal/config/bootstrap.go | sort -u`. Confirm every var the table lists appears here, and that no `SR_*` var read by bootstrap.go is missing from the table. Expected match: the 11 vars in Global Constraints.

- [ ] **Step 3: Run link check** on `docs/configuration.md`. Expected: no `MISS`.

- [ ] **Step 4: Commit.**
  ```bash
  git add docs/configuration.md && git commit -m "docs: add configuration reference"
  ```

---

### Task 7: Deployment page

**Files:**
- Create: `docs/deployment.md`

**Interfaces:**
- Produces: `docs/deployment.md` — referenced by getting-started, configuration, README.

- [ ] **Step 1: Verify Docker facts:** `cat Dockerfile` — confirm the image bundles `aws`, `gcloud`, `az` CLIs and the data dir path (`/home/nonroot/.simrun`). Document only what the Dockerfile actually shows.

- [ ] **Step 2: Write the page.** Sections:
  1. Title + one-liner ("Run SimRun in production with Docker and optional auth.").
  2. **Docker** —
     ```bash
     docker build -t simrun .
     docker run -p 8080:8080 \
       -e SR_DATABASE_URL="postgres://..." \
       -v simrun-data:/home/nonroot/.simrun \
       simrun
     ```
     Note the image bundles `aws`, `gcloud`, `az` (used by detonators). Persist `SR_DATA_DIR` (the volume) so the secret-encryption key survives restarts.
  3. **Authentication (Google OAuth)** — set `SR_GOOGLE_CLIENT_ID`/`SR_GOOGLE_CLIENT_SECRET` to enable login; `SR_WEB_URL` for the OAuth redirect base; `SR_GOOGLE_ALLOWED_DOMAIN` to restrict to a Workspace domain; `SR_AUTH_SESSION_TTL_HOURS` for session lifetime. Link `configuration.md`.
  4. **Data persistence** — the encryption key in `SR_DATA_DIR` must persist or stored secrets become unreadable.
  5. **See also** — `configuration.md`, `getting-started.md`.

- [ ] **Step 3: Run link check** on `docs/deployment.md`. Expected: no `MISS`.

- [ ] **Step 4: Commit.**
  ```bash
  git add docs/deployment.md && git commit -m "docs: add deployment guide"
  ```

---

### Task 8: Hero walkthrough page (+ images dir)

**Files:**
- Create: `docs/walkthrough.md`
- Create: `docs/images/README.md` (notes the dir holds doc screenshots)

**Interfaces:**
- Consumes: links to `connectors-and-secrets.md`, `packs.md`, `scenarios.md`, `getting-started.md`, `concepts.md` (all created in earlier tasks).
- Produces: `docs/walkthrough.md`, `docs/images/`.

- [ ] **Step 1: Create the images dir marker.** Write `docs/images/README.md` with one line: "Screenshots referenced by the documentation pages."

- [ ] **Step 2: Write the walkthrough** following the real UI nav, with image placeholders. Sections:
  1. Title + one-liner ("From a fresh SimRun to your first matched detection.").
  2. **Before you start** — link `getting-started.md` (SimRun running) and `concepts.md` (vocabulary). Goal statement: detonate a real pack simulation in AWS and confirm an Elastic alert fires.
  3. **Step 1 — Open the dashboard.** `![Dashboard](images/dashboard.png)`
  4. **Step 2 — Add connectors + secrets.** Add an Elastic connector (your SIEM) and an AWS connector backed by a secret group. Link `connectors-and-secrets.md`. `![Connectors](images/connectors.png)`
  5. **Step 3 — Install a pack.** Go to Packs, install a simulation pack. Link `packs.md`. `![Packs](images/packs.png)`
  6. **Step 4 — Create an assessment.** Go to Assessments → new; choose a pack simulation (`simrunDetonator`), set `targets.aws` to your AWS connector, add an `elasticSecurityAlert` expectation with the rule `name`. Link `scenarios.md` for the underlying YAML. `![New assessment](images/assessment-new.png)`
  7. **Step 5 — Run it.** Running the assessment creates a Run; scenarios execute in parallel. `![Run in progress](images/run.png)`
  8. **Step 6 — Read the results.** On `/runs/[id]`, review matched vs unmatched expectations and any collected logs. `![Run results](images/run-results.png)`
  9. **Next steps** — Rule Coverage (`/rules/coverage`, MITRE), scheduling, and writing your own scenarios (`scenarios.md`).

- [ ] **Step 3: Verify routes referenced exist:** `ls web/frontend/src/routes/assessments web/frontend/src/routes/runs web/frontend/src/routes/packs web/frontend/src/routes/connectors web/frontend/src/routes/secrets web/frontend/src/routes/rules/coverage`. Expected: all exist.

- [ ] **Step 4: Run link check** on `docs/walkthrough.md`. Expected: no `MISS` (image `.png` links are not checked by the command; that is intended).

- [ ] **Step 5: Commit.**
  ```bash
  git add docs/walkthrough.md docs/images/README.md && git commit -m "docs: add hero walkthrough"
  ```

---

### Task 9: Docs index

**Files:**
- Create: `docs/README.md`

**Interfaces:**
- Consumes: links to all eight pages from Tasks 1–8.
- Produces: `docs/README.md` — the table of contents, linked from the root README.

- [ ] **Step 1: Write the index.** A title, one intro line, a suggested reading order (Getting Started → Concepts → Walkthrough → references), then a bullet list linking each page with a one-line description: `getting-started.md`, `concepts.md`, `walkthrough.md`, `scenarios.md`, `connectors-and-secrets.md`, `packs.md`, `configuration.md`, `deployment.md`.

- [ ] **Step 2: Run link check** on `docs/README.md`. Expected: no `MISS` and all eight pages listed.

- [ ] **Step 3: Verify every docs page is linked:** `for p in getting-started concepts walkthrough scenarios connectors-and-secrets packs configuration deployment; do grep -q "$p.md" docs/README.md && echo "OK $p" || echo "MISS $p"; done`. Expected: all OK.

- [ ] **Step 4: Commit.**
  ```bash
  git add docs/README.md && git commit -m "docs: add docs index"
  ```

---

### Task 10: README revamp + badges

**Files:**
- Modify: `README.md` (full rewrite of the front-door structure)

**Interfaces:**
- Consumes: links into `docs/*.md` (all created in Tasks 1–9).

- [ ] **Step 1: Verify badge backing exists before adding each badge.** Run `ls .github/workflows` (already known: `docker-publish.yml`, `release-please.yml`, `pr-size.yml`, `pr-title.yml`). Include a badge ONLY if its backing exists:
  - License (Apache-2.0) — always safe (static shields.io).
  - Latest release — `release-please.yml` exists → use the GitHub release shields badge for `IBM/simrun`.
  - Docker — `docker-publish.yml` exists → link a "build/publish" workflow-status badge for that workflow.
  - Go version — static shields.io badge `Go-1.25` (from `go.mod`; verify with `grep '^go ' go.mod`).
  - Omit any badge you cannot back. Note omissions in the PR description.

- [ ] **Step 2: Rewrite README.md** with this structure:
  1. **Header** — `# SimRun` (logo placeholder comment: `<!-- logo: snake glyph + wordmark, added in a later pass -->`), one-line tagline ("An Attack Simulation Platform for detection testing — detonate attacks, verify the alerts fire.").
  2. **Badge row** — the verified badges from Step 1.
  3. **What is SimRun** — the existing two short intro paragraphs (single Go binary, REST+WS, PostgreSQL, embedded SvelteKit). Keep concise.
  4. **Screenshot slot** — `![SimRun dashboard](docs/images/dashboard.png)` placeholder.
  5. **Quickstart (60 seconds)** — minimal build + run block (mirror getting-started), then "Full guide → [docs/getting-started.md](docs/getting-started.md)".
  6. **How it works** — the vocabulary anchor sentence + "Read more → [docs/concepts.md](docs/concepts.md)".
  7. **Documentation** — bullet links into the `/docs` pages (mirror the index) or a single link to `docs/README.md`.
  8. **Development** — keep the existing dev commands block (`mise run build-frontend`, `go test ./...`, `mise run lint`, `go generate ./...`, `mise run parser`).
  9. **Contributing** — keep short.
  10. **License** — Apache-2.0, link `LICENSE`.
  - **Move, don't duplicate:** the deep Configuration table and the Concepts prose now live in `docs/`; remove them from README (replace with links). 

- [ ] **Step 3: Run link check** on `README.md` (adjust the command's `dirname` handles `docs/` prefixes correctly since links are repo-relative):
  ```bash
  f=README.md; grep -oE '\]\((docs/[^)]+\.md)(#[^)]*)?\)' "$f" | sed -E 's/\]\(([^)#]+).*/\1/' | while read -r l; do [ -f "$l" ] && echo "OK $l" || echo "MISS $l"; done
  ```
  Expected: no `MISS`.

- [ ] **Step 4: Verify no orphaned deep content remains:** `grep -nE 'SR_DATABASE_URL|kibana.alert' README.md` should now appear only inside the short quickstart (the full env table must be gone). Confirm the long env table is removed.

- [ ] **Step 5: Commit.**
  ```bash
  git add README.md && git commit -m "docs: revamp README as project front door with badges"
  ```

---

## Final verification (after all tasks)

- [ ] **Repo-wide link check:** for every `docs/*.md` and `README.md`, run the link check; expected: zero `MISS` across all files.
- [ ] **Tree check:** `ls docs/` shows `README.md getting-started.md concepts.md walkthrough.md scenarios.md connectors-and-secrets.md packs.md configuration.md deployment.md images/`.
- [ ] **Render check (manual):** open `README.md` and `docs/walkthrough.md` in a Markdown preview; confirm headings, tables, and code blocks render and image placeholders are clearly marked.

## Self-review notes (author)

- Spec coverage: every spec file (getting-started, concepts, walkthrough, scenarios, connectors-and-secrets, packs, configuration, deployment, docs index, README) maps to a task (Tasks 1–10). ✓
- Out-of-scope items (logo, `CLAUDE.md` stale paths) are intentionally untouched and flagged. ✓
- No placeholders in the plan; verified facts (schema fields, env table, pack params) are embedded inline. ✓
