## Why

Today, a `pack.Simulation`'s `Detonate` function reads from `input.TerraformOutputs`
purely by string key. If the embedded Terraform body forgets to declare the
matching `output "x" { ... }` block, the map silently returns the zero value
and the simulation fails downstream — only after a real `terraform apply`
against a live cloud account, costing time and (in the case of EC2/S3/GCS
resources) real money. The sim ↔ TF contract is invisible: nothing in the
codebase, IDE, or CI tells the author which outputs their detonator depends
on, and refactoring the TF in one direction without the Go in the other is
a silent footgun.

## What Changes

- Add `RequiredOutputs []string` to `pack.Simulation` so each simulation
  declaratively states which Terraform outputs its `Detonate` reads from.
- At `pack.Register()` time, parse the embedded `Terraform` HCL string,
  extract all top-level `output "<name>" {}` block labels, and panic if
  any `RequiredOutputs` entry is missing from the declared set.
- Apply the same check when `Terraform` is empty: a non-empty
  `RequiredOutputs` against an empty Terraform string is a panic.
- Surface the check on every code path that boots the pack binary —
  `init()` registration runs unconditionally, so `simrun manifest`,
  `detonate`, `cleanup`, smoke tests, and CI builds all fail loudly and
  deterministically with no cloud spend.
- Document the field in the pack SDK guidance and update the example
  pack(s) so authors copy the new pattern.

## Capabilities

### New Capabilities
- `pack-sdk`: Documents the contract pack authors interact with via
  `simrun/pack/` — `Register`, the `Simulation` struct shape, and the
  validations the SDK performs at registration time before any pack
  command (`manifest`, `detonate`, `cleanup`) is dispatched.

### Modified Capabilities
<!-- None. `pack-execution` describes runtime behavior of installed pack
     binaries from simrun's perspective; this change is internal to the
     pack SDK and does not alter the wire protocol or runtime contract. -->

## Impact

- **Code**: `simrun/pack/types.go` (new field), `simrun/pack/pack.go`
  (`Register` validation hook), new `simrun/pack/tfparse.go` (or
  equivalent) implementing minimal HCL `output` block extraction, and
  tests in `simrun/pack/`.
- **Dependencies**: prefer `github.com/hashicorp/hcl/v2` (already a
  transitive dep of the terraform tooling used elsewhere) for a robust
  parse; fall back to a regex-based extractor if introducing the
  dependency is unwanted. Decision deferred to design.md.
- **Pack authors**: existing simulations continue to work — `RequiredOutputs`
  defaults to nil and skips the check. Authors opting in get a hard
  guarantee at boot time. No wire-protocol changes.
- **simrun runtime**: unchanged. The simrun binary still passes
  `TerraformOutputs` over stdin per the existing `pack-execution` spec;
  the validation runs inside the pack process, before the pack ever
  reads from `os.Stdin`.
- **CI / smoke tests**: any pack with a broken declaration will fail
  every command (including `manifest`) immediately, blocking merges.
