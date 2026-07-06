# Design: Settings Page Redesign

## Context

`/config` (`src/routes/config/+page.svelte`, 156 lines) iterates `Object.entries(config)` and renders each key as `mono label + text Input + Save`. `AppConfig` has 8 known keys (parallelism, terraform_version, pack_logs_enabled, ssh_logging_enabled, run_log_retention_enabled/days, run_retention_enabled/days) plus `default_tags` after the `org-default-tags` change. The frontend follows shadcn-svelte (nova baseline) with restraint-first design taste: quiet, data-grounded, no decorative flourishes. `SchemaForm.svelte` already contains a MapEntry key/value editor used for `default_tags` in the pack parameters dialog.

## Goals / Non-Goals

**Goals:**
- Typed, grouped, human-labeled settings in four tabs: General, Default tags, Retention, About.
- A proper key/value editor for org default tags, shared with the pack parameters dialog.
- Remove dead generic machinery (KV loop, sensitive masking).

**Non-Goals:**
- Any backend or API change.
- Settings search, audit history, or per-user preferences.
- A settings sidebar/sub-navigation (four tabs suffice at this scale).

## Decisions

### D1: Tabs over a single sectioned page
User decision. Four tabs, each substantive: **General** absorbs execution + logging so the landing tab isn't hollow; **Default tags** gets its own surface (the editor plus precedence explanation); **Retention** stands alone because it deletes history — different stakes than tuning execution; **About** separates read-only from editable. Uses installed `Tabs`.

### D2: Hardcoded settings metadata over generic rendering
The frontend declares each setting's label, description, control type, and tab. Rationale: human labels/descriptions require per-key knowledge anyway; the generic loop's only advantage (rendering unknown keys) is not worth booleans-as-text. Trade-off: a future config key requires a frontend change — accepted, it effectively already does. The sensitive-key masking heuristic is deleted (secrets live in secret groups; `app_config` has none).

### D3: Save model — switches commit on toggle, one Save per tab otherwise
Switches are self-describing transactions (existing `PUT /api/config` is per-key, so a toggle is one call). Text/number fields batch into a single Save per tab issuing one PUT per dirty key, replacing eight per-row buttons. Retention day inputs are disabled while their enable switch is off. Errors surface via the existing `Alert` pattern.

### D4: Shared string-map editor extracted from `SchemaForm.svelte`
The MapEntry editor (entries state, `objectToEntries`/`entriesToObject`, blank key/value validation) moves to a shared component (e.g. `$lib/components/KeyValueEditor.svelte`); `SchemaForm` delegates to it for `object` + `additionalProperties.type: "string"` properties, and the Default tags tab uses it directly. One editor, one behavior, both surfaces. It also accepts read-only inherited rows so the pack dialog's inheritance display (from `org-default-tags`) renders through the same component.

### D5: Composition from existing components; add `field`
`Tabs`, `Card`, `Switch`, `Input`, `InputGroup` (days suffix), `Alert`, `Skeleton` are installed. Add shadcn-svelte `field` and build forms with `Field.FieldGroup`/`Field.Field` per the project's forms convention. No custom-styled divs; restraint-first — no new visual vocabulary, the page should read as the same app.

## Risks / Trade-offs

- [Unknown/legacy `app_config` keys become invisible in the UI] → accepted; still reachable via API. Known keys are exactly the `AppConfig` struct.
- [Frontend and backend key lists can drift] → the typed `AppConfig` struct is the source of truth; task includes aligning the frontend `AppConfig` TS type.
- [`SchemaForm` regression while extracting the map editor] → pack parameters dialog is exercised manually as part of verification; extraction is move-not-rewrite.
- [Tab state lost on reload] → acceptable at this scale; General is the default tab.

## Migration Plan

Frontend-only rewrite behind the same route; no data or API migration. Rollback = revert the frontend commit. Implement after `org-default-tags` so the Default tags tab edits a validated, existing key.

## Open Questions

None.
