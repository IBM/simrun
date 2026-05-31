# Collectors Specification

## Purpose
Collects supporting telemetry from a SIEM after a scenario detonation, for
post-hoc inspection of what events the platform observed. Today only
`ElasticCollector` exists; it queries Elasticsearch by execution-ID
correlation and writes results as NDJSON files referenced from
`scenario_results.collected_log_path`. Collection is optional per
scenario.

## Requirements

### Requirement: Collect Returns Document Count
The system SHALL define a `Collect(ctx) -> (int, error)` contract on every
collector, where the int is the number of documents written. Zero
documents SHALL NOT be treated as an error.

#### Scenario: Empty result
- **WHEN** the Elastic collector runs and the search returns 0 hits
- **THEN** `Collect` returns `(0, nil)` and no error is logged at error level

### Requirement: Collection Runs Post-Detonation
The system SHALL invoke a configured collector after the scenario's
assertion phase concludes (whether by success, failure, or timeout). A
scenario without an assertion that requires waiting SHALL still wait for
its expectation timeout when in collect mode.

#### Scenario: Collect after assertion success
- **WHEN** a scenario's assertions pass at minute 2 of a 5-minute timeout
- **THEN** collection runs at that point, not at the full timeout

### Requirement: Collect Mode
The system SHALL detect "collect mode" when every assertion in a scenario
has a name ending in `" - collect mode"`. In collect mode, the runner
SHALL wait for the full scenario timeout before collecting and SHALL skip
assertion pass/fail evaluation.

#### Scenario: All assertions in collect mode
- **WHEN** every assertion in a scenario ends in `" - collect mode"` and the timeout is 5m
- **THEN** the runner waits 5 minutes and then runs the collector without recording assertion outcomes

### Requirement: Time Window Fixed at One Hour
The system SHALL apply a fixed `@timestamp >= now-1h` filter to the
Elasticsearch query. **Note:** this is independent of scenario start time;
events older than 1 hour at collection time will be silently missed
even if they are correlated to this run.

#### Scenario: Long-running scenario
- **WHEN** a scenario runs for 70 minutes and completes, then collection runs
- **THEN** events emitted in the first 10 minutes are excluded from the result

### Requirement: Correlation Query Structure
The system SHALL build a `bool.should` query (with `minimum_should_match: 1`)
combining: a wildcard against `<UserAgentField>` for `execution_id`, a
wildcard against `<UserAgentField>` for `execution_uuid`, and per-field
match-phrase clauses for any user-defined `additionalFields`. When
`UserAgentField` is empty, the user-agent clauses SHALL be omitted.

#### Scenario: Default correlation
- **WHEN** the scenario provides `execution_id` and `execution_uuid` in indicators and configures `userAgentField: "user_agent.original"`
- **THEN** the search matches any document whose `user_agent.original` contains either ID

### Requirement: Result Cap of 100 Documents
The system SHALL limit the search to 100 documents (sorted ascending by
`@timestamp`). When more matches exist, only the 100 oldest are returned.
**Note:** there is no pagination and no warning logged when truncation
occurs; flagged as a silent data limit.

#### Scenario: Truncation
- **WHEN** Elasticsearch holds 350 documents matching the query
- **THEN** the NDJSON output contains exactly 100 documents

### Requirement: Output is NDJSON with Metadata Fields
The system SHALL write one JSON object per line. Each line SHALL include
the document's `_source` merged with `_id` (Elasticsearch document ID)
and `_index` (Elasticsearch index name) injected at the top level.

#### Scenario: Metadata injection
- **WHEN** a hit has `_id="abc"` and `_index="logs-aws"`
- **THEN** the corresponding NDJSON line contains `_id: "abc"` and `_index: "logs-aws"` alongside the original source fields

### Requirement: Output Path Convention
The system SHALL write the NDJSON to
`<outputDir>/<simulationID|scenarioName>/<UTC-YYYYMMDD-HHMMSS>_<executionID>.ndjson`.
The first segment SHALL be `simulation_id` from indicators if present,
otherwise the scenario name. When `execution_id` is absent from
indicators, the filename SHALL be the timestamp without the `_<id>`
suffix.

#### Scenario: Full path
- **WHEN** indicators include `simulation_id="aws.iam.pivot"` and `execution_id="abc-123"`, and collection runs at `2026-05-03T14:30:00Z`
- **THEN** the output path is `<outputDir>/aws.iam.pivot/20260503-143000_abc-123.ndjson`

### Requirement: Indicator Templates in Additional Fields
The system SHALL resolve Go-style template references such as
`{{ indicators.terraformOutput.<key> }}` and `{{ indicators.static.<key> }}`
in `additionalFields` values against the flat indicators map. The prefix
(`terraformOutput.` or `static.`) SHALL be stripped before lookup.
References that do not resolve SHALL cause the affected field to be
silently dropped from the query (not an error).

#### Scenario: Resolved template
- **WHEN** an additional field is `"resource.id": "{{ indicators.terraformOutput.bucket_name }}"` and indicators contain `bucket_name="my-bucket"`
- **THEN** the query includes a match-phrase on `resource.id = "my-bucket"`

#### Scenario: Unresolved template
- **WHEN** the indicator key is missing
- **THEN** that field is omitted from the query and collection proceeds

### Requirement: Collector Persistence
The system SHALL store the absolute output path on the
`scenario_results.collected_log_path` column and the document count on
`scenario_results.collected_doc_count`. The path SHALL end in `.ndjson`.

#### Scenario: DB persistence
- **WHEN** collection writes 42 documents to a file
- **THEN** the scenario result has `collected_log_path` set to the absolute file path and `collected_doc_count = 42`

### Requirement: Connection Configuration Reuse
The system SHALL connect to Elasticsearch using the same
`SR_ELASTIC_CLOUD_ID` / `SR_ELASTIC_URL` / `SR_ELASTIC_API_KEY` resolution
as the injector and matcher.

#### Scenario: Same cluster as matcher
- **WHEN** the elastic matcher and elastic collector both run in a scenario
- **THEN** both connect to the same cluster identified by `SR_ELASTIC_CLOUD_ID` or `SR_ELASTIC_URL`
