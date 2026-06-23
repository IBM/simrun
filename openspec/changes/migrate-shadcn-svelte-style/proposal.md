## Why

The frontend's shadcn-svelte components were generated from an older registry style. The current CLI (v1.3.0+) ships a different style ("nova") with restyled primitives, a new `data-icon` spacing convention, and updated theme expectations. Today `npx shadcn-svelte@latest add <c> --overwrite` is unusable: it silently restyles `button` (e.g. `destructive` flips from solid to tinted), bumps `@lucide/svelte`, and emits classes/icons we don't have. This blocks pulling in any new or updated component cleanly and leaves us pinned to a frozen, undocumented style. Migrating deliberately unblocks future component installs and brings us onto a supported baseline.

## What Changes

- Re-generate all 33 installed UI components (`web/frontend/src/lib/components/ui/*`) from the current shadcn-svelte registry style so `add`/`update`/`--overwrite` become safe going forward.
- **BREAKING (internal)**: Adopt the new `data-icon="inline-start"|"inline-end"` convention for icons inside `Button` (and other components that document it), replacing the current `class="mr-2 h-4 w-4"` / `size`-on-icon pattern used across every page and dialog.
- Reconcile `app.css` theme tokens with what the new style expects (any new CSS variables / utility classes such as the `cn-*` component classes the new components reference), while preserving the existing "ahsoca" palette, fonts (DM Sans / JetBrains Mono), and the custom additions (status/attribution tokens, `animate-fade-up`/`stagger`, indicator-pulse).
- Preserve all project-specific component customizations (e.g. `badge` `success` variant, any custom `button` variants) by re-applying them on top of the regenerated base.
- Bump `@lucide/svelte` to the version the new style requires and verify icon import paths (e.g. `ellipsis` vs `more-horizontal`).
- Audit and update every consumer (`web/frontend/src/routes/**`, `web/frontend/src/lib/components/**`) so buttons, badges, and other restyled primitives render correctly under the new style — no visual regressions in light or dark mode.

## Capabilities

### New Capabilities
- `frontend-design-system`: The shared shadcn-svelte component library, theme tokens, and icon/styling conventions that govern how the SvelteKit frontend is built — including which registry style is the baseline and the rules for adding/updating components.

### Modified Capabilities
<!-- No existing spec captures the frontend component library; nothing modified. -->

## Impact

- **Components**: all of `web/frontend/src/lib/components/ui/*` (33 components) regenerated; project-specific variants re-applied.
- **Consumers**: every route under `web/frontend/src/routes/**` and shared components under `web/frontend/src/lib/components/**` that render `Button` with icons or use restyled primitives.
- **Theme**: `web/frontend/src/app.css` (CSS variables, custom utilities) reconciled with the new style.
- **Dependencies**: `@lucide/svelte` version bump in `web/frontend/package.json` + lockfile.
- **Verification**: `mise run build`, `npm run check`, and a visual pass across pages in both themes.
- **Risk**: visual regressions if a customization is dropped during regeneration; mitigated by per-component diff review and the recorded list of known customizations.
