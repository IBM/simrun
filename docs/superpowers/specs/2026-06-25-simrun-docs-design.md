# SimRun Documentation — Design

**Date:** 2026-06-25
**Status:** Approved (design); implementation pending
**Scope:** End-user documentation and README revamp. Logo is explicitly out of scope for this spec (handled in a later pass).

## Problem

SimRun has a single README that mixes "what it is," configuration reference, and concept
prose. There is no hands-on walkthrough, so a new user cannot get from clone to a first
green detection result on their own. The README also lacks the visual/structural polish
expected of a mature OSS project (badges, a front-door layout that links into deeper docs).

## Goals

- A `/docs` tree of cross-linked Markdown that a detection engineer can follow to:
  - understand the SimRun mental model,
  - install and run SimRun,
  - get a first real detection result end-to-end (the "hero" walkthrough),
  - look up scenario YAML, connectors/secrets, packs, config, and deployment when needed.
- A README that works as a polished front door: tagline, badges, a 60-second quickstart,
  a single screenshot/diagram slot, and links into `/docs` instead of inlining everything.

## Non-goals

- Logo / brand artwork (deferred to a later pass; README uses a placeholder).
- A built docs site (Docusaurus/MkDocs). Decision: plain Markdown in `/docs`, rendered
  natively by GitHub. Lowest tooling and maintenance.
- API/developer-internals reference beyond what already lives in `CLAUDE.md`. These docs
  are user-facing.
- Capturing live screenshots (cannot run the UI here). Walkthrough leaves clearly-marked
  image placeholders for the maintainer to fill.

## Vocabulary anchor (source of truth)

The concepts page opens with this sentence verbatim, and all other pages use these terms
consistently:

> An Assessment defines scenarios. Running it creates a Run. The run executes each
> scenario (in parallel, like jobs), and each scenario checks its expectations via matchers.

Term map (verified against the frontend routes and sidebar):

- **Assessment** — a saved definition of one or more scenarios (`/assessments`).
- **Run** — one execution of an assessment (`/runs`, detail at `/runs/[id]`).
- **Scenario** — a single job within an assessment; scenarios run in parallel.
- **Expectation** — what a scenario asserts should fire (e.g. an Elastic alert).
- **Matcher** — platform integration that verifies an expectation (Elastic / Datadog).
- Supporting: **Detonator**, **Injector**, **Collector**, **Pack**, **Connector**,
  **Secret group**, correlation by execution UUID / indicators.

## File structure

```
README.md                    front door: what/why, badges, 60s quickstart, links into docs
docs/
  README.md                  docs index / table of contents
  getting-started.md         install (build / run / Docker), first launch, prerequisites
  concepts.md                mental model + vocabulary; the conceptual source of truth
  walkthrough.md             HERO: zero-to-first-detection via a real AWS pack detonation
  scenarios.md               scenario YAML reference (detonate/inject/expectations/etc.)
  connectors-and-secrets.md  per-platform connector + secret-group setup
  packs.md                   installing/managing packs; pack parameters & precedence
  configuration.md           full SR_* env var reference + DB-backed app config defaults
  deployment.md              Docker/production, Google OAuth, data dir + encryption key
```

The `docs/superpowers/` subtree (specs) is internal and is not linked from user docs.

## Page contents

### README.md (front door)
- Header: logo placeholder, project name, one-line tagline.
- Badge row: latest release, license (Apache-2.0), Go version, CI/build status, Docker.
  - Badge URLs target `github.com/IBM/simrun`. Any badge whose backing
    workflow/registry cannot be verified is omitted rather than guessed (fail loud).
- One short paragraph: what SimRun is and who it's for.
- One screenshot/diagram slot (placeholder).
- 60-second quickstart: minimal build + run + open browser. Links to
  `docs/getting-started.md` for detail.
- "How it works" — the vocabulary sentence + link to `docs/concepts.md`.
- Short link list into the rest of `/docs`.
- Development, Contributing, License sections retained (trimmed).
- Deep config/concept prose currently inlined in README is **moved** to `/docs`, not
  duplicated.

