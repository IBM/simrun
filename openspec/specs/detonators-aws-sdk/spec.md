# AWS SDK Detonator Specification

## Purpose
Type-specific behavior of `AWSDetonator`, a programmatic-SDK detonator
where the caller supplies a `DetonationFunc` closure operating on an
`aws.Config`. The detonator's job is to generate a UUID, build a
modified AWS config that injects that UUID into the User-Agent header,
and invoke the closure. Used for in-process AWS attack simulations.

## Requirements

### Requirement: ExecutionID Is UUIDv4
The system SHALL generate the execution ID as a UUIDv4 per detonation.

#### Scenario: Format
- **WHEN** `AWSDetonator.Detonate` returns
- **THEN** `execution_id` parses as a v4 UUID

### Requirement: User-Agent Replacement
The system SHALL replace the AWS SDK config's User-Agent entirely with
`"simrun_<uuid>"` (not appended). All SDK calls in the closure SHALL
emit this User-Agent so CloudTrail logs the token.

#### Scenario: Custom User-Agent in CloudTrail
- **WHEN** the closure issues an STS or S3 call
- **THEN** the corresponding CloudTrail entry's `userAgent` is `simrun_<uuid>`

### Requirement: Closure Receives Config and UUID
The system SHALL call `m.DetonationFunc(awsConfig, detonationUuid)`,
passing the modified `aws.Config` and the generated UUID. The closure
is responsible for the actual AWS operations.

#### Scenario: Closure invocation
- **WHEN** detonation runs
- **THEN** the configured closure is invoked with the User-Agent-modified config and the UUID

### Requirement: Closure Errors Propagate
The system SHALL surface any error returned by `DetonationFunc` directly
to the runner without wrapping. The runner's normal failure handling
applies (scenario result marked failed, matching skipped).

#### Scenario: Closure error
- **WHEN** `DetonationFunc` returns `errors.New("api denied")`
- **THEN** `Detonate` returns the same error

### Requirement: Static SimulationId
The system SHALL return the literal string `"AWSSDKSimulation"` from
`SimulationId()` regardless of the actual operation performed.

#### Scenario: Static identifier
- **WHEN** any AWSDetonator runs
- **THEN** `SimulationId()` returns `"AWSSDKSimulation"`

### Requirement: No Bash, No Terraform
The system SHALL NOT spawn subprocesses (no bash) and SHALL NOT manage
any Terraform state. All effects are produced by SDK calls inside the
closure.

#### Scenario: In-process
- **WHEN** an AWSDetonator runs
- **THEN** no child processes are spawned for the detonation itself
