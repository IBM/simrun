## 1. Pre-work

- [x] 1.1 Grep `web/frontend/src` for all callers of `listScenarios` and the `scenarios` store; list them so each call site can be updated atomically with the API shape flip (`rg -n "listScenarios|stores/scenarios" web/frontend/src`)
- [x] 1.2 Confirm `NewAssessmentDialog.svelte` either keeps working with the paginated store (page-1, no filter) or switches to its own fetch — resolve design.md Open Question

## 2. Database

- [x] 2.1 Add migration `simrun/internal/db/migrations/010_saved_scenarios_updated_at_index.up.sql` creating `idx_saved_scenarios_updated_at` on `saved_scenarios (updated_at DESC)`
- [x] 2.2 Add matching `010_saved_scenarios_updated_at_index.down.sql` dropping the index
- [x] 2.3 Verify migrations run on startup against a fresh DB (`go test ./simrun/internal/db/...` if migration tests exist; otherwise spot-check manually) — no migration tests; pattern matches 009 exactly

## 3. Backend store

- [x] 3.1 In `simrun/internal/db/scenarios.go`, add `ScenarioPage{Scenarios []SavedScenario; Total int}` and `ListScenariosFilters{Name string; Types []string; Since *time.Time}`
- [x] 3.2 Change `ScenarioStore.List` signature to `List(ctx, ListScenariosFilters, limit, offset) (ScenarioPage, error)` and update the implementation to mirror `runStore.List` (WHERE-builder + `COUNT(*) OVER()` + empty-page COUNT fallback). Also added `ListAll(ctx)` for the coverage-map use case
- [x] 3.3 Implement `buildScenariosWhere(ListScenariosFilters)` returning `(whereSQL, args)`; `name` → `ILIKE`, `Types` → `IN (...)`, `Since` → `updated_at >= $N`
- [x] 3.4 Regenerate mocks: `go generate ./...` — `ScenarioStore` is not in `.mockery.yml`; the hand-rolled fake in `testutil/fakes/fakes.go` was updated instead (see 4.4)

## 4. Backend handler

- [x] 4.1 In `simrun/internal/web/handlers.go`, add `parseScenarioFilters(r) (db.ListScenariosFilters, error)` next to `parseRunFilters`, reusing `validScenarioTypes`
- [x] 4.2 Rewrite `HandleListScenarios` to call `parsePagination(r, 50, 100)` + `parseScenarioFilters`, invoke `ScenarioStore.List`, and respond with `{scenarios, total, page, perPage}`
- [x] 4.3 Update `simrun/internal/web/api_runs_test.go` (or add `api_scenarios_test.go` if it doesn't exist) with tests for: default page, name filter, type filter (valid + invalid), since filter (valid + invalid), per_page clamp, empty page beyond range
- [x] 4.4 Update `simrun/internal/testutil/fakes/fakes.go` if a hand-written ScenarioStore fake exists there (or rely on the generated mocks regenerated in 3.4)

## 5. Frontend API client + store

- [x] 5.1 In `web/frontend/src/lib/api/client.ts`, add `ScenarioFilters {name?, types?, since?}` and `ScenarioListResponse {scenarios, total, page, perPage}`; change `listScenarios(page=1, perPage=50, filters={})` to build a `URLSearchParams` (same pattern as `listRuns`) and return the envelope
- [x] 5.2 In `web/frontend/src/lib/stores/scenarios.ts`, replace `loadScenarios()` with `loadScenarioPage(page, perPage, filters)` that calls the new `listScenarios` and `set`s the current page rows on the store; keep `scenarios` as `writable<SavedScenario[]>` so existing reads still work
- [x] 5.3 Update any caller flagged in 1.1 (other than `routes/scenarios/+page.svelte`) to use the new API — dashboard (`routes/+page.svelte`) and `NewAssessmentDialog` switch to direct paginated fetches; `routes/scenarios/{new, [id], [id]/edit}` drop their redundant `loadScenarios()` calls (they `goto` away anyway)

## 6. Frontend page

- [x] 6.1 Port the Assessments page filter UI scaffolding into `web/frontend/src/routes/scenarios/+page.svelte`: name `Input` (debounced 300ms), `ToggleGroup` for `SCENARIO_TYPES`, `Select` for `TIME_PRESETS` ("Updated"), "Clear filters" `Button` gated on `hasActiveFilters`
- [x] 6.2 Add pagination state (`page`, `perPage`, `total`, `pageRuns`-equivalent `pageScenarios`) and the URL-sync (`seedFromUrl` / `syncUrl`) + page-range builder + request-sequence guard, all copied from Assessments
- [x] 6.3 Wire `load()` to call `loadScenarioPage(page, perPage, currentFilters())` and update `total` + the local page array
- [x] 6.4 Render the paginator (prev / numbered / next + "Rows per page" Select with `[25, 50, 100]`) and the "Showing X–Y of N" summary, identical to Assessments' layout
- [x] 6.5 Update the delete handler to follow the Assessments pattern: if deleting the only row on a page > 1, step back one page before reloading; otherwise just reload the current page
- [x] 6.6 Update the rename handler and schedule-success handler to reload the current page (not the full list)
- [x] 6.7 Update the Empty state to show "No matching scenarios" / "Clear filters" when `hasActiveFilters`, mirroring Assessments

## 7. Verification

- [x] 7.1 `mise run build` succeeds (frontend + Go build)
- [x] 7.2 `go test ./simrun/...` passes
- [x] 7.3 Manually load `/scenarios` in the running dev server: verify default load, name search debounce, type toggle multi-select, time preset, clear-filters, paginator navigation, page-size change, URL reflects state on refresh, delete from page>1 with one row steps back correctly
- [x] 7.4 Manually confirm `NewAssessmentDialog` (or any other consumer of saved scenarios) still lets users pick a scenario after the store contract change
- [ ] 7.5 Update PR description to flag the `GET /api/scenarios` response-shape break for any external API consumers