### docs/README.md
- Table of contents with a one-line description per page and a suggested reading order
  (Getting Started → Concepts → Walkthrough → references).

### docs/getting-started.md
- Prerequisites (mise / Go 1.25 / Node 22, PostgreSQL).
- Build (`mise build`), run with `SR_DATABASE_URL`, open `http://localhost:8080`.
- Auth-optional note. Pointer to `configuration.md` and `deployment.md`.

### docs/concepts.md
- Vocabulary sentence as the opener, then the term map.
- Detonators (Simrun / AWS CLI), Injectors (Elastic), Matchers (Elastic / Datadog),
  Collectors (Elastic), Packs, Connectors, Secret groups.
- Correlation: execution UUID vs static/dynamic indicators.
- This page absorbs the "Concepts" prose currently in the README.

### docs/walkthrough.md (hero)
Zero-to-first-detection, following the real UI, with image placeholders:
1. Launch SimRun → Dashboard.
2. Connectors + Secrets: add an Elastic connector (SIEM) and an AWS connector backed by
   a secret group.
3. Packs: install a simulation pack.
4. Assessments: create an assessment — choose a simulation, set targets/parameters,
   define an expectation (the matcher).
5. Run it → watch scenarios execute in parallel.
6. Read results on `/runs/[id]` — matched vs unmatched expectations, collected logs.
7. Next steps: Rule Coverage (MITRE), scheduling, writing your own scenarios.

### docs/scenarios.md
- YAML reference. Each field verified against `schemas/`:
  `simrun.schema.json`, `simrunDetonator.schema.json`, `awsCliDetonator.schema.json`,
  `elasticInjector.schema.json`, `elasticCollector.schema.json`,
  `elasticSecurityAlert.schema.json`, `datadogSecuritySignal.schema.json`.
- Worked examples drawn from `internal/parser/testdata/scenarios/`.
- `targets`, `detonate`/`inject`, `expectations` (timeout + matcher), `indicators`,
  `collect`.

### docs/connectors-and-secrets.md
- What a connector is, what a secret group is, how they link.
- Per-platform setup: Elastic, Datadog, AWS, GCP, Azure, Kubernetes, SSH (as supported).

### docs/packs.md
- Installing/managing packs via the UI; the shipped packs (base, stratus).
- Pack parameters: built-ins (`default_tags`, `aws_region`, `gcp_region`,
  `azure_location`), author-declared params, and precedence
  (TF default < pack-level < per-sim scenario). Verified against `pack/` + `CLAUDE.md`.

### docs/configuration.md
- Full `SR_*` env var table (moved/expanded from README).
- DB-backed app config defaults (parallelism, etc.) managed at `/config`.

### docs/deployment.md
- Docker build/run, volume for `SR_DATA_DIR` + encryption key persistence.
- Google OAuth setup (`SR_GOOGLE_*`), allowed domain, session TTL.

## Principles / constraints

- Task-oriented and cross-linked; every page links to its prerequisites and next steps.
- Accuracy over volume: page names, routes, YAML fields, env vars, and pack-parameter
  behavior are verified against the code while writing. No invented options.
- Screenshots: clearly-marked `![alt](images/<name>.png)` placeholders under
  `docs/images/` (directory created with a short README noting it holds doc screenshots).
- Fail loud: omit any badge or claim that can't be verified; flag anything uncertain in
  the PR description rather than papering over it.

## Out of scope / follow-ups (flag, don't fix here)

- Logo and favicon redesign (snake glyph + "SimRun" wordmark) — separate pass.
- `CLAUDE.md` references `simrun/internal/...` paths, but the actual tree is `internal/`,
  `pack/`, etc. at repo root. Note for a separate cleanup; not touched by this work.

## Success criteria

- A new user can follow `getting-started.md` then `walkthrough.md` and reach a first
  matched expectation without external help.
- README renders cleanly on GitHub with working badges and links; no broken internal
  links across the `/docs` tree.
- Every documented YAML field, env var, route, and pack parameter corresponds to
  something that actually exists in the code.
