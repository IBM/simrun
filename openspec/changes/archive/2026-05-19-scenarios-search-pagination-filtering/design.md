## Context

`GET /api/scenarios` currently returns every saved scenario as a JSON
array, sorted by `updated_at DESC`. The frontend store
(`web/frontend/src/lib/stores/scenarios.ts`) caches the full list and
the Scenarios page renders it as one unfiltered table. The Assessments
page (`/assessments`) recently shipped server-side pagination,
substring search, multi-type toggle, time-window filter, and URL
state-sync — backed by `RunStore.List(ctx, filters, limit, offset)`
returning `RunPage{Runs, Total}` and `HandleListRuns` parsing
`page` / `per_page` / `name` / `type` / `since`. We want feature parity
on Scenarios. The two stores are unrelated entities, so this is a
copy-the-pattern job, not a shared-abstraction job.

Saved scenarios share `name`, `type`, and timestamps with the run
filter inputs; the same `validScenarioTypes` map already lives in
`simrun/internal/web/handlers.go` (used by `parseRunFilters`). The
`parsePagination` helper is also already there and is reusable.

## Goals / Non-Goals

**Goals:**
- Server-side pagination on `GET /api/scenarios` with the same envelope
  shape as `GET /api/runs` (`{scenarios, total, page, perPage}`).
- Filter by name (substring, case-insensitive), type (multi-value), and
  "updated since" Go-duration window.
- Frontend page matches Assessments' UX patterns: debounced search,
  toggle group, time preset Select, paginator with 25/50/100 page
  sizes, URL state-sync, request-sequence guard for stale responses.
- Keep `DELETE`, rename, and schedule flows on the Scenarios page
  working (they currently call `loadScenarios()` after mutating).

**Non-Goals:**
- Sort controls (still always `updated_at DESC`).
- Filter on `created_by` / `updated_by` (no UI need yet).
- Filter on schedule presence / schedule enabled state (would require
  joining `schedules`; out of scope, can follow up).
- Faceted counts ("X standard, Y explore, Z collect"). Not in
  Assessments, not adding here.
- Cursor-based pagination. Offset is fine at expected list sizes; the
  existing `COUNT(*) OVER()` pattern in `runStore.List` is the
  template.
- Backward-compatible response shape. The endpoint flips to the
  envelope; we update all in-tree callers atomically.

## Decisions

### Decision 1: Match `RunStore.List` shape verbatim
- **Choice:** New `ScenarioStore.List(ctx, ListScenariosFilters,
  limit, offset) (ScenarioPage, error)` replacing the current
  no-arg `List(ctx) ([]SavedScenario, error)`. Introduce
  `ScenarioPage{Scenarios []SavedScenario; Total int}` and
  `ListScenariosFilters{Name string; Types []string; Since *time.Time}`.
- **Alternative considered:** Keep the current `List` and add a second
  `ListPage` method. Rejected — every in-tree caller (just the
  handler) wants pagination; carrying both is dead weight and risks
  callers grabbing the unbounded version.
- **Why:** Mirrors `RunStore.List` exactly, so reviewers see one
  pattern. The SQL pattern (`COUNT(*) OVER()` + fallback `COUNT(*)`
  when the page is empty) is copy-paste from `runs.go` with the
  WHERE-builder swapped out.

### Decision 2: Reuse `parsePagination` and `validScenarioTypes`
- **Choice:** `HandleListScenarios` calls the existing
  `parsePagination(r, 50, 100)` and re-uses the same
  `validScenarioTypes` map for type validation. Add a new
  `parseScenarioFilters` next to `parseRunFilters` (they differ only
  in which time column `since` applies to and the absence of
  `scenario_id`).
- **Alternative considered:** Hoist a single `parseListFilters`
  shared between runs and scenarios. Rejected — `scenario_id`
  doesn't apply to scenarios and `since` semantics differ
  (`runs.created_at` vs `saved_scenarios.updated_at`). One small
  copy is cheaper than a configurable abstraction.
- **Why:** Rule 6 — when two callers want different filter sets, two
  parse functions beat one parameterized one.

### Decision 3: `since` filters on `updated_at`, not `created_at`
- **Choice:** Scenarios' default order is `ORDER BY updated_at DESC`
  (existing behavior, used as the "most-recently-updated first"
  promise). To keep "Last 24 hours" semantically aligned with that
  order, `since` filters on `updated_at >= now() - dur`.
- **Alternative considered:** Filter on `created_at` to match runs.
  Rejected — users will read "updated in the last 24 hours" from the
  ordering and that's the more useful question.
- **Why:** The list is already sorted by update time; the filter
  should match the column the user is implicitly looking at.

