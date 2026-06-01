## Context

The pack SDK at `simrun/pack/` is consumed by external Go modules
(simulation packs) that compile to standalone binaries. Each binary
calls `pack.Register(Simulation{...})` from `init()` for every
simulation it ships, then `pack.Run()` from `main()` to dispatch
`manifest`/`detonate`/`cleanup`.

`Simulation.Terraform` is a `string` of raw HCL, almost always loaded
via `//go:embed`. simrun runs `terraform apply` on that HCL, parses
`terraform.tfstate`, and passes the resulting outputs to `Detonate` as
`DetonateInput.TerraformOutputs map[string]string`. Today nothing ties
the keys a `Detonate` reads from that map to the `output` blocks the
embedded HCL declares. A typo or stale rename produces a zero-value
string and a silent runtime failure that only surfaces after a real
cloud apply.

`github.com/hashicorp/hcl/v2 v2.24.0` is already a direct dependency
(see `go.mod:24`), so robust HCL parsing is free.

## Goals / Non-Goals

**Goals:**
- Make the sim ↔ TF output contract explicit and self-documenting on
  the `Simulation` struct.
- Fail every pack-binary boot when the declared `RequiredOutputs` are
  not all present in the embedded HCL's `output` blocks — including
  the `manifest` command, which simrun calls before any cloud apply.
- Zero cost for existing simulations that opt out (nil
  `RequiredOutputs` skips the check).
- Use a robust HCL parse, not a regex, so commented-out `output`
  blocks, heredocs, and string literals containing `output "..."` do
  not produce false positives or negatives.

**Non-Goals:**
- Do not validate the *types* of declared outputs (string vs. number
  vs. object). The wire protocol flattens everything to
  `map[string]string`; richer typing belongs in a separate change.
- Do not validate that a pack's `Detonate` actually reads every
  declared `RequiredOutputs` entry — over-declaration is harmless.
- Do not propagate this contract to simrun's runtime; simrun continues
  to ship whatever outputs `terraform.tfstate` produces. The check is
  pack-author-facing only.
- Do not validate `variable` blocks against a declared `RequiredVars`
  field; out of scope for this change.
- Do not change the wire protocol or the on-disk format of installed
  pack binaries.

## Decisions

### Decision 1: Validate at `Register()` time, not lazily
**Choice:** run the parse + diff inside `pack.Register` and `panic` on
mismatch, before the function returns.

**Rationale:** every binary entry path (`simrun manifest`, `simrun
detonate`, `simrun cleanup`, `simrun list`, `go test`, `go run`)
executes `init()` and therefore `Register`. A panic in `init()` aborts
the binary before `main()` runs, so even `--help`-style smoke tests
will fail. This is the strongest possible feedback loop short of a
build-time check, with no extra invocation surface.

**Alternatives considered:**
- *Validate at `Run()` start:* misses authors who only run unit tests
  or `--help`. Rejected.
- *Validate when `manifest` is called:* still cheap (no apply), but
  silent for binaries that never have manifest invoked in CI.
  Rejected — authors deserve faster feedback.
- *Build-time check via `go vet`/linter:* much higher implementation
  cost, requires authors to run an extra tool. Rejected; revisit if
  the runtime panic proves too late.

### Decision 2: Use `hashicorp/hcl/v2` for parsing, not regex
**Choice:** parse `Simulation.Terraform` with `hclparse.NewParser().
ParseHCL([]byte(s.Terraform), "<simulation>.tf")`, then walk the body
schema for blocks of type `"output"` with one label.

**Rationale:** `hcl/v2` is already a direct go.mod dep (`v2.24.0`).
Regex against HCL is fragile against `# output "x" {}` comments,
heredocs (`<<EOT ... EOT`), strings containing the word `output`, and
nested-block formatting variants. Using the official parser eliminates
all of those edge cases.

**Alternatives considered:**
- *Regex `(?m)^output\s+"(\w+)"`:* light and dependency-free, but
  fails on commented or multi-line declarations and on whitespace
  variations. Rejected.
- *`hashicorp/terraform-config-inspect`:* heavier dep, designed for
  module-directory input rather than a single in-memory string.
  Rejected.

### Decision 3: Panic with a structured message, not return error
**Choice:** `panic(fmt.Sprintf("simulation %q: missing terraform
outputs %v (declared in HCL: %v)", fullID, missing, declared))`.

