# Settings Page Redesign

## Why

The Configuration page (`/config`) renders every `app_config` key through a generic loop: raw system keys in monospace as labels, every value — including booleans and numbers — as a text input, one Save button per row, plus a sensitive-key masking heuristic that is dead weight now that credentials live in secret groups. It cannot reasonably host the org-wide `default_tags` map (change `org-default-tags`), which today would be edited as raw JSON in a text box.

## What Changes

- Replace the generic KV loop with a tabbed Settings page using typed controls and human labels/descriptions for the known `AppConfig` keys:
  - **General**: parallelism (number), terraform version (text), pack logs (switch), SSH session logging (switch)
  - **Default tags**: key/value editor for the org-wide `default_tags` setting, with a description of precedence (packs override individual tags)
  - **Retention**: run log retention and run retention — each an enable switch paired with a days input, days disabled while the switch is off
  - **About**: read-only version info (version, commit, build date, Go version)
- Switches save on toggle; tabs with text/number inputs get one Save action per tab.
- Remove the generic key/value rendering and the sensitive-key masking heuristic; settings metadata (label, description, control type, grouping) is hardcoded in the frontend.
- Extract the string-map (key/value) editor from `SchemaForm.svelte` into a shared component used by both the pack parameters dialog and the Default tags tab.
- Add the shadcn-svelte `field` component; forms use `Field.FieldGroup`/`Field.Field`.

Depends on `org-default-tags` for the `default_tags` key and its server-side validation; implement that change first.

## Capabilities

### New Capabilities

- `settings-page`: The Settings page UI — tab structure, typed controls per setting, save behavior, tags editor, version display, and the shared string-map editor component contract.

### Modified Capabilities

_None — backend config API behavior is unchanged; this is a frontend presentation change._

## Impact

- **Frontend**: `src/routes/config/+page.svelte` (full rewrite), new shared map-editor component under `$lib/components/`, `SchemaForm.svelte` (delegate map editing to the shared component), `$lib/components/ui/field/` (new shadcn component). Uses already-installed `Tabs`, `Card`, `Switch`, `Input`, `InputGroup`.
- **Backend/APIs**: none — existing `GET/PUT /api/config` and `GET /api/version` are sufficient.
- **Behavior loss (accepted)**: unknown/legacy `app_config` keys are no longer visible or editable in the UI; they remain reachable via the API.
