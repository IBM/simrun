# Pack Execution Specification

## Purpose
Defines what happens at scenario-run time when a `simrunDetonator` invokes
an installed pack: how the pack binary is resolved, how parameters and
credentials are passed, the terraform lifecycle, stderr/log routing, and
cleanup. This spec is the runtime counterpart to `packs` (which covers
install). The contract here is what scenario authors and operators
observe during a run, not the install path.

## Requirements

### Requirement: All Pack Types Resolve to a Local Binary
The system SHALL resolve every pack type (`local`, `upload`, `remote`,
`go-remote`) to a local binary path before execution. Pack processes
SHALL run on the same host as simrun; there is no remote pack execution
protocol.

#### Scenario: Resolution paths
- **WHEN** a scenario runs against a pack
- **THEN** the resolved binary is located under `<DataDir>/packs/...` (for `remote`, `go-remote`, `upload`) or at the configured `source` path (for `local`)

### Requirement: Pack Protocol over stdin/stdout
The system SHALL communicate with packs via three stdin-supplied
commands: `manifest`, `detonate`, `cleanup`. The pack reads JSON input
from stdin and writes a JSON response on stdout. Stderr is captured
separately for logging.

#### Scenario: Detonate invocation
- **WHEN** a scenario detonates against a pack
- **THEN** the pack receives a JSON `DetonateInput` on stdin and emits a `DetonateResponse` on stdout

### Requirement: Pack Exit Codes Have Semantic Meaning
The system SHALL interpret pack process exit codes as follows: `0` =
success (with valid JSON on stdout); `1` = simulation error (stdout
still contains a JSON response); `2` = protocol error (invalid input).
Other exit codes SHALL be reported as generic failures.

#### Scenario: Simulation error
- **WHEN** a pack exits with code 1 and emits a structured error JSON
- **THEN** the runner surfaces the error with the JSON message

### Requirement: Manifest Lookup Per Detonation
The system SHALL call the pack's `manifest` command at the start of
each detonation to resolve the target simulation. If the requested
`simulation.ID` is not in the manifest, the runner SHALL fail with an
error listing all available IDs.

#### Scenario: Available IDs surfaced
- **WHEN** a scenario references `simulation: "missing"` and the pack manifest contains `["a","b","c"]`
- **THEN** the detonation fails with an error mentioning `a`, `b`, `c`

### Requirement: Parameter Injection Two Ways
The system SHALL pass pack parameters (from DB `packs.parameters`) into
both the `ManifestInput.Parameters` field on the manifest call and as
`TF_VAR_<key>=<value>` environment variables for terraform invocations.
Per-scenario `params:` from the YAML SHALL be merged into
`DetonateInput.Params` and ALSO promoted to `TF_VAR_*` env vars. When
both scopes set the same key, the per-scenario value SHALL take
precedence over the pack-level value; both layers SHALL be applied on
top of any `default = ...` declared on the matching Terraform
`variable` block. Pack-level values SHALL be passed for every key in
`packs.parameters`, regardless of whether the value was declared in
the pack's `params_schema` (so previously-stored unknown keys continue
to flow until cleaned up).

#### Scenario: Parameter as TF_VAR
- **WHEN** the pack DB record has `parameters: {"region":"us-east-1"}`
- **THEN** terraform inside the detonation runs with `TF_VAR_region=us-east-1`

#### Scenario: Per-scenario override of pack-level value
- **WHEN** the pack record has `parameters: {"aws_region": "us-east-1"}`
  and a scenario YAML sets `params: { aws_region: "us-west-2" }` for one
  simulation
- **THEN** that simulation's `terraform apply` runs with
  `TF_VAR_aws_region=us-west-2`, while sims without a scenario-level
  override still see `us-east-1`

#### Scenario: TF variable default used when neither scope provides value
- **WHEN** a sim's TF declares `variable "resource_prefix" { default = "simrun" }`
  and neither the pack record nor the scenario sets `resource_prefix`
- **THEN** terraform uses the declared default of `"simrun"`

#### Scenario: Pack-level unknown keys still flow
- **WHEN** the pack's stored parameters include a key not present in
  the pack's `params_schema` (e.g., a legacy key)
- **THEN** the key is still exported as a `TF_VAR_*` env var to the
  detonation's terraform process

### Requirement: Conditional Terraform Lifecycle
The system SHALL perform Terraform setup, apply, destroy only when the
pack manifest's selected simulation contains a non-empty
`terraform` field (base64-encoded HCL content). Simulations without
terraform SHALL skip directly to detonation with no terraform side
effects.

#### Scenario: Terraform-less pack
- **WHEN** a simulation has `terraform: ""`
- **THEN** no `<DataDir>/terraform/<id>/` directory is created and the runner emits no `"warmup"` status

