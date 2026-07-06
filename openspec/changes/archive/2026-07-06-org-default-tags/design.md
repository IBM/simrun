# Design: Org-wide Default Tags

## Context

`default_tags` is a built-in pack param: the pack SDK guarantees a `variable "default_tags"` block exists in every sim's Terraform and includes it in every pack's `params_schema`, so injecting a value for it is safe for all packs. Today the only persistence is `packs.parameters` (JSONB); `HandleDeletePack` → `packStore.Delete` hard-deletes the row, so tags are lost on remove/reinstall. `app_config` is a generic KV table (`key` TEXT PK, `value` JSONB) with a typed `AppConfig` view (`internal/config/appconfig.go`, mapped in `internal/db/config.go`), and `PUT /api/config` already does per-key validation for retention days.

Alternatives rejected during exploration: named parameter presets with copy-on-apply (doesn't survive reinstall without manual re-apply; drift between preset and stamped copies) and live preset references from packs (merge/FK complexity disproportionate to the problem).

## Goals / Non-Goals

**Goals:**
- Org default tags entered once, applied to every pack's detonations, surviving pack remove/reinstall.
- Per-key override precedence: TF variable default < org default tag < pack-level tag < per-sim scenario param.
- Users can see the effective tag set in the pack parameters dialog.

**Non-Goals:**
- Settings page redesign / proper tags editor UI (separate change: `settings-page-redesign`).
- Tombstones (a pack cannot delete an org tag, only override its value).
- Org-level defaults for any other parameter (`aws_region`, `gcp_project`, custom params).
- Multiple named tag sets ("presets").

## Decisions

### D1: Store in `app_config`, not a new table
One org-wide value, no versioning, no relations — the existing KV table with a typed `AppConfig.DefaultTags map[string]string` field is the established pattern (parallelism, retention, logging flags all live there). A migration backfills `default_tags = {}` following the migration-008 pattern so the key shows up in `GET /api/config`.

### D2: Merge in `loadPacksFromDB`, not the detonator
`ScenarioService.loadPacksFromDB` is the single place DB pack rows become `config.PackConfig`, and it feeds both `Run` and `Lint`. Merging there means every consumer (parser manifest calls, detonator TF_VAR promotion) sees effective parameters, and `internal/detonators/simrun_detonator.go` stays untouched — its existing pack-level < per-sim precedence keeps working on the already-merged map. Alternative (merge inside the detonator) rejected: the detonator has no access to `AppConfig` and would need new plumbing.

Merge rule: `effective = org map, overlaid per-key by pack's parameters["default_tags"]` (when present and a string map). If the pack has no `default_tags` key and the org map is non-empty, the merged org map is set as `parameters["default_tags"]`. Empty org map is a no-op — `parameters` passes through unchanged. A malformed pack-level value (not a string map) is left as-is and org tags are not merged into it, preserving current behavior.

### D3: Validate `default_tags` in `HandleUpdateConfig`
Add a per-key branch (same shape as the retention-days validation): the value must decode as `map[string]string`. Prevents the generic KV editor from storing a string/array that would later break the merge. Other keys keep permissive behavior.

### D4: Pack dialog shows inheritance read-only, fetched from config API
`PackParametersDialog.svelte` additionally calls the existing `getConfig()` and renders org tags as muted, non-editable rows in the `default_tags` section, labeled as inherited from Settings; a pack-level entry with the same key visually marks the inherited row as overridden. Display only — saving still writes only the pack's own entries to `packs.parameters`. This keeps the copy-drift problem out: merged values are never persisted per-pack.

## Risks / Trade-offs

- [Users with existing per-pack tags see no behavior change until they set org tags] → intended; org map defaults to `{}` (no-op), rollout is opt-in.
- [Per-sim `default_tags` param replaces the whole map, not per-key] → existing behavior for pack-level vs per-sim, unchanged; documented in the spec scenario.
- [Until `settings-page-redesign` lands, org tags are edited as raw JSON on the Configuration page] → acceptable interim; D3 validation rejects malformed input at save time.
- [Lint/manifest calls now see merged parameters] → intended (consistent view), but worth verifying no manifest-time validation rejects org-injected tags for packs whose schema marks `default_tags` differently; `default_tags` is a built-in present in every schema, so this is theoretical.

## Migration Plan

1. Deploy migration backfilling `default_tags = '{}'::jsonb` (idempotent `ON CONFLICT DO NOTHING`).
2. Deploy backend + frontend together (single binary).
3. Rollback: revert deploy; the extra `app_config` row is inert for older code (unknown keys are ignored by `parseAppConfig`).

## Open Questions

None — precedence, storage, and merge point were settled during exploration.
