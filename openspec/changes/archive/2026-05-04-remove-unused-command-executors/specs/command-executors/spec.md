## REMOVED Requirements

### Requirement: Common CommandDetonator Interface
**Reason**: The `CommandDetonator` interface and both implementations (`LocalCommandExecutor`, `SSHCommandExecutor`) are deleted from `simrun/internal/detonators/`. No scenario, pack, or test in tree exercises them. The team plans to re-implement command-based detonation with a different design (env propagation, host-key verification, per-command timeout) under a fresh spec.
**Migration**: None. No saved scenarios in tree or in any deployed environment use `localDetonator` or `remoteDetonator`. The replacement, when introduced, will be proposed as a new change with its own spec.

### Requirement: Detonation UUID Wraps Each Command
**Reason**: Implementation deleted along with the executors.
**Migration**: None.

### Requirement: Bash Path Hardcoded
**Reason**: Implementation deleted along with the executors.
**Migration**: None. The next iteration is expected to remove the `/bin/bash` portability constraint.

### Requirement: SetEnvVars Is a No-Op for Command Detonators
**Reason**: Implementation deleted along with the executors.
**Migration**: None. The next iteration is expected to propagate env vars to commands.

### Requirement: Local Executor Runs On Server Host
**Reason**: `LocalCommandExecutor` deleted.
**Migration**: None.

### Requirement: SSH Executor Resolves Target From Connector
**Reason**: `SSHCommandExecutor` deleted. The SSH connector type and the `targets.ssh` resolver in `simrun/internal/web/scenarios.go` are kept verbatim for re-use by the planned re-implementation, but no executor consumes them after this change.
**Migration**: None.

### Requirement: SSH Key Materialization
**Reason**: Tied to the deleted `SSHCommandExecutor`. The temp-key-file materialization code in `simrun/internal/web/scenarios.go` (`resolveConnectorCreds` case `"ssh"`) remains in source but is unreachable after this change; it is documented as orphan code in the change's `design.md`.
**Migration**: None.

### Requirement: SSH Host Key Verification Disabled
**Reason**: Implementation deleted along with the executors.
**Migration**: None. The next iteration is expected to enable host-key verification.

### Requirement: SSH Connection Reuse Within a Run
**Reason**: Implementation deleted along with the executors.
**Migration**: None.

### Requirement: SSH Session Logging
**Reason**: The `SR_SSH_LOG_DIR` env var injection in `simrun/internal/web/scenarios.go` is preserved (it powers terraform pack SSH log capture in `simrun/pack/ssh.go`, an unrelated code path). What is removed is the requirement that this env var is consumed by the SSH command executor — that consumer no longer exists.
**Migration**: None. `ssh_logging_enabled` config remains; it now serves only the terraform pack SSH log path.

### Requirement: No Per-Command Timeout
**Reason**: Implementation deleted along with the executors.
**Migration**: None. The next iteration is expected to enforce a per-command timeout.