#### Scenario: Terraform-driven pack
- **WHEN** a simulation has non-empty terraform content
- **THEN** the runner creates `<DataDir>/terraform/<executionID>/`, runs `init` then `apply`, executes detonation, then runs `destroy` and removes the directory

### Requirement: Terraform Outputs Available to Detonation
The system SHALL parse `terraform.tfstate` after `apply` and provide
the outputs as `DetonateInput.TerraformOutputs` on the detonate stdin
payload, so packs can use deployed resource identifiers.

#### Scenario: Output passed to detonate
- **WHEN** terraform `apply` produces output `bucket_arn=arn:aws:s3:::x`
- **THEN** the pack receives `TerraformOutputs: {"bucket_arn":"arn:aws:s3:::x"}` on stdin during detonate

### Requirement: Terraform Destroy Is Best-Effort
The system SHALL always call `terraform destroy` after a terraform-using
detonation completes (including failure paths). If `destroy` fails, the
working directory SHALL be preserved (not deleted) so an operator can
manually run `terraform destroy`. The detonation SHALL still surface
the destroy error.

#### Scenario: Successful destroy
- **WHEN** detonation succeeds and destroy succeeds
- **THEN** the working directory `<DataDir>/terraform/<id>/` is removed

#### Scenario: Destroy failure preserves state
- **WHEN** `terraform destroy` returns an error after detonation
- **THEN** the working directory remains on disk and an error is logged identifying the path for manual cleanup

### Requirement: Custom Cleanup Best-Effort
When a manifest entry has `has_custom_cleanup: true`, the system SHALL
invoke the pack's `cleanup` command after detonation. A failure of
custom cleanup SHALL log a warning but SHALL NOT fail the detonation
result.

#### Scenario: Cleanup failure tolerated
- **WHEN** `cleanup` returns a non-zero exit but detonation succeeded
- **THEN** the scenario result has `is_success = true` and a warning is logged

### Requirement: User-Agent Tagging for Cloud Audit Logs
The system SHALL set `TF_APPEND_USER_AGENT=simrun/<executionID>` for
every Terraform execution, so cloud-provider audit logs can be
correlated to the run by user-agent.

#### Scenario: UA in CloudTrail
- **WHEN** a scenario detonates against AWS via terraform
- **THEN** the corresponding CloudTrail entries' `userAgent` field contains `simrun/<executionID>`

### Requirement: Pack Stderr Logging Toggle
The system SHALL gate pack stderr emission to logrus on
`AppConfig.pack_logs_enabled`. When the flag is false, pack stderr
SHALL be discarded silently. **Note:** flagged — debugging pack
failures requires re-enabling this flag.

#### Scenario: Logging disabled
- **WHEN** `pack_logs_enabled = false` and a pack writes to stderr
- **THEN** no log entries are emitted for that stderr output

### Requirement: SSH Logging Directory Provisioning
The system SHALL set `SR_SSH_LOG_DIR=<DataDir>/ssh-logs/` in the run
environment when `AppConfig.ssh_logging_enabled` is true. simrun SHALL
NOT create the directory itself; pack SDKs are expected to create it
when writing.

#### Scenario: Env var injection
- **WHEN** `ssh_logging_enabled = true` and a scenario uses an SSH-based pack
- **THEN** the pack process sees `SR_SSH_LOG_DIR=<DataDir>/ssh-logs/`

### Requirement: Terraform Binary Cached Process-Wide
The system SHALL download and cache the configured Terraform version on
first use via `sync.Once`, reusing the cached binary path for the life
of the process. **Note:** changing `AppConfig.terraform_version` while
the server runs has no effect until restart; flagged as a known
behavior.

#### Scenario: Version change requires restart
- **WHEN** the operator updates `terraform_version` in AppConfig at runtime
- **THEN** in-flight and subsequent runs continue to use the previously cached binary until the server restarts

### Requirement: Detonation Working Directory Per ExecutionID
The system SHALL place each terraform-using detonation under a unique
working directory `<DataDir>/terraform/<executionID>/`, where
`executionID` is the nanoid generated for that detonation. Concurrent
detonations SHALL NOT share working directories.

#### Scenario: Concurrent isolation
- **WHEN** two simrunDetonator scenarios run in parallel
- **THEN** each executes in its own `<DataDir>/terraform/<id>/` directory with no shared state

### Requirement: Remote Resolve Not Concurrency-Safe
The system SHALL note as a known gap that the remote-pack download path
checks "is cached" then downloads if absent without synchronization. If
two scenarios trigger a download for the same uncached remote pack
simultaneously, both SHALL attempt the download. **Note:** flagged as
an unaddressed race; current behavior is to allow last-writer-wins.

#### Scenario: Concurrent first-time fetch
- **WHEN** two runs reference the same uncached remote pack at the same time
- **THEN** both initiate downloads, the cache file may be written twice, and both runs proceed once their write completes
