# Org-wide Default Tags

## Why

`default_tags` (owner, cost-center, `simulated:true`, …) are org-wide standards, but today they only live in per-pack `packs.parameters`. Users retype them for every pack, and they are lost whenever a pack is removed and reinstalled because pack deletion hard-deletes the row. An app-level setting fixes both: enter once, survives pack lifecycle.

## What Changes

- Add `default_tags` (string→string map, default `{}`) to `AppConfig`, stored under the `default_tags` key in the existing `app_config` KV table and carried by the existing `GET/PUT /api/config` endpoints. `PUT` validates the value is a string→string object (same per-key validation pattern as retention days).
- Migration backfills the `default_tags` key with `{}` so it appears in `GET /api/config` output.
- When the scenario service builds pack configs from the DB (`loadPacksFromDB`), org default tags are merged per-key beneath each pack's own `default_tags`: org value < pack-level value < per-sim scenario param. Packs override individual tags; they cannot delete an org tag.
- Pack parameters dialog shows org default tags as read-only inherited rows (sourced from `GET /api/config`) so users see the effective tag set; a pack-level entry with the same key visibly overrides the inherited one. The dialog never writes merged values into `packs.parameters`.

Editing org default tags uses the existing Configuration page's generic key/value editing (JSON object in the text field) until the settings page redesign (separate change `settings-page-redesign`) ships a proper key/value editor.

## Capabilities

### New Capabilities

- `app-settings`: App-level admin settings — the org-wide `default_tags` setting: storage shape, config API validation, and inherited-tags visibility in the pack parameters dialog.

### Modified Capabilities

- `pack-execution`: Parameter injection gains a new bottom layer — org-wide default tags are merged per-key beneath pack-level `default_tags` before promotion to `TF_VAR_default_tags`.

## Impact

- **Backend**: `internal/config/appconfig.go` (`DefaultTags` field + default), `internal/db/config.go` (KV mapping), new migration (backfill `default_tags` key), `internal/web/handlers.go` (`HandleUpdateConfig` validation branch), `internal/web/scenarios.go` (`loadPacksFromDB` merge).
- **Frontend**: `PackParametersDialog.svelte` (inherited rows). No settings page changes in this proposal.
- **APIs**: no new endpoints; existing config GET/PUT carries the new key.
- **No breaking changes**: existing pack-level `default_tags` keep working and take precedence over org defaults; empty org map is a no-op.
