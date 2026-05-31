## Why

Pack-level parameters today are an untyped `map[string]any` with no
declared schema. The web UI surfaces them as a generic key/value editor
that requires the operator to already know which keys a pack accepts —
including the one key the SDK currently honors (`default_tags`). Pack
authors have no way to advertise additional parameters, so cross-cutting
knobs like an AWS region or a resource-name prefix can't be exposed to
operators without forking the pack or hand-editing Terraform. Making
pack params discoverable unblocks a typed setup form and a uniform
mechanism for pack-wide configuration that scenarios can override.

## What Changes

- **NEW** SDK function `pack.RegisterPackParams(...pack.PackParam)` so pack
  authors declare custom params with name/type/description/default/required/enum.
- **NEW** built-in pack params provided by the SDK unconditionally:
  `default_tags` (existing), `aws_region`, `gcp_region`,
  `azure_location`. Built-ins are always-on; injection is a no-op when
  the matching provider/resource block is absent. `gcp_project` is
  intentionally not a built-in — GCP project IDs are organization-
  specific and have no useful SDK default; packs that need it should
  declare it via `RegisterPackParams`.
- **NEW** `params_schema` field on `pack.ManifestResponse`, a JSON-Schema-
  shaped document covering the merged set of built-in + custom params.
  Shape mirrors `Simulation.ParamsSchema`.
- **BREAKING (internal)** SDK manifest-time TF rewriting changes from
  literal-baked values to Terraform-variable references. The SDK ensures
  each sim's TF has `variable "X" {}` declarations for built-ins it
  injects, and rewrites provider/resource blocks to reference `var.X`.
  Functional behavior is preserved; `terraform plan` output is observably
  different (e.g., `region = var.aws_region` rather than
  `region = "us-east-1"`). The merge semantics for `default_tags` are
  preserved as `merge(var.default_tags, existing)`.
- **NEW** pack-level param values flow into every sim's
  `terraform apply` as `-var` flags. Precedence (later wins):
  TF `variable` default → pack-level value → per-sim scenario value.
- **MODIFIED** `PUT /api/packs/{name}/parameters` strict-validates
  declared keys against the pack's schema (type, required, enum
  membership). Unknown keys are kept, not rejected, and the API echoes
  them back so the UI can surface a soft warning.
- **MODIFIED** Frontend pack-params dialog renders a typed form from
  the schema. The simulation-params renderer is folded into a shared
  `SchemaForm` component used by both.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `pack-sdk`: adds `RegisterPackParams` authoring API, the registry of
  built-in pack params, the `params_schema` manifest field, and the
  manifest-time rewriting rules that wire pack params through Terraform
  variables.
- `packs`: tightens parameter validation against the declared schema
  while preserving previously stored keys via a soft-warning model.
- `pack-execution`: defines how pack-level param values are layered into
  per-sim `terraform apply` invocations alongside per-sim params, and
  the precedence rule when both scopes define the same key.

## Impact

- `simrun/pack/`: new authoring API, new built-in registry, new manifest
  field, TF rewriting moves to var-based wiring (`pack/protocol.go`,
  `pack/tags.go`, new file for built-ins).
- `simrun/internal/packs/runner/` and the simrun detonator path: pass
  pack-level params as `-var` flags layered before per-sim params.
- `simrun/internal/web/packs_handler.go`: schema-aware validation on
  the parameters update endpoint; surface unknown-key warnings.
- `simrun/internal/db/`: no schema changes; `packs.parameters` JSONB
  column keeps its current shape.
- `web/frontend/`: upgrade `PackParametersDialog.svelte`, extract a
  shared `SchemaForm` used by pack-params and simulation-params, group
  built-ins under a "Cloud defaults" section, render unknown saved
  keys with a warning.
- `simrun-base-pack` and any other shipped pack repos: opportunity to
  adopt `RegisterPackParams` for custom params and declare matching
  `variable "X" {}` blocks. Not required for the change to ship — packs
  that don't migrate keep working with built-ins only.
