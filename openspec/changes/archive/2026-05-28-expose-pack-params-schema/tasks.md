## 1. SDK — authoring API and built-in registry

- [x] 1.1 Add `PackParam` struct to `simrun/pack/types.go` with `Name`, `Type`, `Description`, `Default`, `Required`, `Enum` fields. Type constants: `"string"`, `"boolean"`, `"object_string_map"`.
- [x] 1.2 Add an unexported package-level slice holding registered custom params, and the `pack.RegisterPackParams(...PackParam)` function that appends to it.
- [x] 1.3 Implement call-time validation: reserved-built-in collision, duplicate name, default-vs-type mismatch, enum-on-non-string. Panic with messages naming the offending param and rule.
- [x] 1.4 Create `simrun/pack/builtins.go` containing the built-in registry (`default_tags`, `aws_region`, `gcp_region`, `azure_location`) with schema entries (type, description, default, enum where applicable) and a per-built-in TF rewriting hook. (`gcp_project` is intentionally excluded — see proposal.)

## 2. SDK — schema in manifest

- [x] 2.1 Add `ParamsSchema json.RawMessage` to `pack.ManifestResponse` in `simrun/pack/protocol.go`.
- [x] 2.2 Implement `buildPackParamsSchema()` in `simrun/pack/protocol.go` that merges the built-in registry with the registered custom params into the JSON-Schema-shaped doc.
- [x] 2.3 Wire `buildPackParamsSchema()` into `buildManifest(...)` so every manifest response carries the merged schema; omit field when empty.

## 3. SDK — var-based TF rewriting

- [x] 3.1 Refactor `simrun/pack/tags.go` rewriting to operate via "ensure `variable "X"` block + reference `var.X`" for `default_tags`. Existing merge semantics become `merge(var.default_tags, existing)`.
- [x] 3.2 Extend the rewriting pass to handle `aws_region` (inject into `provider "aws" {}`), `gcp_region` (inject into `provider "google" {}`), and `azure_location` (inject into every `resource "azurerm_resource_group" {}`).
- [x] 3.3 Ensure all rewriting is no-op when the matching provider/resource block is absent (built-ins remain safe for packs that don't use that cloud).
- [x] 3.4 Update `simrun/pack/tags_test.go` (or write `builtins_test.go`) covering: variable block inserted, provider/resource block rewritten to `var.<name>` reference, default_tags merge with existing tags, multi-block providers, GCP and Azure cases, and the no-op-when-block-absent path.
- [x] 3.5 Update `simrun/pack/pack_test.go` and `simrun/pack/tfparse_test.go` if they cover manifest-emitted TF.

## 4. Runner — pack-level `-var` passthrough

- [x] 4.1 In `simrun/internal/packs/runner/` (and detonator code that builds the `terraform apply` invocation), thread the pack-level `parameters` map alongside per-sim params into the `-var`/`TF_VAR_*` set.
- [x] 4.2 Apply precedence: TF variable default < pack-level value < per-sim scenario value. Implement via env-var ordering or `-var` flag ordering, whichever the existing path uses.
- [x] 4.3 Pass all keys from `packs.parameters` (even those not in `params_schema`) so legacy values continue to flow.
- [x] 4.4 Add a runner-level unit test asserting precedence: when both pack and per-sim set `aws_region`, the per-sim value reaches terraform.

## 5. Backend — parameter validation endpoint

- [x] 5.1 In `simrun/internal/web/packs_handler.go`, fetch the pack manifest (or read a cached `params_schema`) at `PUT /api/packs/{name}/parameters` time and validate the request body: type, enum, required against declared keys.
- [x] 5.2 On validation failure, return HTTP 400 with a structured body naming the offending key(s) and rule(s) violated.
- [x] 5.3 On success, persist the full request body verbatim (declared keys + unknown keys) and return a response containing both `parameters` and `unknown_keys` (the list of saved keys not in the schema).
- [x] 5.4 If the manifest fetch fails or `params_schema` is empty, fall back to permissive storage (today's behavior); unknown_keys lists every key.
- [x] 5.5 Add HTTP-level tests for type mismatch, enum violation, missing required, unknown-key passthrough, and the permissive fallback.

## 6. Frontend — shared SchemaForm component

- [x] 6.1 Create `web/frontend/src/lib/components/SchemaForm.svelte` accepting `schema`, `values`, `onchange`, and rendering one field per `properties` entry.
- [x] 6.2 Implement renderers: `string` → `<Input>`; `string` with `enum` → `<Select>`; `boolean` → `<Switch>`; `object` with `additionalProperties.type === "string"` → key/value editor (factor out the existing `PackParametersDialog` repeater).
- [x] 6.3 Show `description` as a tooltip on the label, `default` as input placeholder, mark required fields, and group built-in cloud params under a collapsible "Cloud defaults" section (use a heuristic: collapse closed by default, open if any built-in has a value).
- [x] 6.4 Render unknown saved keys below the form with a warning callout.

## 7. Frontend — adopt SchemaForm

- [x] 7.1 Rewrite `web/frontend/src/lib/components/PackParametersDialog.svelte` to fetch the pack manifest, extract `params_schema`, and render via `SchemaForm`. Submit triggers `PUT /api/packs/{name}/parameters` and displays per-field validation errors from the structured 400 response.
- [ ] 7.2 Replace the simulation-param rendering block in `web/frontend/src/lib/components/ScenarioSection.svelte` with `SchemaForm` (sharing the same component). **DEFERRED**: the scenario editor's per-sim params use a `FormParam[]` array model tightly coupled to YAML generation in `yaml-generator.ts`; swapping the renderer requires migrating the model to `Record<string, any>` throughout the scenario form and YAML output paths. Tracked for a follow-up.
- [ ] 7.3 Update the simulation-detail views (`SimulationDetailSheet.svelte`, `PackSimulationsSheet.svelte`) to render via `SchemaForm` for consistency. **DEFERRED**: these are read-only JSON-pre displays that already work. `SchemaForm` is interactive; a read-only mode would have to be added before adoption. Tracked for a follow-up.
- [x] 7.4 Update the API client in `web/frontend/src/lib/api/client.ts` so `updatePackParameters` returns the `{parameters, unknown_keys}` body shape.

## 8. Integration and rollout

- [ ] 8.1 End-to-end test (or scripted smoke check): install a pack, set `aws_region` via the API, run a scenario, confirm `terraform plan` shows the var reference and the region applies. **DEFERRED**: requires a real pack binary that runs terraform; behaviors covered at the layer-level (SDK rewriting in `builtins_test.go` asserts `region = var.aws_region`; runner-level `simrun_detonator_test.go` asserts `TF_VAR_aws_region` flows through). Manual smoke verification needed before release.
- [ ] 8.2 End-to-end test of per-sim override: scenario YAML sets `aws_region` for one sim; assert that sim's `terraform apply` receives the override while a peer sim sees the pack-level value. **DEFERRED** with the same reason as 8.1; precedence asserted directly in `TestTerraformEnvVars_PerSimOverridesPackLevel`.
- [x] 8.3 Update `CLAUDE.md` and any pack-author docs to describe `RegisterPackParams`, the built-in catalog, the var-based wiring, and the per-sim override behavior.
- [x] 8.4 Run `openspec validate expose-pack-params-schema --strict` and address any reported issues.

## 9. Follow-up (optional, separate change)

- [ ] 9.1 Migrate `simrun-base-pack` to adopt `RegisterPackParams` for real custom params (`resource_prefix`, `vpc_id`, `subnet_id`, `security_group_id`) and add matching `variable "X" {}` blocks to the AWS sims that hard-code those values today.
