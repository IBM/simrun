## Context

Pack-level parameters today live in three loosely connected places:

- `db.Pack.Parameters` (JSONB on the `packs` table) — operator-supplied
  values, untyped.
- `pack.ManifestInput.Parameters` — the same map handed to the pack's
  `manifest` stdin command.
- The SDK's `extractDefaultTags(parameters)` in `simrun/pack/pack.go` —
  the only place in the SDK that actually consumes a pack-level key.

The frontend `PackParametersDialog.svelte` exposes the map as a generic
key/value editor. Pack authors have no SDK affordance for declaring
additional params; whatever they expect, operators have to type
verbatim.

This contrasts with per-simulation params, which already carry a JSON
Schema (`Simulation.ParamsSchema`, also fall-back-extracted from TF
`variable` blocks via `extractTerraformVarsSchema`) and are rendered as
a per-field form in the scenario editor. The intent of this change is
to bring pack-level params up to the same model and use the same
plumbing where possible.

The relevant surface area:

- `simrun/pack/protocol.go` — manifest types, fallback schema extraction.
- `simrun/pack/tags.go` — current literal-baked HCL rewriting for
  `default_tags`.
- `simrun/pack/pack.go` — `handleManifest` stdin entry, where new
  parameter handling will hook in.
- `simrun/internal/packs/runner/` and the detonator path — where `-var`
  flags are assembled for `terraform apply`.
- `simrun/internal/web/packs_handler.go` — the parameters HTTP endpoint.
- `web/frontend/src/lib/components/PackParametersDialog.svelte` and
  `ScenarioSection.svelte` — the two existing schema renderers, both
  string-only.

## Goals / Non-Goals

**Goals:**

- Pack binaries advertise the full set of pack-level params they accept
  through `pack.ManifestResponse.ParamsSchema`. Built-ins are included
  unconditionally; custom ones are author-declared via a new SDK API.
- Frontend renders a typed form (string / boolean / enum / string→string
  map) for pack params, sharing a `SchemaForm` component with the
  simulation-params renderer.
- Pack params flow as Terraform variables to every sim's
  `terraform apply`, enabling per-sim override from scenario YAML
  without any per-sim SDK changes.
- Backend validation tightens for declared keys while remaining
  permissive for unknown ones, so existing packs keep working through
  the rollout.

**Non-Goals:**

- Per-run param overrides at the API layer. Workaround: install the
  pack a second time under a different name with a different parameter
  set. Revisit if real demand emerges.
- Dynamic schemas (a function returning a schema). Schema is computed
  once at pack `Register`/`init` time.
- Array, nested-object, `oneOf`/`anyOf`, integer, or number types in
  v1. Strings, booleans, enums, and the specific `object<string,string>`
  shape (for `default_tags` and similar) are the only types supported.

## Decisions

### D1: Where author-declared schemas live

**Decision:** A new package-level function
`pack.RegisterPackParams(...pack.PackParam)` in `simrun/pack/`. Pack
authors call it from `main()` next to `pack.SetPackInfo(...)`. The
function appends into an SDK-internal slice, validates immediately, and
panics on author bugs the same way `pack.Register(Simulation{})` does
today.

**Alternatives considered:**

- *Extend `pack.SetPackInfo` to take a `ParamsSchema` field.* Rejected
  because it overloads what `SetPackInfo` means (currently identity-only)
  and forces every author to think about params even when they have
  none.
- *A `pack.RegisterPackParam(spec)` per param.* Rejected: the variadic
  form lets us validate name uniqueness across the whole declaration in
  one shot, and reads better with mixed shapes (built-ins, customs).
- *Per-simulation pack-param hints.* Rejected: pack-level params are by
  definition pack-wide, and per-sim params already have their own
  channel.

### D2: Built-ins are always-on

**Decision:** The SDK exposes its built-in params (`default_tags`,
`aws_region`, `gcp_region`, `azure_location`) unconditionally in every
pack's schema. There is no `BuiltinParam` opt-in type. Injection logic
is a no-op on packs whose TF lacks the matching provider/resource
blocks, so always-on is safe. `gcp_project` is intentionally NOT a
built-in: project IDs are org-specific and have no useful default; the
pack-author who needs it declares it via `RegisterPackParams`.

