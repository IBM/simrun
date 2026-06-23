## Context

The frontend (`web/frontend`, SvelteKit + Tailwind v4) uses shadcn-svelte components that were generated from an older registry style. A controlled test of `npx shadcn-svelte@latest add pagination --overwrite --yes` (CLI v1.3.0) showed the current registry serves a different style ("nova"): it regenerated `button` with a different look (`destructive` solid → tinted, `rounded-md` → `rounded-lg`, default size `h-9` → `h-8`), introduced a `data-icon` spacing convention, referenced `cn-*` component classes absent from our `app.css`, and bumped `@lucide/svelte` `^1.17` → `^1.21` (pulling renamed icons like `more-horizontal`).

Because of this drift, `add`/`update`/`--overwrite` cannot be used safely today, and the project is frozen on an undocumented style. The codebase places icons inside buttons with `class="mr-2 h-4 w-4"` rather than `data-icon`, so the convention change is cross-cutting across every route and dialog. The theme is the custom "ahsoca" palette (DM Sans / JetBrains Mono, status + attribution tokens, `animate-fade-up`/stagger, indicator-pulse) which must survive the migration.

## Goals / Non-Goals

**Goals:**
- Bring all 33 `ui/*` components onto the current registry style so future `add`/`update` are clean.
- Adopt the `data-icon` convention and migrate all consumers.
- Preserve the ahsoca theme identity and all project-specific component customizations.
- Land with zero new `npm run check` / `mise run build` errors and no visual regressions in light or dark mode.

**Non-Goals:**
- No redesign of pages or new features — this is a style/library migration, not a visual redesign (the recent assessment/coverage redesigns stay as-is, only re-expressed on the new primitives).
- No change to backend, API, or data shapes.
- No switch of icon library, component framework, or Tailwind major version.

## Decisions

### Regenerate with the CLI, then re-apply customizations via diff review
Use `npx shadcn-svelte@latest add <component> --overwrite` (or `update`) to regenerate each component, then inspect `git diff` and re-apply documented project customizations (e.g. `badge` `success` variant, custom `button` variants). Work on a dedicated branch with frequent commits so any single component can be reverted in isolation.
- *Alternative considered*: hand-porting each component by reading the registry. Rejected — slower and error-prone vs. letting the CLI emit canonical output and reviewing the diff.
- *Alternative considered*: re-`init` the whole project. Rejected — too blunt; would also rewrite `components.json`/theme and obscure what changed.

### Treat the icon convention as a global, mechanical pass
After `button` is on the new style, migrate icon usages from `class="mr-2 h-4 w-4"` to `data-icon="inline-start"|"inline-end"` repo-wide. Inventory occurrences with grep, convert per file, and verify spacing visually. Keep standalone icons (not inside a component slot) unchanged.
- *Rationale*: doing this as one sweep avoids a mixed convention that is hard to reason about.

### Reconcile theme tokens additively
Keep the ahsoca tokens and custom utilities in `app.css`; add only what the new style requires (new CSS variables, `cn-*` component classes). Do not let the CLI overwrite `app.css`; merge by hand so the palette and custom animations are preserved.
- *Rationale*: the theme is the product's identity and contains deliberate additions the registry doesn't know about.

### Pin the dependency bump explicitly
Bump `@lucide/svelte` to the version the new style needs in `package.json` as a reviewed change, and re-verify icon import paths that were renamed.

### Sequence: foundation → primitives → composites → consumers → verify
Migrate `button`/`badge` and theme first (highest blast radius), then the rest of `ui/*`, then sweep consumers, then verify build + visual pass. Order chosen so the icon sweep can rely on the final `button`.

## Risks / Trade-offs

- **Dropped customization during regeneration (e.g. `badge.success`)** → Maintain an explicit checklist of known customizations; re-apply and grep-verify each after regenerating its component.
- **Silent visual regression (e.g. tinted vs. solid `destructive`)** → Per-component diff review plus a deliberate light/dark visual pass on key pages; treat emphasis changes on `destructive`/primary actions as blocking.
- **Large, hard-to-review diff** → Commit per component (or small group); keep the consumer icon sweep in its own commit.
- **`app.css` merge conflicts / lost custom tokens** → Never overwrite `app.css` via CLI; diff-review every token change.
- **Dependency bump side effects (renamed/removed icons)** → Build after the bump; fix import paths flagged by the compiler before proceeding.

## Migration Plan

1. Branch off `main`; record the known-customizations checklist (button/badge variants, app.css additions) from current `git` state.
2. Regenerate `button` + `badge`, re-apply variants, reconcile `app.css` tokens/`cn-*` classes, bump `@lucide/svelte`.
3. Regenerate remaining `ui/*` components in small groups; re-apply any customizations; commit per group.
4. Repo-wide icon-convention sweep across `routes/**` and `lib/components/**`.
5. `npm run check` + `mise run build`; fix fallout.
6. Visual pass in light and dark on dashboard, assessments (list + detail), rule coverage, connectors, scenarios, packs.
7. **Rollback**: the work is branch-isolated and committed per component; revert offending commits or the whole branch without touching `main`.

## Open Questions

- Are there `button`/`badge` (or other) customizations beyond the `badge` `success` variant? Resolve by diffing current components against the old-style registry output before starting.
- Does the new style expect any `app.css` changes beyond `cn-*` classes (e.g. new base-color variables)? Confirm against the regenerated components and the style's documented theme.