### Decision 4: Frontend page copies Assessments wholesale
- **Choice:** Lift the URL-sync + request-sequence + debounce + page
  range builder pattern from `assessments/+page.svelte` directly into
  `scenarios/+page.svelte`. No shared component yet.
- **Alternative considered:** Extract a `<PaginatedTable>` /
  `usePagination` helper. Rejected for this change — only two
  callers, and the table bodies (columns, row actions) diverge
  enough that abstracting now would force premature decisions.
- **Why:** Rule 2 — minimum code that solves the problem. Once a third
  page wants the pattern, refactor then.

### Decision 5: `scenarios` store holds the current page only
- **Choice:** Replace `loadScenarios()` (full list) with
  `loadScenarioPage(page, perPage, filters)` that updates the store
  with the current page's rows + total. Callers that mutated and
  then called `loadScenarios()` (delete, rename, schedule
  refresh) instead call the new function with the page's
  *current* filters/page. The Scenarios page owns the filter state
  and passes it in; other callers (the `new` and `[id]` routes
  trigger their own `goto('/scenarios')`, so they don't need to
  refresh the store directly).
- **Alternative considered:** Keep two stores — one for the full
  list (sidebar autocompletes?) and one for the page. Rejected
  pending a real consumer; no current code relies on the full
  list outside the list page itself.
- **Why:** Smallest blast radius — one store, one contract, one
  page that owns the filters.

### Decision 6: Index `saved_scenarios(updated_at DESC)`
- **Choice:** Add migration
  `010_saved_scenarios_updated_at_index.{up,down}.sql` creating
  `CREATE INDEX IF NOT EXISTS idx_saved_scenarios_updated_at
  ON saved_scenarios (updated_at DESC);`.
- **Why:** Runs added `009_runs_created_at_index` for the same
  reason. Default sort + `since` predicate both hit this column;
  an index keeps the paginator cheap at scale.

### Decision 7: Polling parity with Assessments — skip
- **Choice:** No polling on the Scenarios page. Assessments polls
  because running runs are mutable and time-sensitive; saved
  scenarios mutate only on user action (save / rename / delete /
  schedule toggle), all of which trigger an explicit reload.
- **Why:** Avoids carrying complexity (the request-sequence guard,
  `stopPolling`, the `page === 1 && !hasActiveFilters` global-store
  write) that has no driver on this page.

## Risks / Trade-offs

- **[Risk] Breaking external `GET /api/scenarios` consumers.**
  → **Mitigation:** The repo's only caller is the SvelteKit frontend
  (`listScenarios` in `client.ts`); we flip both atomically in the
  same PR. Call this out in the PR description and in release notes.
  No deprecation period since there is no documented external
  contract.

- **[Risk] `LEFT JOIN`-less ILIKE on `name` is cheap today but
  unindexed.** → **Mitigation:** Accept it; `saved_scenarios` is
  expected to stay small (hundreds, not millions). If the table
  grows, follow up with a `pg_trgm` GIN index on `name`. Out of
  scope here.

- **[Risk] Off-by-one on "last page after delete".** Assessments
  handles this by decrementing `page` before reloading when the
  current page has exactly one row left and we're past page 1.
  → **Mitigation:** Port that same conditional into the Scenarios
  delete handler.

- **[Trade-off] No back-compat envelope.** A future client written
  against the old array shape will 500 (or render junk). Acceptable
  because (a) no documented external clients, (b) the envelope
  matches the runs endpoint that ships from the same binary.

- **[Trade-off] Rename / schedule operations now reload only the
  current page.** If a rename pushes a scenario off the active
  filtered view (e.g. the new name no longer matches the search),
  it visibly disappears. That's correct behavior — the filter is the
  user's stated intent — but worth noting.

## Migration Plan

1. Ship the migration first (it's additive, safe to apply
   independently). Server startup runs migrations, so the index lands
   on next deploy.
2. Backend + frontend land in one PR — there's no way to do them
   independently without breaking the page.
3. Rollback: revert the PR; the down migration drops the index. No
   data shape changes, so no data backfill or cleanup needed.

## Open Questions

- Are there *any* other in-tree consumers of `listScenarios()` or the
  `scenarios` store beyond the Scenarios page and the
  `NewAssessmentDialog`? A pre-implementation grep
  (`rg "listScenarios|stores/scenarios" web/frontend/src`) will
  confirm. If `NewAssessmentDialog` reads the full list to populate
  a picker, it must keep working — either by fetching its own
  unfiltered first page or by switching to a dedicated
  `GET /api/scenarios?per_page=1000` call. Resolve before flipping
  the store contract.
