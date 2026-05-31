# AWS CLI Detonator Specification

## Purpose
Type-specific behavior of `AWSCLIDetonator`, which runs an inline bash
script using the AWS CLI. Designed for ad-hoc scenarios that don't
require Terraform. The detonation UUID is stamped into AWS API calls
via `AWS_EXECUTION_ENV` so CloudTrail records carry a correlation
token.

## Requirements

### Requirement: ExecutionID Is UUIDv4
The system SHALL generate the execution ID as a UUIDv4 per detonation.

#### Scenario: Format
- **WHEN** `AWSCLIDetonator.Detonate` returns
- **THEN** `execution_id` parses as a v4 UUID

### Requirement: Pre-Flight Credential Check
The system SHALL call `Credentials.Retrieve()` on the loaded AWS SDK
config before executing the bash script. A retrieval error SHALL fail
detonation with the message `"you are not authenticated to AWS"`
without running the script.

#### Scenario: No credentials
- **WHEN** AWS credentials are not available in the run env or ambient chain
- **THEN** `Detonate` returns `"you are not authenticated to AWS"` and the script is not executed

### Requirement: AWS_EXECUTION_ENV Injection
The system SHALL set the env var `AWS_EXECUTION_ENV=simrun_<uuid>` for
the script process. AWS SDK calls within the script SHALL therefore
include this token in their User-Agent / `userAgent` field, and
CloudTrail entries SHALL carry the same token for correlation.

#### Scenario: Token in env
- **WHEN** the bash script invokes `aws s3 ls`
- **THEN** the AWS API call's CloudTrail `userAgent` field contains `simrun_<uuid>`

### Requirement: Script Executed via Bash -c
The system SHALL execute the user-supplied script via
`exec.Command("bash", "-c", script)`. Combined stdout and stderr SHALL
be captured. A non-zero exit code SHALL fail detonation with an error
that includes the full combined output.

#### Scenario: Exit non-zero
- **WHEN** the script exits with code 2 and prints "denied" to stderr
- **THEN** `Detonate` returns an error containing the captured "denied" output

### Requirement: Multi-Line Scripts Supported
The system SHALL accept multi-line scripts as a single `script` string
(YAML literal block). Bash heredoc semantics SHALL apply.

#### Scenario: Multi-line script
- **WHEN** the YAML supplies a multi-line script with several `aws` commands
- **THEN** all commands run in sequence in one bash process

### Requirement: SimulationId Is The Script Name
The system SHALL set `SimulationId()` to the YAML-supplied scenario or
script identifier (not a generic placeholder).

#### Scenario: Identifier passed through
- **WHEN** the AWS CLI scenario has `name: "create-iam-user"`
- **THEN** `SimulationId()` returns `"create-iam-user"`

### Requirement: No Terraform Lifecycle
The system SHALL NOT perform any Terraform setup, apply, or destroy.
Scenarios needing infrastructure must use `simrunDetonator` with a
Terraform-enabled pack.

#### Scenario: No working directory
- **WHEN** an AWSCLI scenario runs
- **THEN** no `<DataDir>/terraform/` subdirectory is created
