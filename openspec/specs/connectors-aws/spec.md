# AWS Connector Specification

## Purpose
Type-specific behavior of `aws`-typed connectors. Resolves AWS credentials
either via STS `AssumeRole` (when `role_arn` is configured) or by relying
on simrun's ambient credential chain (IRSA, instance profile, env). The
resolved credentials are injected into the run environment for use by
`SimrunDetonator` (Terraform), `AWSCLIDetonator`, and `AWSDetonator`.

## Requirements

### Requirement: Optional Role ARN
The system SHALL accept an optional `config.role_arn` string. When
present, AWS credentials SHALL be obtained by calling STS `AssumeRole`
against that ARN. When absent, simrun's ambient AWS credentials SHALL
be used.

#### Scenario: With role_arn
- **WHEN** an AWS connector has `role_arn: "arn:aws:iam::1234:role/x"`
- **THEN** STS `AssumeRole` is called with that ARN at run time

#### Scenario: Without role_arn
- **WHEN** an AWS connector has no `role_arn`
- **THEN** the run uses ambient AWS credentials from the simrun process

### Requirement: External ID From Secret
The system SHALL pass `SR_AWS_EXTERNAL_ID` from the linked secret group
to STS `AssumeRole` as the `ExternalId` parameter when present. The
key SHALL NOT be forwarded as an env var into the run environment
(it is consumed by the AssumeRole call only).

#### Scenario: External ID consumed
- **WHEN** the linked secret group has `SR_AWS_EXTERNAL_ID = "abc"`
- **THEN** `AssumeRole` is invoked with `ExternalId = "abc"` and the value is not present in the run env

### Requirement: Session Duration One Hour
The system SHALL request a 1-hour session duration on `AssumeRole`.

#### Scenario: Session duration
- **WHEN** AssumeRole is invoked
- **THEN** the request `DurationSeconds` is 3600

### Requirement: Credentials Injected as AWS_* Env Vars
The system SHALL inject the resulting credentials as
`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, and `AWS_SESSION_TOKEN`
into the run environment.

#### Scenario: Env injection
- **WHEN** AssumeRole returns credentials
- **THEN** the run env contains the three `AWS_*` keys

### Requirement: Other Secret Entries Pass Through
The system SHALL inject all entries in the linked secret group into the
run environment except `SR_AWS_EXTERNAL_ID`. **Note:** entries with
keys identical to `AWS_ACCESS_KEY_ID`/`SECRET_ACCESS_KEY`/`SESSION_TOKEN`
will be overwritten by the AssumeRole-derived values.

#### Scenario: Pass-through
- **WHEN** the secret group has `MY_CUSTOM_VAR=v`
- **THEN** the run env contains `MY_CUSTOM_VAR=v`

### Requirement: is_default Allowed
The system SHALL allow `is_default = true` on AWS connectors, subject to
the umbrella one-default-per-cloud-type rule.

#### Scenario: Default AWS connector
- **WHEN** no other AWS connector is the default and a client sets `is_default: true`
- **THEN** the response is success

### Requirement: Test Connection Implemented
The system SHALL implement `POST /api/connectors/test` for `type: "aws"`
by performing AssumeRole (if `role_arn` set) and then executing
`aws sts get-caller-identity` as a subprocess with the resolved
credentials. Either step's failure SHALL return
`{success: false, error}`.

#### Scenario: Successful test
- **WHEN** AssumeRole succeeds and `aws sts get-caller-identity` returns
- **THEN** the response is `{success: true}`

#### Scenario: AWS CLI missing
- **WHEN** the `aws` binary is not on PATH
- **THEN** the test response is `{success: false, error: <message>}`

### Requirement: AssumeRole Failure Surfaces
The system SHALL surface AssumeRole errors (bad ARN, permission denied,
invalid external ID) in the test response and at run time.

#### Scenario: Permission denied
- **WHEN** the simrun process's identity is not allowed to assume the configured role
- **THEN** test returns `{success: false, error}` and a run using the connector fails before detonation
