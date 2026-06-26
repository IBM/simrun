# Scenarios YAML reference

The YAML shape behind every assessment.

---

## Top-level structure

A scenario file has three top-level keys. Only `scenarios` is required.

| Field | Type | Required | Description |
|---|---|---|---|
| `metadata` | object | No | Human-readable name and description for the file. |
| `metadata.name` | string | No | Name of the scenarios set. |
| `metadata.description` | string | No | Description of the scenarios set. |
| `targets` | object | No | Connector names for each cloud/infrastructure target used by all scenarios in this file. |
| `targets.aws` | string | No | Name of the AWS connector to use for cloud credentials. |
| `targets.gcp` | string | No | Name of the GCP connector to use for cloud credentials. |
| `targets.azure` | string | No | Name of the Azure connector to use for cloud credentials. |
| `targets.kubernetes` | string | No | Name of the Kubernetes connector to use for cluster access. |
| `targets.ssh` | string | No | Name of the SSH connector to use for remote command detonation. |
| `scenarios` | array | **Yes** | List of scenario objects (see below). |

---

## A scenario

Each item in `scenarios` describes one unit of work: how to trigger activity and what alerts to expect.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | **Yes** | Display name for this scenario. |
| `expectations` | array | **Yes** | One or more alert expectations (see [Expectations](#expectations)). |
| `enabled` | boolean | No | Whether to run this scenario. Defaults to `true`. Set to `false` to skip without deleting. |
| `indicators` | object | No | Values extracted after detonation and used for log correlation. |
| `indicators.terraformOutput` | string[] | No | Terraform output keys to extract (e.g. `attacker_vm_public_ip`). |
| `indicators.static` | string[] | No | Fixed strings that will appear in generated activity (e.g. a known username). |
| `detonate` | object | No | How to execute the attack. Mutually exclusive with `inject`. |
| `inject` | object | No | How to push a document directly into the SIEM instead of running an attack. Mutually exclusive with `detonate`. |
| `collect` | object | No | How to retrieve related logs after detonation for post-hoc analysis. |

---

## Detonators

A `detonate` block must contain exactly one of `awsCliDetonator` or `simrunDetonator`.

### awsCliDetonator

Runs one or more AWS CLI commands. No pack or Terraform required.

| Field | Type | Required | Description |
|---|---|---|---|
| `script` | string | No | Shell script to execute using the AWS CLI. |

```yaml
scenarios:
  - name: aws cli detonator
    detonate:
      awsCliDetonator:
        script: "aws sts get-caller-identity"
    expectations:
      - timeout: 1m
        datadogSecuritySignal:
          name: "AWS API call"
```

### simrunDetonator

Runs a pack simulation using Terraform. The pack must be installed before the run.

| Field | Type | Required | Description |
|---|---|---|---|
| `pack` | string | **Yes** | Name of the installed pack containing the simulation. |
| `simulation` | string | **Yes** | Simulation ID within the pack (e.g. `aws.exfil.s3`). |
| `params` | object | No | Key-value parameters passed to the simulation. Merged with pack-level defaults; per-scenario values take precedence. Map and array values are JSON-encoded. |

```yaml
scenarios:
  - name: s3 exfiltration
    detonate:
      simrunDetonator:
        pack: attack-pack
        simulation: aws.exfil.s3
        params:
          aws_region: us-east-1
          bucket_name: my-test-bucket
    expectations:
      - timeout: 5m
        elasticSecurityAlert:
          name: "S3 Exfiltration Detected"
          severity: high
```

---

## Injector

An `inject` block must contain `elasticInjector`. Use injection to verify a detection rule is wired up without running a real attack: SimRun writes the document and then polls for the expected alert.

### elasticInjector

| Field | Type | Required | Description |
|---|---|---|---|
| `documents` | array | **Yes** | One or more documents to inject into Elasticsearch. |
| `documents[].index` | string | **Yes** | Elasticsearch index to write into. |
| `documents[].file` | string | See note | Path to a JSON document file. Supports `{{variable_name}}` placeholder substitution. Required unless `template` is used. |
| `documents[].template` | string | See note | Pack template ID (e.g. `okta.add-group-member`). Required unless `file` is used. |
| `documents[].pack` | string | See note | Pack providing the template. Required when `template` is set. |
| `documents[].vars` | object | No | String-to-string map of variables to substitute in the document using `{{variable_name}}` syntax. |

Each document must supply either `file` or both `template` + `pack`.

```yaml
scenarios:
  - name: elastic injector
    inject:
      elasticInjector:
        documents:
          - index: "logs-test"
            file: "doc.json"
    expectations:
      - timeout: 1m
        elasticSecurityAlert:
          name: "Test alert"
```

---

## Collector

A `collect` block must contain `elasticCollector`. Collectors run after detonation and store related logs with the run result.

### elasticCollector

| Field | Type | Required | Description |
|---|---|---|---|
| `index` | string | **Yes** | Elasticsearch index to search for logs. |
| `additionalFields` | object | No | Extra fields to filter by. Values can be static strings or template expressions referencing Terraform outputs, e.g. `{{ indicators.terraformOutput.attacker_vm_public_ip }}`. |

```yaml
scenarios:
  - name: with collector
    detonate:
      awsCliDetonator:
        script: "aws sts get-caller-identity"
    collect:
      elasticCollector:
        index: "logs-test"
        additionalFields:
          source.ip: "{{ indicators.terraformOutput.attacker_vm_public_ip }}"
    expectations:
      - timeout: 1m
        datadogSecuritySignal:
          name: "Test signal"
```

---

## Expectations

Every scenario must declare at least one expectation. Each expectation specifies a `timeout` and exactly one matcher.

| Field | Type | Required | Description |
|---|---|---|---|
| `timeout` | string | No | Maximum time to wait for the alert. Written as a Go duration (e.g. `5m`, `30s`, `2m30s`). Defaults to `5m`. |
| `elasticSecurityAlert` | object | See note | Match an Elastic Security Detection alert. Required unless `datadogSecuritySignal` is used. |
| `datadogSecuritySignal` | object | See note | Match a Datadog security signal. Required unless `elasticSecurityAlert` is used. |

### elasticSecurityAlert

Polls Kibana until a Detection Engine alert appears whose `kibana.alert.rule.name.keyword` matches `name`.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | **Yes** | Exact rule name to match (matched against `kibana.alert.rule.name.keyword`). |
| `severity` | string | No | Alert severity to match. One of: `low`, `medium`, `high`, `critical`. |

### datadogSecuritySignal

Polls Datadog Security Signals until a signal with a matching name appears.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | **Yes** | Exact signal name to match. |
| `severity` | string | No | Signal severity to match. |

---

## Full example

The following file exercises every top-level field — `targets`, `metadata` is omitted for brevity since it carries no executable semantics. The content matches `internal/parser/testdata/scenarios/targets-all.yaml` exactly.

```yaml
targets:
  aws: prod-aws
  gcp: prod-gcp
  azure: prod-azure
  kubernetes: prod-k8s
  ssh: prod-ssh
scenarios:
  - name: targeted all
    detonate:
      awsCliDetonator:
        script: "true"
    expectations:
      - timeout: 1m
        datadogSecuritySignal:
          name: "Test signal"
```

**Walkthrough:**

- `targets` maps each cloud/infra connector type to a named connector. These override any defaults set in app configuration for this file's scenarios.
- The single scenario uses `awsCliDetonator` to call `true` (a no-op shell command) — sufficient to trigger whatever AWS CloudTrail or Datadog rule is being tested.
- The expectation waits up to 1 minute (`timeout: 1m`) for a Datadog security signal named `"Test signal"`.

---

## See also

- [concepts.md](concepts.md) — vocabulary, detonators, matchers, and collectors explained
- [connectors-and-secrets.md](connectors-and-secrets.md) — set up the connectors referenced in `targets`
