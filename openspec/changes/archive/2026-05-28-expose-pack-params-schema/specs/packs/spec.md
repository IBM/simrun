## MODIFIED Requirements

### Requirement: Pack Parameters Storage
The system SHALL store a pack's parameters as a JSONB map. Storage
itself SHALL accept arbitrary JSON values; validation is enforced at
the API layer against the pack's declared schema, not at the column
level. **Note:** invalid Terraform variable names will fail at run time
when passed as `TF_VAR_*` env vars.

#### Scenario: Storage accepts arbitrary JSON
- **WHEN** a row is updated with `{ "region": "us-east-1", "size": 3 }`
- **THEN** the row's `parameters` JSONB stores the values verbatim

### Requirement: Get and Update Parameters
The system SHALL respond to `GET /api/packs/{name}/parameters` with the
current `parameters` JSONB and to `PUT /api/packs/{name}/parameters`
with a full replacement (no merge). Replacement SHALL drop keys absent
from the request. On `PUT`, the system SHALL fetch the pack's
`params_schema` (via the manifest command) and SHALL strict-validate
every request key that matches a declared schema property: declared
keys SHALL pass type check, enum membership check, and the required-key
check. Unknown keys (present in the request but absent from the
schema) SHALL be persisted alongside declared keys without rejection.
The response body SHALL include both the persisted parameters and a
list of unknown keys so the client can render a soft warning.

#### Scenario: Replace not merge
- **WHEN** the existing parameters are `{a:1,b:2}` and a client PUTs `{a:3}`
- **THEN** the persisted parameters are `{a:3}` (key `b` is removed)

#### Scenario: Strict validation rejects type mismatch on declared key
- **WHEN** the pack declares `aws_region` as a string and a client
  PUTs `{"aws_region": 5}`
- **THEN** the request is rejected with a structured validation error
  naming `aws_region` and the expected type

#### Scenario: Strict validation rejects enum violation
- **WHEN** the pack declares `aws_region` with `enum: ["us-east-1", "us-west-2"]`
  and a client PUTs `{"aws_region": "eu-west-9"}`
- **THEN** the request is rejected with a structured validation error
  naming `aws_region` and listing the allowed values

#### Scenario: Strict validation rejects missing required custom param
- **WHEN** the pack declares `vpc_id` with `required: true` and a
  client PUTs a body without `vpc_id`
- **THEN** the request is rejected with a structured validation error
  naming `vpc_id`

#### Scenario: Unknown keys are kept and surfaced
- **WHEN** the pack's schema lists `aws_region` and the client PUTs
  `{"aws_region": "us-east-1", "legacy_key": "x"}`
- **THEN** the persisted parameters contain both keys, and the
  response body includes `"unknown_keys": ["legacy_key"]`

#### Scenario: Pack with no schema falls back to permissive storage
- **WHEN** a pack's manifest returns no `params_schema` and a client
  PUTs `{"anything": "goes"}`
- **THEN** the request is accepted, the value is persisted verbatim,
  and the response's `unknown_keys` list contains every key in the
  request
