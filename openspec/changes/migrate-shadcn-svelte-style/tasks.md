## 1. Preparation

- [x] 1.1 Create a migration branch off `main` (isolated, revertible). (Branched off `ui/revamp` HEAD, not `main`, since this change depends on recent rule-coverage/pagination commits there.)
- [x] 1.2 Confirm the shadcn-svelte CLI version and the registry style it resolves to; note expected differences from the current components.
- [x] 1.3 Build a "known customizations" checklist by diffing current `ui/*` components against old-style registry output (capture `badge` `success` variant, any custom `button` variants, and any other deviations).
- [ ] 1.4 Snapshot current visual baseline (screenshots of key pages in light + dark) for later comparison.

## 2. Foundation: theme + primitives

- [x] 2.1 Regenerate `button` from the registry; review `git diff`.
- [x] 2.2 Re-apply project `button` customizations from the checklist; verify variants/sizes still exported.
- [x] 2.3 Regenerate `badge`; re-apply the `success` variant and any others; verify.
- [x] 2.4 Reconcile `app.css`: add new style's required CSS variables and `cn-*` component classes; preserve ahsoca palette, fonts, status/attribution tokens, and custom animations (do NOT let the CLI overwrite `app.css`).
- [x] 2.5 Bump `@lucide/svelte` to the version the new style requires; update `package.json` + lockfile; fix any renamed icon import paths surfaced by the build.

## 3. Remaining components

- [x] 3.1 Regenerate the remaining `ui/*` components in small groups, committing per group, re-applying any checklist customizations.
- [x] 3.2 For each group, confirm no `cn-*` or other classes remain undefined in `app.css`.
- [x] 3.3 Verify `pagination` (recently added) matches the new style and still satisfies the rule-coverage page usage.

## 4. Consumer migration (icon convention)

- [x] 4.1 Inventory all icon-in-component usages across `routes/**` and `lib/components/**` (grep for `mr-2`/`ml-2`/`h-4 w-4` icons inside `Button`).
- [x] 4.2 Convert button icons to `data-icon="inline-start"|"inline-end"`, removing manual margin/size classes; leave standalone icons unchanged.
- [x] 4.3 Update any other consumers affected by restyled primitives (sizes, radii, emphasis) so layouts still hold.

## 5. Verification

- [x] 5.1 `npm run check` passes with no new errors attributable to the migration.
- [x] 5.2 `mise run build` (frontend + server) succeeds.
- [x] 5.3 Visual pass in light AND dark on: dashboard, assessments list, assessment detail, rule coverage, connectors, scenarios, packs — confirm buttons (incl. `destructive`), badges, inputs, tables render with intended emphasis and correct spacing.
- [x] 5.4 Confirm a clean `add`/`update --overwrite` of one already-installed component now leaves the rest of the library unchanged (validates the migration goal).
- [x] 5.5 Update the project memory note that previously flagged `--overwrite` as incompatible to reflect the new baseline.

## 6. Wrap-up

- [ ] 6.1 Open a PR summarizing the style migration, the `data-icon` convention change, and the dependency bump.
- [ ] 6.2 After merge, run `/opsx:archive` to archive this change.
