# Datadog Matcher Specification

## Purpose
Verifies expected Datadog Security signals via the
`SearchSecurityMonitoringSignals` API. Polls for open signals filtered
by rule name and (optionally) severity, then matches indicators against
the signal's `Attributes.Custom` JSON. Cleanup archives matched signals.
Umbrella behavior (polling cadence, indicator-substring strategy,
explore mode) is in `matchers`.

## Requirements

### Requirement: Eager Credential Initialization
The system SHALL read Datadog credentials at matcher construction time,
not lazily. Credentials are sourced first from
`SR_DATADOG_API_KEY` / `SR_DATADOG_APP_KEY` / `SR_DATADOG_SITE`, with
fallback to `DD_API_KEY` / `DD_APP_KEY` / `DD_SITE`. Missing credentials
SHALL build a non-functional matcher (API calls return 403 at poll
time).

#### Scenario: Construction without creds
- **WHEN** the matcher is constructed and no Datadog env vars are set
- **THEN** construction succeeds but `HasExpectedAlert` calls fail with auth errors

### Requirement: Default Site
The system SHALL default `Site` to `"datadoghq.com"` when no site env var
is set.

#### Scenario: Default site
- **WHEN** neither `SR_DATADOG_SITE` nor `DD_SITE` is set
- **THEN** API calls go to `datadoghq.com`

### Requirement: Legacy Env Var Deprecation
The system SHALL log a deprecation warning when any of
`DD_API_KEY` / `DD_APP_KEY` / `DD_SITE` are used.

#### Scenario: Legacy keys logged
- **WHEN** the matcher initializes from `DD_API_KEY`
- **THEN** a warning log entry is emitted noting the deprecation

### Requirement: Match Query
The system SHALL build the Datadog query as
`@workflow.triage.state:open @workflow.rule.name:"<name>"` with an
optional `status:<severity>` clause appended when severity is set.

#### Scenario: With severity
- **WHEN** an expectation has `severity: "info"`
- **THEN** the Datadog query is `@workflow.triage.state:open @workflow.rule.name:"<name>" status:info`

### Requirement: Severity Free-Form
The system SHALL accept any string for `severity` and pass it verbatim
to the query as `status:<value>`. Unlike the Elastic matcher there is
no enum constraint.

#### Scenario: Custom severity accepted
- **WHEN** an expectation has `severity: "notice"`
- **THEN** parsing succeeds and the query includes `status:notice`

### Requirement: Fixed One-Hour Time Window
The system SHALL search signals from `time.Now().Add(-1h)` on every
poll iteration. **Note:** unlike the Elastic matcher which pins to run
start, the Datadog matcher uses a rolling 1-hour window. Long-running
scenarios or scenarios that start near an hour boundary may miss
signals or pick up signals from prior runs. Flagged as a known
asymmetry.

#### Scenario: Rolling window
- **WHEN** the matcher polls at time T
- **THEN** the query's `From` is `T - 1h`

### Requirement: Hard Limit of 1000 Signals
The system SHALL fail with the error
`"unsupported: more than 1000 open signals"` if a search returns 1000
signals. **Note:** there is no pagination today; flagged as a known
scaling gap.

#### Scenario: Too many signals
- **WHEN** the open-signal count exceeds 1000
- **THEN** the matcher returns the unsupported-pagination error

### Requirement: Indicator Substring Match Against Custom Attributes
The system SHALL serialize each candidate signal's `Attributes.Custom`
to JSON and treat the signal as matched when any indicator string
appears as a substring (`strings.Contains`).

#### Scenario: Match in custom attribute
- **WHEN** a signal's `Attributes.Custom` JSON contains the run's execution UUID
- **THEN** the matcher returns matched

### Requirement: Cleanup Archives Matched Signals
The system SHALL, on `Cleanup`, query all open signals (without rule-name
filtering) via `QueryAllOpenSignals = "@workflow.triage.state:open"`,
filter by indicators, and `PATCH /api/v1/security_analytics/signals/<id>/state`
with `state: "archived"` and `archiveReason: "testing_or_maintenance"`.
**Note:** the cleanup query is broader than the match query; in noisy
environments signals from other rules could be inadvertently archived
if their custom attributes happen to contain an indicator string.
Flagged as a known cleanup-broadness risk.

#### Scenario: Archive matched signal
- **WHEN** a signal contains an indicator and `Cleanup` is invoked
- **THEN** the signal is patched to `archived` with reason `testing_or_maintenance`
