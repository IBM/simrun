## MODIFIED Requirements

### Requirement: Execution ID Format Varies By Detonator
The system SHALL allow each detonator to choose its execution-ID format:
`SimrunDetonator` SHALL emit a nanoid; `AWSCLIDetonator` and
`AWSDetonator` SHALL emit UUIDv4. **Note:** matchers and collectors must
therefore not assume a UUID format for correlation.

#### Scenario: Simrun pack uses nanoid
- **WHEN** `SimrunDetonator.Detonate` returns
- **THEN** `execution_id` is a URL-safe nanoid, not a UUID

### Requirement: Env Vars Threading Differs By Detonator
The system SHALL pass `SetEnvVars(env)` through to `SimrunDetonator`,
`AWSCLIDetonator`, and `AWSDetonator` to use as cloud credentials and
configuration. **Note:** captured per-type in sibling specs; flagged
here because matchers/collectors share env semantics.

#### Scenario: Env passes to terraform
- **WHEN** a `SimrunDetonator` is given `AWS_ACCESS_KEY_ID` in env
- **THEN** terraform invocations have that env var set
