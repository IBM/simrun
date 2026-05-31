## REMOVED Requirements

### Requirement: SSH Detonator Requires Run-Time Env
**Reason**: The `remoteDetonator` YAML field and its backing `SSHCommandExecutor` are removed. With no consumer, the parser-level requirement that `SR_SSH_HOST`/`SR_SSH_USERNAME` be present in `ParseOptions.EnvVars` is moot.
**Migration**: None. The replacement command-executor will introduce its own env-vars contract.

## MODIFIED Requirements

### Requirement: Lint Endpoint Behavior
The lint code path SHALL invoke the parser with `EnvVars=nil` and
`DataDir=""`, and return a `LintResponse` summarizing each parsed scenario
with `name, executorType, executorName, assertions` count. **Note:**
because `EnvVars` is empty and `DataDir` is empty, lint cannot fully
validate `simrunDetonator` scenarios that depend on run-time env or
DataDir; those report errors during lint that may not surface during a
real run.

#### Scenario: Valid scenario lint
- **WHEN** a client posts a valid scenario YAML to `/api/scenarios/lint`
- **THEN** the response is `{valid: true, scenarios: [{name, executorType, executorName, assertions}]}`

#### Scenario: Invalid YAML
- **WHEN** a client posts malformed YAML
- **THEN** the response is `{valid: false, error: "<message>"}`
