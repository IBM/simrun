## 1. SDK Field

- [x] 1.1 Add `RequiredOutputs []string` to `pack.Simulation` in
  `simrun/pack/types.go` with a doc comment that names
  `DetonateInput.TerraformOutputs` so callers can find the symmetric
  side via grep.
- [x] 1.2 Confirm `json:"-"` (or omit-empty) on the new field —
  `Simulation` is not serialized over the wire, but keep the JSON tags
  consistent with the rest of the struct.

## 2. HCL Output Extraction

- [x] 2.1 Create `simrun/pack/tfparse.go` (new file) with an unexported
  `extractDeclaredOutputs(hclBody string, source string) (declared
  []string, err error)` that uses `github.com/hashicorp/hcl/v2/hclparse`
  to parse and walks the body for blocks of type `"output"` with one
  label.
- [x] 2.2 Return the parser diagnostic as an error wrapped with the
  `source` argument so callers can produce a descriptive panic.
- [x] 2.3 Treat empty input as `(nil, nil)` (caller decides how to
  react).

## 3. Register-Time Validation

- [x] 3.1 In `simrun/pack/pack.go`, add an unexported
  `validateSimulation(s *Simulation, fullID string)` invoked from
  `Register` after the existing scope/ID/duplicate checks.
- [x] 3.2 If `len(s.RequiredOutputs) == 0`, return immediately.
- [x] 3.3 If `s.Terraform == ""`, panic with
  `simulation %q: declares RequiredOutputs %v but has no Terraform body`.
- [x] 3.4 Otherwise call `extractDeclaredOutputs`; on parser error,
  panic with `simulation %q: failed to parse embedded Terraform: %v`.
- [x] 3.5 Compute `missing = RequiredOutputs - declared`. If non-empty,
  panic with `simulation %q: missing terraform outputs %v (declared in
  HCL: %v)`.
- [x] 3.6 Wire the call into `Register` in
  `simrun/pack/pack.go:41-43` so it runs before `Register` returns and
  before `registerItem` stores the simulation.

## 4. Tests

- [x] 4.1 In `simrun/pack/tfparse_test.go`, cover
  `extractDeclaredOutputs` for: well-formed multi-output HCL,
  commented-out outputs (must not be reported), heredoc strings
  containing the word `output`, an empty string, and invalid HCL
  (returns error).
- [x] 4.2 In `simrun/pack/pack_test.go` (or extend
  `protocol_test.go`), add tests that call `Register` with
  representative `Simulation` literals using a small embedded HCL
  fixture string, asserting:
  - happy path returns normally;
  - missing output panics with a message containing the missing key
    and the declared set;
  - empty `Terraform` + non-empty `RequiredOutputs` panics with the
    expected message;
  - unparseable HCL panics with the expected wrapping.
- [x] 4.3 Reset the package-level `registry` between test cases so
  duplicate-registration panics from prior cases do not mask the
  validation panics under test.

## 5. Example & Internal Pack Migration

- [x] 5.1 Audit `simrun/pack/aws/`, `simrun/pack/azure/`, and
  `simrun/pack/gcp/` (and any other simulation registrations under
  `simrun/pack/...`) for `Register` calls that bundle Terraform with
  output-dependent `Detonate` logic; populate `RequiredOutputs` for
  each. **Result:** these subpackages are SDK helpers only — no
  `pack.Register` callers exist anywhere in this repo (real simulation
  packs are external Go modules).
- [x] 5.2 Add at least one example simulation in tests that
  demonstrates the pattern, so future authors have a working reference.
  Covered by `TestRegister_RequiredOutputsHappyPath` in
  `simrun/pack/pack_test.go`.
- [x] 5.3 Confirm `go test ./simrun/pack/...` passes after the audit;
  any failure means the audited pack had a previously hidden contract
  bug — fix the HCL or the `RequiredOutputs` declaration as
  appropriate.

## 6. Verification

- [x] 6.1 Run `go vet ./...` and `go test ./...` from the repo root.
- [x] 6.2 Build the simrun binary via `mise run build` and confirm
  startup is unaffected (simrun does not call `pack.Register`; the
  check lives in pack binaries).
- [x] 6.3 Manually invoke a representative pack binary with `manifest`
  and confirm `init()` panics deterministically when a fixture
  declares a missing output, and exits non-zero with a clear stderr
  message before reading stdin. **Result:** no pack binary exists in
  this repo (real packs are external Go modules). The
  `TestRegister_*` cases in `simrun/pack/pack_test.go` exercise the
  same `Register` code path that pack binaries hit during `init()`,
  so the panic format and trigger condition are covered. External
  pack authors will see the same behavior on their next build.
