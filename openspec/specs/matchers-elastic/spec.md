# Elastic Matcher Specification

## Purpose
Verifies expected Elastic Security alerts via the Kibana Detection Engine
API. Polls `POST /api/detection_engine/signals/search` filtered by rule
name, then matches indicators against the alert's full document. Cleanup
closes matched alerts via the signal-status endpoint. Umbrella behavior
(polling cadence, indicator-substring strategy, explore mode) is in
`matchers`.

## Requirements

### Requirement: Match Query Composition
The system SHALL build the Detection Engine query as a `bool` with these
clauses: `match_phrase` on `kibana.alert.rule.name` (exact phrase match
with the YAML expectation's rule name), `term` on
`kibana.alert.workflow_status = "open"`, `must_not exists` on
`kibana.alert.building_block_type`, optional `term` on
`kibana.alert.severity` (when severity is in the YAML expectation), and
a time-range filter (see `Since` requirement).

#### Scenario: Acknowledged alert excluded
- **WHEN** an alert with the configured rule name has `workflow_status = "acknowledged"`
- **THEN** the matcher does not consider it a candidate

#### Scenario: Building block excluded
- **WHEN** an alert is a building-block alert
- **THEN** the matcher does not consider it a candidate

### Requirement: Severity Enum
The system SHALL accept `severity` values from the set
{`low`, `medium`, `high`, `critical`} only. The parser SHALL reject other
values; the matcher SHALL include the `term` clause when severity is
set.

#### Scenario: Severity included in query
- **WHEN** an Elastic expectation has `severity: "high"`
- **THEN** the query includes `term: kibana.alert.severity = "high"`

### Requirement: Run-Time Time Window
The system SHALL apply a time-range filter via `SetSince(start)` set by
the runner at scenario start. Alerts with `@timestamp < start` SHALL NOT
be returned. When `SetSince` is not called (e.g., legacy CLI use), the
filter SHALL fall back to `now-15m`.

#### Scenario: Pinned to run start
- **WHEN** a scenario starts at `T` and the matcher's `SetSince(T)` has been called
- **THEN** the query filter is `range: {@timestamp: {gte: T}}`

### Requirement: Result Limit
The system SHALL request up to 1000 alerts per query. Indicator matching
proceeds against this set.

#### Scenario: 1000-result query
- **WHEN** the matcher calls Detection Engine search
- **THEN** the request body includes `size: 1000`

### Requirement: Indicator Substring Match
The system SHALL serialize each candidate alert's full `_source` to JSON
and treat the alert as matched when any indicator string appears as a
substring (`strings.Contains`). This is the broad correlation strategy
shared with the Datadog matcher.

#### Scenario: Execution ID found in any field
- **WHEN** an alert document contains the execution UUID anywhere in `_source`
- **THEN** the matcher returns matched

### Requirement: Lazy Credential Read
The system SHALL read `SR_KIBANA_URL` and `SR_ELASTIC_API_KEY` from the
run environment on the first call to `HasExpectedAlert` or `Cleanup`,
not at construction time. Missing credentials SHALL cause that first
call to error.

#### Scenario: Lint without creds
- **WHEN** a scenario is parsed without env vars
- **THEN** the matcher object is constructed without contacting Kibana

#### Scenario: Run-time credential miss
- **WHEN** the matcher's first poll runs and `SR_KIBANA_URL` is empty
- **THEN** the matcher returns an error from `HasExpectedAlert`

### Requirement: Cleanup Closes Matched Alerts
The system SHALL, on `Cleanup`, re-run the same matching query and call
`POST /api/detection_engine/signals/status` with `status: "closed"` for
every matched alert.

#### Scenario: All matches closed
- **WHEN** three alerts match the indicators
- **THEN** all three are transitioned to `closed` status in Kibana

### Requirement: No-Match Returns Nil-Nil
The system SHALL return `(nil, nil)` (no error, no match) from
`searchAndMatch` when the query returns alerts but none contain any
indicator. **Note:** captured as the contract; callers in the runner
distinguish "no match" from errors via the nil-nil pattern.

#### Scenario: Empty match
- **WHEN** alerts exist with the rule name but none contain any indicator
- **THEN** `searchAndMatch` returns `(nil, nil)`
