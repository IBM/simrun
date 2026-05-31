# Simrun Detonator Specification

## Purpose
Type-specific behavior of `SimrunDetonator`, the Terraform-driven attack
detonator that invokes installed packs. The runtime concerns (pack
binary resolution, terraform lifecycle, parameter injection, cleanup)
are detailed in `pack-execution`; this spec captures detonator-specific
contract: execution-id format, status callbacks, manifest lookup, and
error semantics.

## Requirements

### Requirement: ExecutionID Is a Nanoid
The system SHALL generate the execution ID as a `go-nanoid` URL-safe
random string. **Note:** this differs from `AWSCLIDetonator` and
`AWSDetonator`, which use UUIDv4. Matchers and collectors must not
assume execution IDs from this detonator are UUIDs.

#### Scenario: Format
- **WHEN** `SimrunDetonator.Detonate` returns
- **THEN** `execution_id` is a nanoid (not a v4 UUID)

### Requirement: Manifest Lookup at Detonate Time
The system SHALL invoke the pack's `manifest` command on every detonation
to resolve the target simulation. If the requested simulation ID is
not present in the manifest, `Detonate` SHALL return an error listing
all available IDs.

#### Scenario: Unknown simulation ID
- **WHEN** the YAML references `simulation: "x"` and the pack manifest does not contain `x`
- **THEN** `Detonate` returns an error including the available simulation IDs

### Requirement: Status Callback Phases
The system SHALL emit `"warmup"` via the status callback before
Terraform `apply` when the resolved simulation has non-empty
`terraform`. It SHALL emit `"detonating"` immediately before invoking
the pack's `detonate` command. Simulations without terraform SHALL
emit `"detonating"` only.

#### Scenario: TF pack phases
- **WHEN** a simulation with terraform runs
- **THEN** the callback receives `"warmup"` then `"detonating"`

#### Scenario: Non-TF pack
- **WHEN** a simulation without terraform runs
- **THEN** the callback receives `"detonating"` only

### Requirement: Terraform Lifecycle Always Cleaned Up
The system SHALL run `terraform destroy` after detonation in all paths
when terraform was set up, including when detonation fails or returns
an error. If `apply` fails, `destroy` SHALL be invoked immediately
before returning the error. If `destroy` succeeds, the working
directory SHALL be removed; if `destroy` fails, the directory SHALL be
preserved for manual cleanup.

#### Scenario: Apply failure cleans up
- **WHEN** `terraform apply` fails
- **THEN** `terraform destroy` runs and the working directory is removed

#### Scenario: Destroy failure preserved
- **WHEN** `destroy` returns an error
- **THEN** the working directory remains on disk and an error is logged identifying the path

### Requirement: Custom Cleanup Best-Effort
The system SHALL invoke the pack's `cleanup` command after detonation
when the resolved simulation manifest entry has `has_custom_cleanup: true`.
A failure SHALL log a warning but SHALL NOT fail the detonation result.

#### Scenario: Custom cleanup logs warning
- **WHEN** `cleanup` returns non-zero after a successful detonation
- **THEN** the detonation result is success and a warning is logged

### Requirement: CloudProvider Heuristic
The system SHALL infer `CloudProvider()` by case-insensitive substring
match on the simulation ID against `"aws"`, `"gcp"`, `"azure"`. **Note:**
this is informational metadata only and may produce surprising results
for simulation IDs that happen to contain these substrings (e.g.,
`backup_to_aws`).

#### Scenario: AWS pack inferred
- **WHEN** simulation ID is `aws.iam.escalation`
- **THEN** `CloudProvider()` returns `"aws"`

#### Scenario: No cloud inferred
- **WHEN** simulation ID is `linux.persistence.cron`
- **THEN** `CloudProvider()` returns `""`

### Requirement: Constructor Validates Static Inputs Only
The system SHALL validate at construction time only that the DataDir
and Terraform-version inputs allow creating directories and locating a
binary. Runtime validation (manifest lookup, cloud auth) SHALL happen
inside `Detonate`.

#### Scenario: Bad simulation deferred
- **WHEN** a `SimrunDetonator` is constructed with a simulation ID that does not exist in the pack
- **THEN** construction succeeds; `Detonate` is the call that fails
