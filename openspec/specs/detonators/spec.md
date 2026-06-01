# Detonators Specification

## Purpose
Executes the attack-simulation portion of a scenario. Detonators perform
real or simulated attack actions and return an `execution_id` that the
matchers and collectors use to correlate the resulting alerts and logs.
Three concrete detonator types exist: `SimrunDetonator` (terraform-driven
attack packs), `AWSCLIDetonator` (bash scripts using the AWS CLI), and
`AWSDetonator` (programmatic AWS SDK). This spec covers the umbrella
contract; implementation-specific behavior is in sibling specs.

## Requirements

### Requirement: Detonator Interface
The system SHALL define detonators as objects implementing
`Detonate(ctx) -> (map[string]string, error)` plus configuration setters
`SetRunID`, `SetEnvVars`, and `SetStatusCallback`. Every successful
`Detonate` MUST return a map containing the key `"execution_id"`.

#### Scenario: Successful detonation
- **WHEN** any detonator successfully completes
- **THEN** the returned map has `execution_id` set to a non-empty correlation token

### Requirement: Execution ID Format Varies By Detonator
The system SHALL allow each detonator to choose its execution-ID format:
`SimrunDetonator` SHALL emit a nanoid; `AWSCLIDetonator` and
`AWSDetonator` SHALL emit UUIDv4. **Note:** matchers and collectors must
therefore not assume a UUID format for correlation.

#### Scenario: Simrun pack uses nanoid
- **WHEN** `SimrunDetonator.Detonate` returns
- **THEN** `execution_id` is a URL-safe nanoid, not a UUID

### Requirement: Status Callback Reports Phase
The system SHALL invoke the `statusCallback` (when set) with
`"detonating"` at the start of execution. Detonators that include a
warmup phase (e.g., terraform setup) SHALL also emit `"warmup"` before
`"detonating"`.

#### Scenario: Terraform pack signals warmup
- **WHEN** a `SimrunDetonator` runs against a pack with terraform configured
- **THEN** the callback receives `"warmup"` before `"detonating"`

#### Scenario: No-op when callback is nil
- **WHEN** a detonator runs with a nil status callback
- **THEN** detonation proceeds without panicking and without status emissions

### Requirement: Run ID Threaded Into Logging
The system SHALL accept a `SetRunID(runID)` call before `Detonate`. The
run ID is used by the run-log routing layer to direct entries into the
correct per-run JSONL file.

#### Scenario: Run-scoped logs
- **WHEN** a detonator with `RunID = R` logs during execution
- **THEN** the entry is appended to `<DataDir>/run-logs/<R>.jsonl`

### Requirement: Env Vars Threading Differs By Detonator
The system SHALL pass `SetEnvVars(env)` through to `SimrunDetonator`,
`AWSCLIDetonator`, and `AWSDetonator` to use as cloud credentials and
configuration. **Note:** captured per-type in sibling specs; flagged
here because matchers/collectors share env semantics.

#### Scenario: Env passes to terraform
- **WHEN** a `SimrunDetonator` is given `AWS_ACCESS_KEY_ID` in env
- **THEN** terraform invocations have that env var set

### Requirement: Detonation Errors Surface as Result Failures
The system SHALL convert any error from `Detonate` into a failed
`scenario_results` row with the error string in `error_message`. The
runner SHALL skip the matching phase for a failed detonation.

#### Scenario: Detonation fails
- **WHEN** a detonator returns an error
- **THEN** the scenario result has `is_success = false`, `error_message` populated, and no assertion outcomes

### Requirement: Indicators Composed From Detonation Output
The system SHALL build the per-scenario indicators list at run time from
the detonation output map: `execution_id` is always added; if
`execution_uuid` is present in the map (e.g., for stratus packs), it is
added; values keyed by `indicators.terraformOutput[]` from the YAML are
resolved against the detonation output and added; `indicators.static`
values from the YAML are added verbatim.

#### Scenario: All indicator sources merged
- **WHEN** detonation returns `{execution_id:"x", bucket_arn:"y"}` and the YAML has `indicators.terraformOutput: ["bucket_arn"]` and `indicators.static: ["foo"]`
- **THEN** the indicators list passed to matchers is `["x", "y", "foo"]`

### Requirement: Cloud Provider Inferred Heuristically
The system SHALL infer the cloud provider for `SimrunDetonator` by
substring-matching the simulation ID against `"aws"`, `"gcp"`, `"azure"`
(case-insensitive). **Note:** this is informational metadata only — it
is not used in execution routing, and a simulation named `aws_backup`
would match `aws` even if not AWS-specific. Flagged as a heuristic, not
a guarantee.

#### Scenario: AWS pack inferred
- **WHEN** simulation ID is `aws.iam.escalation`
- **THEN** `CloudProvider()` returns `"aws"`

### Requirement: Constructor Validation
The system SHALL validate detonator construction inputs at
construction time only for static configuration (e.g., DataDir
existence, terraform binary version). Run-time inputs (target simulation
ID in pack manifest, AWS credentials availability) SHALL be validated
inside `Detonate` and reported as detonation errors.

#### Scenario: Bad simulation ID at runtime
- **WHEN** a `SimrunDetonator` is constructed successfully but the YAML references a simulation ID not in the pack manifest
- **THEN** `Detonate` returns an error listing the available simulation IDs in the pack
