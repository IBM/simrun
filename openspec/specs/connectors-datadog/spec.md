# Datadog Connector Specification

## Purpose
Type-specific behavior of `datadog`-typed connectors. Datadog is the
SIEM used by `DatadogSecuritySignalMatcher`. Credentials are not
resolved through the same `buildConnectorCredentials` path as cloud
connectors; instead the linked secret group's entries are merged flat
into the run environment by the secret-loading step.

## Requirements

### Requirement: Loose Config Validation
The system SHALL accept any `config` JSONB on create and update without
validating Datadog-specific fields server-side. The handler SHALL only
enforce the umbrella requirements (`name` and `type`).

#### Scenario: Empty config accepted
- **WHEN** a client creates a Datadog connector with `config: {}`
- **THEN** the response is success

### Requirement: Credentials Via Linked Secret Group
The system SHALL expect Datadog credentials in the linked secret group
under the keys `SR_DATADOG_API_KEY`, `SR_DATADOG_APP_KEY`, and
optionally `SR_DATADOG_SITE`. These are merged flat into the run
environment by the standard secret-decryption step at run time.

#### Scenario: Run-time creds
- **WHEN** the linked secret group has `SR_DATADOG_API_KEY=K, SR_DATADOG_APP_KEY=P`
- **THEN** the run environment contains those keys

### Requirement: Legacy Env Var Fallback
The system SHALL also accept `DD_API_KEY`, `DD_APP_KEY`, and `DD_SITE`
in the run environment for the Datadog matcher, and SHALL log a
deprecation warning when they are used.

#### Scenario: Legacy keys used
- **WHEN** the run env has `DD_API_KEY` but not `SR_DATADOG_API_KEY`
- **THEN** the matcher uses `DD_API_KEY` and a deprecation warning is logged

### Requirement: Default Site
The system SHALL default `Site` to `"datadoghq.com"` when neither
`SR_DATADOG_SITE` nor `DD_SITE` is set.

#### Scenario: No site configured
- **WHEN** neither site env var is set
- **THEN** Datadog API calls go to `datadoghq.com`

### Requirement: is_default Forbidden
The system SHALL reject any create or update that sets `is_default: true`
on a Datadog connector with HTTP 400. Datadog is not in the cloud-type
allowlist for default selection.

#### Scenario: Default attempt
- **WHEN** a client posts a Datadog connector with `is_default: true`
- **THEN** the response is HTTP 400

### Requirement: Test Connection Unsupported
The system SHALL return `{success: false, error: "unsupported connector type: datadog"}`
from `POST /api/connectors/test` when `type: "datadog"`. **Note:**
this is a documented gap — there is no Datadog test path implemented
today.

#### Scenario: Datadog test rejected
- **WHEN** a client posts a Datadog test
- **THEN** the response is HTTP 200 with `{success: false, error: "unsupported connector type: datadog"}`