**Rationale:** Opt-in adds SDK surface, more chances for the author to
forget a built-in their pack actually needs, and a less consistent
experience across packs. The cost — a GCP-only pack's form lists
`aws_region` — is mitigated by grouping built-ins under a "Cloud
defaults" UI section.

**Alternative considered:** explicit per-built-in opt-in via
`pack.BuiltinParam{Name: "aws_region"}`. Rejected as discussed above.

### D3: TF-var-based wiring (not literal-baked)

**Decision:** At manifest build time, for every pack-level param the
SDK ensures the sim's TF has a `variable "X" {}` block and rewrites
provider/resource references to use `var.X` rather than a literal value.
For built-ins, the SDK both adds the variable block and rewrites the
provider/resource block (extending today's `injectDefaultTags` walk).
For custom params, the author has already declared `variable "X" {}` in
their `.tf`; the SDK doesn't modify the TF and just passes the value
through.

Pack-level configured values flow at apply time as `-var` flags, layered
after TF's `default = ...` and before any per-sim scenario values:

```
TF variable default  <  pack-level param value  <  per-sim scenario value
```

This unifies built-in and custom injection under one mechanism and gets
per-sim override "for free" — scenarios can override `aws_region` for
one specific simulation without any new SDK code.

**Alternatives considered:**

- *Keep literal-baked injection for built-ins; only customs flow as
  `-var`.* Rejected because it splits the mental model in two and
  prevents per-sim override of built-ins (which the user specifically
  called out as a desirable feature).
- *Stop rewriting TF entirely; ask authors to declare `variable {}`
  blocks for every built-in.* Rejected — defeats the point of built-ins
  being SDK-owned with zero author effort.

**Trade-off:** `terraform plan` output now shows `region = var.aws_region`
instead of `region = "us-east-1"`. Functionally equivalent; observably
different in logs and plans.

### D4: Schema shape — JSON Schema in ManifestResponse

**Decision:** Add `ParamsSchema json.RawMessage` to
`pack.ManifestResponse`. Same JSON-Schema-shape already used by
`Simulation.ParamsSchema`:

```json
{
  "properties": {
    "aws_region": {
      "type": "string",
      "description": "...",
      "default": "us-east-1",
      "enum": ["us-east-1", "us-west-2", "..."]
    },
    "default_tags": {
      "type": "object",
      "additionalProperties": {"type": "string"},
      "description": "..."
    },
    "resource_prefix": {
      "type": "string",
      "description": "...",
      "default": "simrun"
    }
  },
  "required": ["vpc_id"]
}
```

`object<string,string>` is expressed as JSON Schema's
`{type: "object", additionalProperties: {type: "string"}}`, which the
renderer special-cases as a key/value editor.

**Alternative considered:** a typed Go `ParamSpec` struct with explicit
`StringMap`, `String`, `Bool` variants. Rejected: introduces a parallel
type system with no clear benefit, and would force the frontend to
maintain a second renderer alongside the existing `Simulation.ParamsSchema`
one.

**Alternative considered:** a separate SDK sub-command `params-schema`
alongside `manifest`/`detonate`/`cleanup`. Rejected: the schema is
cheap, computing it during `manifest` adds no measurable cost, and a
new sub-command would expand the runner/factory layer for no win.

### D5: Validation behavior — strict on declared, soft on unknown

**Decision:** `PUT /api/packs/{name}/parameters` validates declared
keys against the schema (type, required, enum membership) and rejects
the request with a structured error on mismatch. Unknown keys are kept,
not rejected. The response includes both the saved declared values and
the list of unknown-but-kept keys so the UI can surface a soft warning
("this pack received params it doesn't declare").

No SDK-side validation of `ManifestInput.Parameters` — the backend has
already filtered, and authors can validate themselves if they want
defense in depth.

**Rationale:** Strict on declared keys catches real bugs
(`defaultTags` typo for `default_tags`). Soft on unknown keys means
existing packs that haven't migrated keep working and saved values
don't get silently dropped when a pack later starts declaring a schema.

**Alternative considered:** reject unknown keys outright once a pack
has a non-empty schema. Rejected — too aggressive during rollout; we
can revisit once the ecosystem has adopted the new authoring API.

### D6: Frontend — extract a shared SchemaForm

**Decision:** Upgrade `PackParametersDialog.svelte` in place (same
entry point from `PackCard.svelte`, same dialog shell, schema-driven
contents). Extract a new shared `SchemaForm` Svelte component used by
both the pack-params dialog and the scenario-editor's
simulation-params section. The existing simulation-param renderer in
`ScenarioSection.svelte` is string-only; folding it into the shared
component is the targeted cleanup of adjacent code that's already
in our way.

Built-ins are grouped into a "Cloud defaults" collapsible section,
expanded by default if any built-in has a saved value. Custom params
render above it. Unknown saved keys render below with a warning badge.

**Alternative considered:** keep two separate renderers, only build a
new one for pack params. Rejected: we'd be writing the same renderer
twice and accepting permanent drift between the two pages.

## Risks / Trade-offs

- **[Risk] TF plans become noisier** with `var.X` references in places
  where literals used to be. → Mitigation: this is purely cosmetic in
  Terraform output; behavior is preserved. The change is mentioned
  in release notes so operators reviewing plans aren't surprised.

- **[Risk] The TF rewriting code (today in `tags.go`) gains
  responsibility for emitting `variable` blocks and references for
  multiple built-ins, not just tags.** → Mitigation: factor the HCL
  manipulation into a small, well-tested builder per built-in. Add
  table-driven tests covering provider blocks that already declare the
  attribute, providers spread across multiple blocks, and the GCP /
  Azure cases that have specific resource-type targets.

- **[Risk] Default-tag merge semantics change from
  `merge(<map literal>, existing)` to
  `merge(var.default_tags, existing)`.** → Mitigation: keep behavior
  equivalent. The variable's `default = {...}` carries the same value
  the literal map used to; the merge order with any
  resource-author-supplied tags is unchanged.

- **[Risk] Pack-level `-var` plumbing may collide with per-sim TF vars
  on name.** → Mitigation: explicit precedence rule — per-sim wins.
  Document in `pack-execution` spec and test with a sim that overrides
  `aws_region` for one scenario.

- **[Risk] Existing packs in the wild may have stored parameters that
  don't match the new built-ins' shape (e.g., a pack with a custom
  key happening to be named `aws_region` as a free string).** →
  Mitigation: the strict-on-declared rule only kicks in once the user
  next saves params via the dialog. Stored values remain untouched
  until then, and a backend startup-time sweep is unnecessary.

- **[Risk] Frontend SchemaForm component becomes a fourth thing to
  maintain alongside pack-params, simulation-params, and the renderer
  in `PackSimulationsSheet`.** → Mitigation: explicitly fold all three
  consumers into the shared component as part of this change; do not
  ship two renderers in parallel.

## Migration Plan

No data migration required — `packs.parameters` JSONB shape is
unchanged. Existing values either match the new built-in schema
(in which case they validate strictly the next time they're saved) or
fall through as unknown keys with a soft warning.

Rollout order, end to end:

1. SDK changes (new `RegisterPackParams`, `ManifestResponse.ParamsSchema`,
   built-in registry, var-based TF rewriting). Updates to bundled tests.
2. Runner / detonator threading of pack-level `-var` flags, with the
   precedence rule.
3. Backend validation on the parameters endpoint, including the
   unknown-keys passthrough.
4. Frontend `SchemaForm` extraction and the dialog upgrade.
5. (Optional, follow-up) `simrun-base-pack` adopts `RegisterPackParams`
   for its real custom params (`resource_prefix`, `vpc_id`, etc.) and
   adds matching `variable {}` blocks to the sims that need them.

Rollback: revert the SDK and runner changes; the backend's strict-on-
declared validation is keyed on `ParamsSchema` being non-empty, so
reverting the SDK (which removes the schema from the manifest)
automatically softens validation back to today's behavior.
