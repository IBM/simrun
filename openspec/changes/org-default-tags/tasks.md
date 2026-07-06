# Tasks: Org-wide Default Tags

## 1. Backend storage and API

- [x] 1.1 Add `DefaultTags map[string]string` to `AppConfig` in `internal/config/appconfig.go` with `{}` default in `DefaultAppConfig`
- [x] 1.2 Map the `default_tags` KV key in `internal/db/config.go` (`parseAppConfig` / `appConfigKVs`)
- [x] 1.3 Add migration backfilling `default_tags = '{}'::jsonb` into `app_config` (idempotent, follows migration 008 pattern)
- [x] 1.4 Add `default_tags` validation branch in `HandleUpdateConfig` (`internal/web/handlers.go`): value must decode as `map[string]string`, else 400
- [x] 1.5 Tests: config store round-trip for `DefaultTags`; handler tests for valid map, non-object, and non-string-value payloads (extend `api_config_test.go`)

## 2. Merge into pack parameters

- [x] 2.1 In `ScenarioService.loadPacksFromDB` (`internal/web/scenarios.go`), load `AppConfig` and merge org default tags per-key beneath each pack's `parameters["default_tags"]` per design D2 (empty org map = pass-through; malformed pack-level value = pass-through, no merge)
- [x] 2.2 Tests encoding the precedence contract: org tag applies when pack has none; pack-level key wins per-key; empty org map leaves parameters byte-identical; malformed pack-level `default_tags` untouched
- [x] 2.3 Verify end-to-end that the merged map reaches `TF_VAR_default_tags` via the existing detonator promotion (existing detonator tests still pass; add one if the merged path is not covered)

## 3. Pack parameters dialog inheritance

- [x] 3.1 In `PackParametersDialog.svelte`, fetch org config via existing `getConfig()` alongside parameters/manifest and extract `default_tags`
- [x] 3.2 Render inherited tags as read-only muted rows in the `default_tags` section, attributed to Settings; mark inherited rows overridden when a pack-level entry uses the same key
- [x] 3.3 Confirm save still submits only pack-level entries (no merged values in the PUT body)

## 4. Verification

- [x] 4.1 `go test ./...` and `mise run lint` pass
- [ ] 4.2 Manual flow: set org tags via `PUT /api/config`, open pack params dialog (inherited rows visible), remove/reinstall a pack, run a scenario, confirm `TF_VAR_default_tags` includes org tags