**Rationale:** `Register` already panics on duplicate registrations,
empty scope, and dotted IDs (see `simrun/pack/pack.go:55-66`). Keeping
the failure mode consistent avoids forcing every pack author to thread
a return value through `init()`. The message lists the missing keys
*and* what the parser found so the author can immediately see typos.

**Alternative:** return an error and have authors check it. Adds
ceremony to every `init()` and breaks symmetry with the existing
`Register` panics. Rejected.

### Decision 4: HCL parse errors are panics, too
**Choice:** if `ParseHCL` returns diagnostics, panic with a wrapped
message naming the simulation and the diagnostic summary.

**Rationale:** unparseable HCL is a strictly worse problem than a
missing output — the binary cannot run apply at all. Surfacing it at
`init()` time is consistent with the spirit of the change.

### Decision 5: Empty `Terraform` + non-empty `RequiredOutputs` is a panic
**Choice:** if `s.Terraform == ""` and `len(s.RequiredOutputs) > 0`,
panic with a message stating the simulation declared required outputs
but has no terraform.

**Rationale:** simrun's `pack-execution` spec already states that
simulations with empty `terraform` skip the apply lifecycle entirely
(see `pack-execution` requirement "Conditional Terraform Lifecycle").
Declaring `RequiredOutputs` against no HCL is incoherent and almost
certainly a mistake.

### Decision 6: Order of validation in `Register`
**Choice:** the new check runs *after* the existing scope/ID/duplicate
checks (i.e., last). The new check is more expensive (full HCL parse)
than the cheap string checks, and it is meaningless if the basic
identity checks already failed.

## Risks / Trade-offs

- **[Risk]** A pack ships `RequiredOutputs: nil` and continues to
  silently fail at runtime. → **Mitigation:** opt-in by design; this
  change does not break backward compatibility. Document the field in
  the pack SDK guidance and update example packs so new authors see
  it; consider a separate follow-up to make the field required after
  internal packs migrate.
- **[Risk]** HCL with dynamic / generated `output` blocks (e.g., from
  templates evaluated at apply time). → **Mitigation:** the SDK only
  validates statically declared `output "<name>"` blocks. Authors with
  dynamic outputs can simply omit the corresponding entries from
  `RequiredOutputs`; the validation only checks declared-vs-required.
  This is consistent with simrun's own static parse of the same
  embedded string for the wire protocol.
- **[Risk]** False positives if an author intentionally reads an
  output that exists in `tfstate` but is not declared in the HCL
  source (e.g., from a `terraform_remote_state` reference). →
  **Mitigation:** that pattern is not used today and would be a
  deliberate stretch of the SDK; if it ever lands, this validation
  becomes opt-out per simulation by leaving `RequiredOutputs` nil.
- **[Risk]** Tests that import the pack SDK now panic during `init()`
  if an example simulation is wrong. → **Mitigation:** desired
  behavior — that is the whole point. Existing tests in
  `simrun/pack/` that construct `Simulation` literals and call
  `Register` must be audited (see tasks).
- **[Trade-off]** Pack registration becomes ~ms slower per simulation
  (one HCL parse). → Acceptable: registrations happen once per binary
  start, count tens at most, and the parse runs on tiny `<1KB`-`<10KB`
  HCL bodies.

## Migration Plan

1. Land the SDK change with `RequiredOutputs` defaulting to nil so all
   existing simulations are unaffected.
2. Update example/test simulations in `simrun/pack/` to declare
   `RequiredOutputs` where applicable; this both verifies the check
   and provides a copy-paste reference for pack authors.
3. Communicate the new field via the pack SDK guidance (existing docs
   path; specifics deferred to tasks.md).
4. Internal packs migrate at their own cadence; we may revisit
   making the field required in a follow-up change once the
   ecosystem has converged.

**Rollback:** revert the SDK change. Since the field is additive and
the validation is internal to `Register`, no downstream binary needs
to be rebuilt to drop the check (though packs that opted in may want
to keep declaring the field for documentation value).

## Open Questions

- Should the check tolerate `count`/`for_each`-driven outputs whose
  literal block label is present in the source (e.g., `output
  "instance_ids" { value = aws_instance.x[*].id }`)? Static analysis
  sees the label, so yes — covered by Decision 2.
- Do we want a future `RequiredVars []string` companion that validates
  declared `variable` blocks against expected `TF_VAR_*` injection?
  Out of scope; track separately if there's appetite.
