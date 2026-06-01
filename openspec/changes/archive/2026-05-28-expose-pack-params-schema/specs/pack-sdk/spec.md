## ADDED Requirements

### Requirement: Pack Authors Declare Pack-Level Params via RegisterPackParams
The pack SDK SHALL expose a package-level function
`pack.RegisterPackParams(...pack.PackParam)` that pack authors call,
typically from `main()` near `pack.SetPackInfo(...)`, to declare custom
pack-level parameters. `PackParam` SHALL carry `Name`, `Type`
(one of `"string"`, `"boolean"`, `"object_string_map"`), `Description`,
`Default`, `Required`, and `Enum` fields. The function SHALL append the
provided specs into an SDK-internal registry and SHALL validate them
synchronously, panicking on author bugs the same way `pack.Register`
panics on bad simulation registration.

#### Scenario: Authors declare custom params
- **WHEN** a pack's `main` calls `pack.RegisterPackParams(pack.PackParam{Name: "resource_prefix", Type: "string", Default: "simrun"})`
- **THEN** the SDK records `resource_prefix` as a custom pack-level
  param and includes it in the manifest's `params_schema` output

#### Scenario: Calling without custom params is allowed
- **WHEN** a pack never calls `RegisterPackParams`
- **THEN** the SDK treats the custom-param list as empty and the
  manifest's `params_schema` contains only the built-in entries

### Requirement: RegisterPackParams Validates Author Input At Call Time
The pack SDK SHALL panic during `RegisterPackParams` when any of the
following conditions hold: a custom param's `Name` collides with a
reserved built-in name; a `Name` is duplicated within the call or
against previously-registered customs; `Default`'s Go type does not
match `Type`; or `Enum` is non-empty for a non-string `Type`. The
panic message SHALL name the offending param and the specific
validation rule violated.

#### Scenario: Reserved built-in name collision
- **WHEN** a pack calls `pack.RegisterPackParams(pack.PackParam{Name: "aws_region", Type: "string"})`
- **THEN** `RegisterPackParams` panics with a message stating that
  `aws_region` is a reserved built-in name

#### Scenario: Duplicate custom name
- **WHEN** a single call lists two `PackParam`s with the same `Name`
- **THEN** `RegisterPackParams` panics with a message naming the
  duplicate

#### Scenario: Default type mismatch
- **WHEN** a `PackParam{Type: "boolean", Default: "yes"}` is registered
- **THEN** `RegisterPackParams` panics with a message stating the
  default value type does not match the declared type

#### Scenario: Enum on non-string type
- **WHEN** a `PackParam{Type: "boolean", Enum: []string{"true"}}` is registered
- **THEN** `RegisterPackParams` panics with a message stating enum is
  only valid for string-typed params

### Requirement: SDK Ships A Fixed Registry Of Built-In Pack Params
The pack SDK SHALL provide a built-in pack-param registry containing,
at minimum: `default_tags` (`object<string,string>`), `aws_region`
(`string` with an enum of canonical AWS region identifiers),
`gcp_region` (`string` with an enum of canonical GCP region
identifiers), and `azure_location` (`string` with an enum of canonical
Azure location identifiers). The registry SHALL be sourced from SDK
code (not from author calls) and SHALL be the source of truth for both
schema entries and TF-rewriting rules. `gcp_project` SHALL NOT be a
built-in because project IDs are organization-specific and have no
useful default value.

#### Scenario: Built-ins always present in schema
- **WHEN** any pack's `manifest` command is invoked
- **THEN** the response's `params_schema.properties` includes entries
  for `default_tags`, `aws_region`, `gcp_region`, and `azure_location`,
  regardless of whether the author called `RegisterPackParams`

### Requirement: Manifest Response Carries Merged Pack Params Schema
The pack SDK SHALL include a top-level `params_schema` field on
`pack.ManifestResponse`. The field SHALL be a JSON document of the
shape `{"properties": {...}, "required": [...]}`, where `properties`
merges the built-in registry's entries with all author-declared custom
params, and `required` lists the names of custom params whose
`Required` flag is set. Each property SHALL carry `type`,
`description`, and where applicable `default`, `enum`,
`additionalProperties`. String→string maps SHALL be expressed as
`{"type": "object", "additionalProperties": {"type": "string"}}`.

#### Scenario: Schema merges built-ins and customs
- **WHEN** a pack calls `RegisterPackParams(pack.PackParam{Name: "vpc_id", Type: "string", Required: true})`
- **THEN** the manifest's `params_schema.properties` contains entries
  for both the built-ins and `vpc_id`, and `required` contains
  `"vpc_id"`

#### Scenario: Empty when SDK and author declare nothing
- **WHEN** an implementation of the SDK is built with an empty built-in
  registry and a pack does not call `RegisterPackParams`
- **THEN** `params_schema` is omitted from the manifest response

### Requirement: SDK Rewrites Sim Terraform To Reference Pack-Level Variables
At manifest build time the pack SDK SHALL, for each built-in pack
param, ensure the sim's Terraform body declares a `variable "<name>"`
block (auto-inserting one with `type` and `default` derived from the
schema if absent) and SHALL rewrite the matching provider or resource
block to reference `var.<name>` instead of a literal value. Author-
declared custom params SHALL NOT trigger TF rewriting; the SDK SHALL
leave the author's existing `variable "<name>" {}` declarations
untouched.

#### Scenario: aws_region rewritten to var reference
- **WHEN** a sim's `main.tf` contains `provider "aws" {}` with no
  `region` attribute and the SDK builds the manifest
- **THEN** the manifest's emitted Terraform contains both
  `variable "aws_region" { type = string; default = "us-east-1" }`
  and `provider "aws" { region = var.aws_region }`

#### Scenario: default_tags retains merge semantics
- **WHEN** a sim's `provider "aws" {}` already declares a
  `default_tags { tags = { existing = "value" } }` block
- **THEN** the SDK rewrites the block to
  `default_tags { tags = merge(var.default_tags, { existing = "value" }) }`
  and inserts `variable "default_tags" { type = map(string); default = { simrun_simulation_id = "<id>" } }`

#### Scenario: azure_location targets resource group
- **WHEN** a sim declares one or more `resource "azurerm_resource_group" "x" {}`
  blocks with no `location` attribute
- **THEN** the SDK rewrites each to set
  `location = var.azure_location` and inserts the corresponding
  `variable "azure_location" {}` block

#### Scenario: Custom param TF left untouched
- **WHEN** a pack declares a custom `vpc_id` param and a sim's
  `main.tf` already contains `variable "vpc_id" {}`
- **THEN** the SDK does not modify the `variable "vpc_id"` block or
  any reference to it in the TF body

#### Scenario: Built-in with no matching block is a no-op
- **WHEN** a sim's TF contains no `provider "aws" {}` block
- **THEN** the SDK does not insert a `variable "aws_region" {}` block
  for that sim and does not modify any other TF content
