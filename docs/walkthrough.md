# Walkthrough: Your First Detection Test

From a fresh SimRun to your first matched detection.

## Before you start

- SimRun is running and reachable. See [getting-started.md](getting-started.md) if you haven't set it up yet.
- You're familiar with the vocabulary — assessments, runs, scenarios, expectations, matchers. See [concepts.md](concepts.md) for a quick overview.

**Goal:** detonate a real pack simulation in AWS and confirm that the expected Elastic Security alert fires.

You'll need:
- A running Elastic Security deployment with Kibana access and an API key.
- An AWS account and credentials (access key or role ARN) that SimRun can use to run the simulation.

---

## Step 1 — Open the dashboard

Navigate to http://localhost:8080. The Dashboard gives an at-a-glance view of recent runs and scenario pass/fail rates.

![Dashboard](images/dashboard.png)

---

## Step 2 — Add connectors and secrets

SimRun needs to know where your SIEM is and how to reach AWS. Both are configured as connectors backed by secret groups.

### Add the Elastic connector

1. Go to **Connectors** (`/connectors`).
2. Click **Add connector**, choose type `elastic`.
3. Enter your `kibana_url` (e.g. `https://kibana.example.com`).
4. Under **Secret group**, create a new group and add `SR_ELASTIC_API_KEY` with your Elasticsearch API key.
5. Save. SimRun will use this connector as the default SIEM for all runs.

### Add the AWS connector

1. Still on **Connectors**, click **Add connector**, choose type `aws`.
2. If you're using role assumption, enter the `role_arn`.
3. Under **Secret group**, create a group and add `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and (if needed) `SR_AWS_EXTERNAL_ID`.
4. Note the connector name — you'll reference it in the assessment.

See [connectors-and-secrets.md](connectors-and-secrets.md) for the full field reference.

![Connectors](images/connectors.png)

---

## Step 3 — Install a pack

A pack bundles the Terraform modules and scenario definitions for a set of simulations.

1. Go to **Packs** (`/packs`).
2. Enter the pack name (e.g. `simrun-base-pack`) and its source URL.
3. Click **Install**. SimRun downloads the pack, validates its manifest, and lists the available simulations.
4. Optionally set pack-level parameters — for example, set `aws_region` to `us-east-1` so every simulation in the pack targets that region by default.

See [packs.md](packs.md) for parameter details.

![Packs](images/packs.png)

---

## Step 4 — Create an assessment

An assessment is a saved definition of scenarios. You'll create one with a single scenario that detonates a pack simulation and expects an Elastic Security alert.

1. Go to **Assessments** (`/assessments`) and click **New assessment**.
2. Give it a name and paste or write the scenario YAML. A minimal example:

```yaml
targets:
  aws: my-aws-connector    # the connector name from Step 2

scenarios:
  - name: S3 public access block disabled
    detonate:
      simrunDetonator:
        pack: simrun-base-pack
        simulation: s3-disable-public-access-block
    expectations:
      - elasticSecurityAlert:
          name: "S3 Public Access Block Disabled"
```

Key points:
- `targets.aws` must match the connector name you created in Step 2.
- `simrunDetonator.pack` and `simrunDetonator.simulation` reference the pack you installed in Step 3.
- `elasticSecurityAlert.name` is the exact Detection Engine rule name in Kibana that you expect to fire.

See [scenarios.md](scenarios.md) for the full YAML reference.

![New assessment](images/assessment-new.png)

---

## Step 5 — Run it

Click **Run** on the assessment. SimRun creates a new Run and begins executing scenarios in parallel.

- Each scenario detonates the simulation, then polls Kibana for the expected alert.
- The run page updates in real time as scenarios complete.

![Run in progress](images/run.png)

---

## Step 6 — Read the results

Open the run from the **Runs** page (`/runs`) or directly via the link shown after starting the run.

Each scenario shows one of:
- **Matched** — the expectation fired; the expected alert was found in Kibana within the timeout.
- **Unmatched** — the alert was not found before the timeout expired. Check whether the rule is enabled in Kibana and that the simulation actually ran (look at the collected logs if `collect` is configured).

If you added a `collect` block to your scenario, the related Elasticsearch logs are shown alongside the results for post-hoc analysis.

![Run results](images/run-results.png)

---

## Next steps

- **Rule Coverage** (`/rules/coverage`) — view your Elastic detection rules mapped to MITRE ATT&CK techniques. Identify which techniques have coverage and which are gaps.
- **Scheduling** — run assessments on a recurring schedule directly from the Assessments page.
- **Write your own scenarios** — see [scenarios.md](scenarios.md) for the full YAML reference: multiple expectations, AWS CLI detonation, log injection, indicators, and collectors.
