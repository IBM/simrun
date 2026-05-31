## MODIFIED Requirements

### Requirement: Parameter Injection Two Ways
The system SHALL pass pack parameters (from DB `packs.parameters`) into
both the `ManifestInput.Parameters` field on the manifest call and as
`TF_VAR_<key>=<value>` environment variables for terraform invocations.
Per-scenario `params:` from the YAML SHALL be merged into
`DetonateInput.Params` and ALSO promoted to `TF_VAR_*` env vars. When
both scopes set the same key, the per-scenario value SHALL take
precedence over the pack-level value; both layers SHALL be applied on
top of any `default = ...` declared on the matching Terraform
`variable` block. Pack-level values SHALL be passed for every key in
`packs.parameters`, regardless of whether the value was declared in
the pack's `params_schema` (so previously-stored unknown keys continue
to flow until cleaned up).

#### Scenario: Parameter as TF_VAR
- **WHEN** the pack DB record has `parameters: {"region":"us-east-1"}`
- **THEN** terraform inside the detonation runs with `TF_VAR_region=us-east-1`

#### Scenario: Per-scenario override of pack-level value
- **WHEN** the pack record has `parameters: {"aws_region": "us-east-1"}`
  and a scenario YAML sets `params: { aws_region: "us-west-2" }` for one
  simulation
- **THEN** that simulation's `terraform apply` runs with
  `TF_VAR_aws_region=us-west-2`, while sims without a scenario-level
  override still see `us-east-1`

#### Scenario: TF variable default used when neither scope provides value
- **WHEN** a sim's TF declares `variable "resource_prefix" { default = "simrun" }`
  and neither the pack record nor the scenario sets `resource_prefix`
- **THEN** terraform uses the declared default of `"simrun"`

#### Scenario: Pack-level unknown keys still flow
- **WHEN** the pack's stored parameters include a key not present in
  the pack's `params_schema` (e.g., a legacy key)
- **THEN** the key is still exported as a `TF_VAR_*` env var to the
  detonation's terraform process
