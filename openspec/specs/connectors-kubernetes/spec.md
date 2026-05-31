# Kubernetes Connector Specification

## Purpose
Type-specific behavior of `kubernetes`-typed connectors. Kubernetes
connectors do not have their own credentials; they reference an existing
cloud connector (AWS / GCP / Azure) by name and delegate credential
resolution to it. At run time, the system invokes the appropriate cloud
provider's CLI to populate a kubeconfig and exposes its path as
`KUBECONFIG`.

## Requirements

### Requirement: Required Fields
The system SHALL require `cluster_name`, `region`, and `cloud_connector`
(name of an enabled AWS/GCP/Azure connector) in `config`. `resource_group`
SHALL be required only for AKS clusters; `project` SHALL be optional for
GKE clusters (defaulting to the GCP connector's `project_id`).

#### Scenario: Missing cluster_name
- **WHEN** a client creates a kubernetes connector with empty `cluster_name`
- **THEN** the response is HTTP 400

### Requirement: Cloud Connector Reference
The system SHALL look up the named cloud connector at run time, requiring
it to be enabled and to have a recognized cloud type (`aws`, `gcp`, or
`azure`). A missing, disabled, or wrong-type reference SHALL fail the
run.

#### Scenario: Unknown cloud connector
- **WHEN** `cloud_connector: "deleted-aws"` does not match any enabled connector
- **THEN** the run-time auth resolution fails

#### Scenario: Disabled cloud connector
- **WHEN** the referenced cloud connector exists but `enabled = false`
- **THEN** the run-time auth resolution fails

### Requirement: Cloud Type Auto-Detection
The system SHALL derive the kubernetes platform (EKS, GKE, AKS) from the
referenced cloud connector's type:
`aws` → EKS, `gcp` → GKE, `azure` → AKS.

#### Scenario: AKS via Azure
- **WHEN** `cloud_connector` references an Azure connector
- **THEN** the kubeconfig is generated via the AKS path

### Requirement: Kubeconfig Generation Per Cloud
The system SHALL invoke the platform-appropriate CLI at run time:
`aws eks update-kubeconfig` for EKS, `gcloud container clusters get-credentials`
for GKE, `az aks get-credentials` for AKS. The CLI SHALL run with the
cloud connector's resolved credentials in env.

#### Scenario: EKS kubeconfig
- **WHEN** a kubernetes connector referencing an AWS connector with role_arn is used at run time
- **THEN** `aws eks update-kubeconfig` runs with `AWS_ACCESS_KEY_ID/SECRET/TOKEN` populated by AssumeRole

### Requirement: KUBECONFIG Env Injection
The system SHALL set `KUBECONFIG` and `KUBE_CONFIG_PATH` in the run
environment to the path of the generated kubeconfig file.

#### Scenario: Env injected
- **WHEN** kubeconfig generation succeeds with the file at `/tmp/kubeconfig-x`
- **THEN** the run env contains `KUBECONFIG=/tmp/kubeconfig-x` and `KUBE_CONFIG_PATH=/tmp/kubeconfig-x`

### Requirement: Test Connection
The system SHALL implement `POST /api/connectors/test` for `type: "kubernetes"`
by performing the same kubeconfig generation flow, then validating the
kubeconfig with `k8s.ValidateKubeconfig`. Failures at either step SHALL
return `{success: false, error}`.

#### Scenario: Bad cluster name
- **WHEN** the cluster does not exist in the configured region
- **THEN** the test returns `{success: false, error}` with the CLI's error

### Requirement: is_default Allowed
The system SHALL allow `is_default = true` on kubernetes connectors,
subject to the umbrella one-default-per-cloud-type rule.

#### Scenario: Default kubernetes connector
- **WHEN** no other kubernetes connector is the default
- **THEN** setting `is_default: true` succeeds
