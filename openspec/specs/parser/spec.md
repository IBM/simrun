# Parser Specification

## Purpose
Parses YAML scenario files into in-memory `Scenario` objects ready for
execution. The parser converts YAML to JSON, validates against a JSON
schema (generated Go types in `simrun/internal/parser/parser.go`), applies
defaults, resolves pack references, and constructs detonators, injectors,
matchers, and collectors. The parser is the single ingress for scenario
definitions; both `lint` and `run` flows go through it.

## Requirements

### Requirement: YAML Input Validated Against JSON Schema
The system SHALL convert YAML to JSON and unmarshal into generated Go
structs whose `UnmarshalJSON` methods enforce the JSON schema in
`simrun/schemas/simrun.schema.json`. Schema violations SHALL be returned
as parse errors with the offending field name.

#### Scenario: Missing required field
- **WHEN** a YAML document is parsed without the top-level `scenarios` key
- **THEN** `ParseWithOptions` returns an error referencing the missing required field

#### Scenario: Wrong type
- **WHEN** a field that requires a string contains a number
- **THEN** parsing fails with a type-error message naming the field

### Requirement: Non-Empty Scenarios List
The system SHALL require at least one scenario in the `scenarios` array.
A YAML where every scenario has `enabled: false` MUST also fail.

#### Scenario: Empty array
- **WHEN** the YAML contains `scenarios: []`
- **THEN** parsing returns `"input file has no scenarios defined"`

#### Scenario: All disabled
- **WHEN** every scenario in the YAML has `enabled: false`
- **THEN** parsing returns `"all scenarios are disabled"`

### Requirement: Disabled Scenarios Skipped
The system SHALL omit any scenario with `enabled: false` from the returned
`Scenarios` slice. The `enabled` field SHALL default to `true` when absent.

#### Scenario: Mixed enabled and disabled
- **WHEN** a YAML has scenarios `A` (enabled), `B` (disabled by `enabled: false`), and `C` (default)
- **THEN** the result contains scenarios `A` and `C` only

### Requirement: Scenario Has Detonate or Inject
The system SHALL require every scenario to have exactly one of `detonate`
or `inject`. A scenario with neither SHALL fail parsing.

#### Scenario: Missing executor
- **WHEN** a scenario has no `detonate` and no `inject` block
- **THEN** parsing returns `"scenario '<name>' has no detonation or injection defined"`

### Requirement: Scenario Has At Least One Expectation
The system SHALL require every scenario to have at least one `expectations`
entry.

#### Scenario: Empty expectations
- **WHEN** a scenario has `expectations: []`
- **THEN** parsing returns `"scenario '<name>' has no assertions defined"`

### Requirement: Expectation Timeout Defaults to 5 Minutes
The system SHALL default `expectations[].timeout` to `"5m"` when absent.
The scenario timeout used by the runner SHALL be taken from the FIRST
expectation only; per-expectation timeout overrides on the second and
subsequent expectations are silently ignored. **Note:** known
simplification, captured here as the contract.

#### Scenario: Default timeout
- **WHEN** an expectation does not specify `timeout`
- **THEN** the parsed expectation has `Timeout = "5m"`

#### Scenario: First-only timeout
- **WHEN** a scenario has expectations with timeouts `"10m"` then `"30m"`
- **THEN** the runner uses `10m` as the scenario timeout

### Requirement: Pack Reference Resolution
For a `simrunDetonator`, the system SHALL look up the named pack in
`ParseOptions.Packs`. If the pack is not present, parsing SHALL fail.
The full pack list MUST be supplied by the caller (the web layer loads
all packs from the DB before parsing).

#### Scenario: Unknown pack
- **WHEN** a scenario references `pack: "missing"` and `Packs` does not contain a pack with that name
- **THEN** parsing returns `"scenario '<name>' references unknown pack 'missing'"`

### Requirement: Pack Template Fetch at Parse Time
The system SHALL fetch and cache pack manifests at parse time for any
`elasticInjector` document that references `template + pack`, base64-decode
template content, and store it in an in-memory cache. If the pack binary is
unreachable or returns a malformed manifest, parsing SHALL fail.

#### Scenario: Successful template resolution
- **WHEN** a document has `template: "T1"` and `pack: "P"` and `P` advertises template `T1` in its manifest
- **THEN** the parsed injector holds the decoded template content for `T1`

#### Scenario: Pack unreachable
- **WHEN** a referenced pack's binary cannot be executed
- **THEN** parsing returns an error and no scenarios are returned

### Requirement: Elastic Severity Enum
The system SHALL accept Elastic alert `severity` values from the set
{`low`, `medium`, `high`, `critical`} only. Any other value SHALL fail
parsing.

#### Scenario: Invalid severity
- **WHEN** an Elastic expectation has `severity: "moderate"`
- **THEN** parsing returns an error listing the valid options

### Requirement: Datadog Severity Free-Form
The system SHALL accept any string value for Datadog `severity` and pass it
verbatim to the Datadog query as `status:<value>`.

#### Scenario: Custom severity
- **WHEN** a Datadog expectation has `severity: "notice"`
- **THEN** parsing succeeds and the matcher is constructed with that value

### Requirement: Targets Block
The `targets:` top-level block SHALL be parsed into a `map[string]string` of
target type → connector name, supporting keys: `aws`, `gcp`, `azure`,
`kubernetes`, `ssh`. When the block is absent or all keys are absent, the
parsed `Targets` SHALL be `nil`.

#### Scenario: Subset of keys
- **WHEN** the YAML has `targets: {aws: "prod-aws"}`
- **THEN** the parsed result has `Targets["aws"] = "prod-aws"` and no other keys

### Requirement: Indicators Pass-Through
The system SHALL copy `indicators.terraformOutput` and `indicators.static`
from the YAML to the runner-level `Scenario.Indicators` without
interpretation. Resolution against detonation outputs happens at run time.

#### Scenario: Indicators present
- **WHEN** a scenario specifies `indicators: {static: ["foo"], terraformOutput: ["bar"]}`
- **THEN** the runner-level `Scenario` has those values copied through

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
