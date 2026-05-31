## ADDED Requirements

### Requirement: Simulation Declares Required Terraform Outputs
The pack SDK SHALL accept an optional `RequiredOutputs []string` field on
`pack.Simulation`. The field is the explicit, declarative contract between
a simulation's `Detonate` function and the embedded Terraform: each entry
names an output that `Detonate` reads from `DetonateInput.TerraformOutputs`.
The field SHALL default to `nil`, in which case the SDK performs no
output-related validation for backward compatibility.

#### Scenario: Field defaults to nil
- **WHEN** a simulation is registered with no `RequiredOutputs`
- **THEN** registration succeeds and the SDK performs no Terraform-output
  validation for that simulation

#### Scenario: Field declares a contract
- **WHEN** a simulation registers with `RequiredOutputs: []string{"bucket_arn"}`
- **THEN** the simulation's binary self-documents that `Detonate` reads
  `DetonateInput.TerraformOutputs["bucket_arn"]`

### Requirement: Register Validates RequiredOutputs Against Embedded Terraform
The pack SDK SHALL validate `RequiredOutputs` inside `pack.Register`,
before the function returns. Validation SHALL parse the embedded
`Simulation.Terraform` HCL string, extract the labels of all top-level
`output "<name>" {}` blocks, and confirm that every entry in
`RequiredOutputs` appears in the declared set. The check SHALL run after
the existing scope, ID-format, and duplicate-registration checks.

#### Scenario: All required outputs declared
- **WHEN** a simulation has `RequiredOutputs: ["a", "b"]` and its
  Terraform body declares `output "a" {}` and `output "b" {}`
- **THEN** `Register` returns normally

#### Scenario: Missing required output panics
- **WHEN** a simulation has `RequiredOutputs: ["bucket_arn"]` and its
  Terraform body declares no `output "bucket_arn" {}` block
- **THEN** `Register` panics with a message naming the simulation, the
  missing output(s), and the set of outputs the parser actually found

#### Scenario: Extra HCL outputs are tolerated
- **WHEN** a simulation has `RequiredOutputs: ["a"]` and its Terraform
  body declares `output "a" {}` and `output "b" {}`
- **THEN** `Register` returns normally — over-declaration in HCL is
  not an error

#### Scenario: Duplicate-registration check still runs first
- **WHEN** a simulation with a duplicate ID is registered, even with a
  matching `RequiredOutputs` set
- **THEN** `Register` panics with the existing duplicate-registration
  message, not the new validation message

### Requirement: Empty Terraform With RequiredOutputs Panics
The pack SDK SHALL panic during `pack.Register` when
`Simulation.Terraform` is empty and `RequiredOutputs` is non-empty. A
simulation that declares it needs Terraform outputs but ships no
Terraform body is incoherent (simrun skips the apply lifecycle entirely
for empty-Terraform simulations) and is treated as an authoring bug.

#### Scenario: Empty TF with no required outputs is fine
- **WHEN** a simulation has `Terraform: ""` and `RequiredOutputs: nil`
- **THEN** `Register` returns normally

#### Scenario: Empty TF with required outputs panics
- **WHEN** a simulation has `Terraform: ""` and
  `RequiredOutputs: ["bucket_arn"]`
- **THEN** `Register` panics with a message stating the simulation
  declared required outputs but supplied no Terraform body

### Requirement: HCL Parse Errors Surface as Register Panics
The pack SDK SHALL panic during `pack.Register` if the embedded
`Simulation.Terraform` string fails to parse as HCL. The panic message
SHALL include the simulation ID and the parser's diagnostic summary so
the author can locate the syntax error without running `terraform
validate`.

#### Scenario: Unparseable HCL
- **WHEN** a simulation's `Terraform` body contains invalid HCL syntax
- **THEN** `Register` panics with a message naming the simulation and
  including the HCL parser's diagnostic message

### Requirement: Validation Runs On Every Pack Binary Boot
The pack SDK SHALL perform `RequiredOutputs` validation on every entry
path of a pack binary that triggers `init()` — including but not
limited to `simrun manifest`, `simrun detonate`, `simrun cleanup`,
`go test`, and any `--help`-style invocation. Failures SHALL be
deterministic and SHALL NOT depend on which command is dispatched.

#### Scenario: Manifest command fails fast
- **WHEN** a pack binary with a broken `RequiredOutputs` declaration is
  invoked with the `manifest` command
- **THEN** the binary panics during `init()` and exits non-zero before
  reading any input from stdin

#### Scenario: Test build fails fast
- **WHEN** a `go test` run imports the pack module and triggers
  registration of a simulation with a broken `RequiredOutputs`
  declaration
- **THEN** the test binary fails during package init, surfacing the
  validation error in CI without requiring a real `terraform apply`
