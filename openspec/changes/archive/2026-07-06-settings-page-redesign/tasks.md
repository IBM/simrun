# Tasks: Settings Page Redesign

## 1. Foundations

- [x] 1.1 Add the shadcn-svelte `field` component (`npx shadcn-svelte@latest add field`)
- [x] 1.2 Extract the string-map editor from `SchemaForm.svelte` into a shared `KeyValueEditor` component (entries state, `objectToEntries`/`entriesToObject`, blank-entry validation, optional read-only inherited rows); wire `SchemaForm` to delegate to it
- [x] 1.3 Verify pack parameters dialog behavior is unchanged after the extraction (add/remove/validate/save `default_tags`)
- [x] 1.4 Align the frontend `AppConfig` TS type with the Go struct (all 8 keys + `default_tags`)

## 2. Settings page rewrite

- [x] 2.1 Rebuild `src/routes/config/+page.svelte` with four tabs (General default, Default tags, Retention, About) using `Tabs` + `Field.FieldGroup`/`Field.Field`; delete the generic KV loop and sensitive-key masking
- [x] 2.2 General tab: parallelism (number), terraform version (text), pack logs (switch), SSH session logging (switch); human labels + descriptions
- [x] 2.3 Default tags tab: `KeyValueEditor` for `default_tags` with precedence description; blank entries rejected client-side
- [x] 2.4 Retention tab: enable switch + days input pairs; days disabled while switch is off
- [x] 2.5 About tab: version, commit, build date, Go version (read-only)
- [x] 2.6 Save behavior: switches persist on toggle; per-tab Save writes only dirty keys; failed saves show an Alert and keep edits

## 3. Verification

- [x] 3.1 `mise run build-frontend` and svelte-check pass
- [x] 3.2 Manual flow: edit every setting type (switch, number, text, tags map), reload, confirm persistence; verify save-failure path keeps edits (e.g. invalid retention days rejected by server validation)
- [x] 3.3 Verify pack parameters dialog still edits `default_tags` correctly through the shared editor
