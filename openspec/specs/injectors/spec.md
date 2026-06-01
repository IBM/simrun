# Injectors Specification

## Purpose
Injects synthetic security events directly into a SIEM as an alternative
to detonation. The only injector implemented today is `ElasticInjector`,
which renders document templates with run-time variables and indexes them
into Elasticsearch. This spec covers both the umbrella injector contract
and the Elastic implementation; a separate spec exists for matchers/
collectors that consume injector output.

## Requirements

### Requirement: Inject Returns Execution ID
The system SHALL define an `Inject(ctx) -> (map[string]string, error)`
contract on every injector. The returned map MUST contain
`"execution_id"` on success.

#### Scenario: Successful injection
- **WHEN** an Elastic injector successfully indexes its documents
- **THEN** the returned map has `"execution_id": "<uuid>"`

### Requirement: Execution ID Is UUIDv4
The system SHALL generate a fresh UUIDv4 per injection invocation. **Note:**
this differs from `SimrunDetonator`, which uses nanoids. Matchers must
not assume execution IDs are always UUIDs across all executor types.

#### Scenario: Format
- **WHEN** the Elastic injector runs
- **THEN** `execution_id` parses as a v4 UUID

### Requirement: Orchestrator Stanza Always Injected
The system SHALL append the keys `orchestrator.type = "simrun"`,
`orchestrator.resource.type = "scenario"`, and
`orchestrator.resource.id = <executionID>` to every indexed document
unconditionally, after template rendering. These keys are written with
literal dots in the key name.

#### Scenario: Orchestrator fields present
- **WHEN** the Elastic injector indexes any document
- **THEN** the indexed `_source` contains the orchestrator keys above

### Requirement: Built-In Template Variables
The system SHALL expose `ExecutionID` and `Timestamp` (RFC3339) as Go
template variables to every document template. User-supplied entries in
`doc.Vars` named `ExecutionID` or `Timestamp` SHALL be silently overridden
by the built-ins. **Note:** documented behavior; user keys with these
names have no effect.

#### Scenario: Template uses ExecutionID
- **WHEN** a document template contains `"id": "{{.ExecutionID}}"`
- **THEN** the indexed document has `id` set to the run's execution UUID

### Requirement: Document Source Mutual Exclusion
The system SHALL accept exactly one of `file` (path to a local JSON
template) or `template + pack` (named pack template) per document. JSON
schema validation SHALL reject any document that has neither, has both,
or has `template` without `pack`.

#### Scenario: Template without pack
- **WHEN** a document specifies `template: "T"` without a `pack` field
- **THEN** parsing fails before injection runs

### Requirement: Pack Template Resolution at Parse Time
The system SHALL fetch and decode pack templates at parse time (not at
inject time) by invoking the pack binary's `manifest` command and
base64-decoding the named template content. **Note:** this means an
unreachable pack causes the entire run request to fail before any
documents are indexed.

#### Scenario: Pack manifest fetched once
- **WHEN** a scenario references templates `T1` and `T2` from the same pack
- **THEN** the pack's manifest is fetched once at parse time and cached for both templates

### Requirement: Atomic-Per-Document Indexing
The system SHALL index documents sequentially. On the first failure, the
injector SHALL return immediately with that error; documents already
indexed SHALL NOT be rolled back. **Note:** there is no partial-success
signal exposed to callers.

#### Scenario: Mid-list failure
- **WHEN** documents 1, 2, and 3 are configured and Elasticsearch rejects document 2
- **THEN** documents 1 is already indexed in Elasticsearch and the injector returns the error for document 2 without attempting document 3

### Requirement: Index Name Pass-Through
The system SHALL use the document's `index` field verbatim as the target
Elasticsearch index. The injector SHALL NOT add prefixes or namespacing.

#### Scenario: Custom index
- **WHEN** a document specifies `index: "logs-custom-default"`
- **THEN** Elasticsearch's `Index` API is invoked against `logs-custom-default`

### Requirement: Connection Configuration Priority
The system SHALL prefer `SR_ELASTIC_CLOUD_ID` over `SR_ELASTIC_URL` for the
endpoint, and `SR_ELASTIC_API_KEY` over `SR_ELASTIC_USERNAME` /
`SR_ELASTIC_PASSWORD` for authentication.

#### Scenario: Cloud ID overrides URL
- **WHEN** both `SR_ELASTIC_CLOUD_ID` and `SR_ELASTIC_URL` are set
- **THEN** the client uses `SR_ELASTIC_CLOUD_ID`

### Requirement: Message Field Coercion
The system SHALL marshal the `message` field to a JSON string when its
parsed value is an object. Strings and other primitive values SHALL pass
through unchanged.

#### Scenario: Object message
- **WHEN** a rendered document has `message: {"event":"failed_login"}`
- **THEN** the indexed document has `message` as the JSON string `'{"event":"failed_login"}'`

### Requirement: Template-Rendering Errors Are Fatal Per Document
The system SHALL fail the injection (returning an error) when a rendered
template does not parse as valid JSON or when the template execution
itself errors.

#### Scenario: Invalid rendered JSON
- **WHEN** a template renders to a string that is not valid JSON
- **THEN** the injector returns an error and stops processing further documents

### Requirement: Elasticsearch Error Surfaces Status and Body
The system SHALL return an error containing the HTTP status and the
response body when Elasticsearch responds 4xx or 5xx.

#### Scenario: 400 from index
- **WHEN** Elasticsearch rejects a document with a 400 mapping error
- **THEN** the injector returns an error including the status code and the response body
