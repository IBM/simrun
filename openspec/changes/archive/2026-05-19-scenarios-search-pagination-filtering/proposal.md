## Why

The Scenarios page (`/scenarios`) loads every saved scenario in one
unpaginated `GET /api/scenarios` call and renders them as a single table
with no search or filtering. The Assessments page recently shipped
server-side pagination, name search, type toggle, and time-window
filtering; users now expect the same affordances on Scenarios, and the
library is approaching sizes where the dump-everything approach is
visibly slow and hard to navigate.

## What Changes

- Switch `GET /api/scenarios` from "return all rows" to a paginated
  response shaped like `RunPage`: `{scenarios, total, page, perPage}`,
  with `page` (default 1) and `per_page` (default 50, clamped to 100)
  query params. **BREAKING** for any API consumer that assumes the
  endpoint returns a bare JSON array.
- Add server-side filters on the list endpoint:
  - `name` — case-insensitive substring match on `saved_scenarios.name`
  - `type` — repeatable, restricts `saved_scenarios.type` to listed
    values (allowed: `standard`, `explore`, `collect`)
  - `since` — Go duration (e.g. `24h`, `168h`) restricting
    `updated_at >= now() - since`
- Update the Scenarios `+page.svelte` to mirror the Assessments page:
  name search input (debounced 300 ms), multi-select type toggle group,
  time-preset Select for "Updated" window, "Clear filters" affordance,
  paginator with `25 / 50 / 100` page sizes, URL-synced state so
  refresh/back-button preserves the view.
- Update the `scenarios` Svelte store to hold the current page only;
  drop `loadScenarios()`'s "fetch everything" semantics in favor of a
  paginated `loadScenarioPage(page, perPage, filters)` API. Callers
  that need a full list (e.g. sidebar/dashboard widgets, if any) get a
  separate unfiltered first-page fetch.
- Add a request-sequence guard (monotonic counter) on the page so stale
  responses from previous filter/page states are discarded — same
  pattern as Assessments.
- Add a DB index on `saved_scenarios(updated_at DESC)` to keep the
  default `ORDER BY updated_at DESC` cheap once filters land.

## Capabilities

### New Capabilities
<!-- None — this is a refinement of an existing capability. -->

### Modified Capabilities
- `scenarios`: `List Saved Scenarios` requirement changes shape — the
  endpoint becomes paginated and gains `name` / `type` / `since`
  filters. Ordering switches to a paginated `updated_at DESC` slice.

## Impact

- **Backend**: `simrun/internal/db/scenarios.go` (`List` signature
  changes to accept filters + limit/offset, return `ScenarioPage`),
  `simrun/internal/web/handlers.go` (`HandleListScenarios` rewrites
  to parse pagination/filters and return the envelope), new DB
  migration `010_saved_scenarios_updated_at_index.{up,down}.sql`,
  generated `fakes` regenerated via `go generate ./...`.
- **Frontend**: `web/frontend/src/lib/api/client.ts` (new
  `ScenarioFilters` + paginated `listScenarios` signature, new
  `ScenarioListResponse` type), `web/frontend/src/lib/stores/scenarios.ts`,
  `web/frontend/src/routes/scenarios/+page.svelte`, and any other
  caller of `listScenarios` / the `scenarios` store. A grep for
  `scenarios` store consumers and `listScenarios` callers is required
  before flipping the contract.
- **API consumers**: any external script hitting `GET /api/scenarios`
  expecting a bare array breaks; release notes must call this out.
- **Specs**: `openspec/specs/scenarios/spec.md`'s "List Saved
  Scenarios" requirement is updated and gains new scenarios for
  pagination + filtering behavior.
