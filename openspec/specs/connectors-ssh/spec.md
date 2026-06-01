# SSH Connector Specification

## Purpose
Type-specific behavior of `ssh`-typed connectors. Used by
`remoteDetonator` scenarios to establish an SSH session to a target host.
Configuration carries host/user/port; the linked secret group carries the
private key. At run time, the key is materialized to a temp file and its
path is exposed as `SR_SSH_KEY`.

## Requirements

### Requirement: Required Fields
The system SHALL require `config.host` and `config.username` to be
non-empty strings at create and update time. `config.port` SHALL be
optional, in the range 0–65535; the value 0 means "use SSH default 22".

#### Scenario: Missing host
- **WHEN** a client creates an SSH connector with empty `host`
- **THEN** the response is HTTP 400

#### Scenario: Out-of-range port
- **WHEN** a client posts `port: 70000`
- **THEN** the response is HTTP 400

### Requirement: Linked Secret Group Provides Private Key
The system SHALL expect the linked secret group to contain entry
`SR_SSH_KEY` with a PEM-encoded private key. Other entries in the group
SHALL be merged flat into the run environment via the standard
secret-decryption step.

#### Scenario: Key entry present
- **WHEN** the linked secret group has `SR_SSH_KEY = <pem>`
- **THEN** the key is available for run-time materialization

### Requirement: Run-Time Env Injection
The system SHALL inject `SR_SSH_HOST`, `SR_SSH_USERNAME`, and (when
`port` is non-zero) `SR_SSH_PORT` into the run environment from the
connector config. The decrypted private key SHALL be written to a temp
file with mode `0600` and the file path SHALL be injected as
`SR_SSH_KEY`.

#### Scenario: Full env injection
- **WHEN** an SSH connector with `host="h", username="u", port=22` and a secret group with private key `K` is selected via `targets.ssh`
- **THEN** the run env contains `SR_SSH_HOST=h, SR_SSH_USERNAME=u, SR_SSH_PORT=22, SR_SSH_KEY=<temp file path containing K>`

### Requirement: Default Port Suppression
The system SHALL omit `SR_SSH_PORT` from the run environment when the
configured `port` is 0; the SSH executor SHALL fall back to port 22.

#### Scenario: No port configured
- **WHEN** an SSH connector has `port: 0`
- **THEN** the run env does not contain `SR_SSH_PORT`

### Requirement: Temp Key File Permissions
The system SHALL create the temp key file with permissions `0600`. **Note:**
the temp file is not explicitly cleaned up during the run lifecycle;
flagged as a known cleanup gap (the OS reclaims on reboot).

#### Scenario: Permissions
- **WHEN** the temp key file is created
- **THEN** its mode is `0600`

### Requirement: is_default Allowed
The system SHALL allow `is_default = true` on SSH connectors, subject to
the umbrella one-default-per-cloud-type rule.

#### Scenario: Default SSH connector
- **WHEN** no other SSH connector is the default
- **THEN** setting `is_default: true` succeeds

### Requirement: Test Connection Unsupported
The system SHALL return `{success: false, error: "unsupported connector type: ssh"}`
from `POST /api/connectors/test` when `type: "ssh"`. **Note:** flagged
as a documented gap — SSH connectivity can be created and used at run
time, but cannot be pre-validated through the API.

#### Scenario: SSH test rejected
- **WHEN** a client posts an SSH test
- **THEN** the response is HTTP 200 with `{success: false, error: "unsupported connector type: ssh"}`
