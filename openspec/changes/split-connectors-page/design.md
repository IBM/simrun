## Context

`web/frontend/src/routes/connectors/+page.svelte` (2,148 lines) was written incrementally as new connector types were added (Elastic first, then AWS, GCP, Azure, Kubernetes, SSH). Each addition appended:

- A new branch in `buildConfigForType` (form → API config)
- A new branch in `isConfigValid` (per-type validation)
- A new branch in `canTestConnection` (whether the type supports test-connection)
- A new branch in `openEdit` (connector → form)
- A new `{#if selectedType === '<type>'}` block in the create dialog template
- A near-duplicate `{#if editTarget.type === '<type>'}` block in the edit dialog template
- A new field set in the flat `ConnectorFormFields` interface

The result is a 6×4-arm structure (six types, four script functions plus two near-duplicate template ladders) that all need to be edited in lock-step. Two pathologies follow:

1. **Adding a seventh connector type requires ~24 separate edits** — easy to miss one, especially the edit-dialog field set, which is silently broken by omission rather than a compile error.
2. **Create and edit form fields drift.** Today they're consistent only by reviewer attention; nothing structurally enforces that the edit dialog supports the same fields the create dialog produces.

The file is also the most likely entry point for an OSS contributor learning the system ("how do I add a new connector type?") and currently it teaches the wrong lesson.

## Goals / Non-Goals

**Goals:**
- Reduce `+page.svelte` to ~250 lines covering only the connector list/table and dialog orchestration.
- Make adding a new connector type a 1-component + 1-dispatch-entry change. No more 24-place edits.
- Eliminate the create-form ↔ edit-form duplication so the two paths cannot drift.
- Co-locate each type's form JSX, validation, config-builder, and populator so a contributor can read one file end-to-end and understand a connector type.

**Non-Goals:**
- No backend changes. No HTTP API changes. No store API changes.
- No new tests. The page has no existing test coverage; adding it is out of scope for this refactor (tracked as a future change).
- No styling/UX changes. Pixel-for-pixel identical output.
- No introduction of a generic "ConnectorForm with runtime type switch" — that would just relocate the 6-arm ladder. Each type gets its own component.
- No relocation of `$lib/stores/connectors` or `$lib/api/client`. Only presentational code moves.

## Decisions

### D1. Per-type form components in `web/frontend/src/lib/components/connectors/`

Each connector type gets a dedicated `.svelte` component that owns four things:

1. The form fields JSX (Label + Input/Select pairs, with help text and per-field validation messages).
2. A `validate(): boolean` exported function (the per-type branch of today's `isConfigValid`).
3. A `buildConfig(): Record<string, unknown>` exported function (the per-type branch of today's `buildConfigForType`).
4. A `populate(connector: Connector): void` exported function (the per-type branch of today's `openEdit`).
5. A `canTest(): boolean` exported function for the create dialog's test-connection button (today's `canTestConnection` branch).

Components use Svelte 5 runes throughout to match the rest of the codebase. Form state is `$state` inside the component, exposed via bindable props so the parent dialog can read the values without prop-drilling each field:

```svelte
<!-- ElasticConnectorForm.svelte -->
<script lang="ts" module>
  export interface ElasticFormFields {
    kibanaUrl: string;
    cloudId: string;
    elasticsearchUrl: string;
    exportEnabled: boolean;
    exportDatastream: string;
  }
  export function emptyElasticFields(): ElasticFormFields { ... }
</script>

<script lang="ts">
  let { fields = $bindable() }: { fields: ElasticFormFields } = $props();
  export function validate(): boolean { ... }
  export function buildConfig(): Record<string, unknown> { ... }
  export function populate(cfg: ElasticConnectorConfig): void { ... }
  export function canTest(): boolean { ... }
</script>

<div class="space-y-2">
  <Label for="kibanaUrl">Kibana URL</Label>
  <Input id="kibanaUrl" bind:value={fields.kibanaUrl} ... />
</div>
... (rest of Elastic fields)
```

The dialog gets a `bind:this={elasticForm}` so it can call `elasticForm.validate()`, `elasticForm.buildConfig()`, etc.

**Alternatives considered:**
- *Single `ConnectorForm` component with a `type` prop and internal switch.* Rejected — relocates rather than removes the 6-arm ladder. Same drift risk, just in a different file.
- *Headless approach: each per-type module exports pure functions, JSX stays in the parent.* Rejected — the JSX is the bulk of the duplication. Co-locating fields-and-logic is the whole point.
- *Imperative API via `bind:this`.* Chosen over event-emitting because the dialog needs to *query* the form (validate, buildConfig, canTest) at submit time, not react to events. `bind:this` matches the access pattern.

### D2. Per-type form-fields interfaces, not one fat union

Today's `ConnectorFormFields` interface (lines 37-74 of the current page) is one flat object with 30+ fields covering all types — most of which are empty/unused for any given connector. Replace with one interface per type, exported from each form component's `<script module>` block (see D1).

The shared/always-present fields (`name`, `description`, `secretGroupId`, `isDefault`, `enabled`) live on `ConnectorFormShell` (D3), not on the per-type interfaces.

**Alternatives considered:**
- *Discriminated union `type ConnectorForm = ElasticForm | AWSForm | ...`.* Rejected as over-engineering — the page never holds a "ConnectorForm of unknown type" value. Each dialog instance owns a specific type-shaped state.
- *Keep the flat interface, only split JSX.* Rejected — leaves the same "every type sees every field" type pollution that hides typos.

### D3. `ConnectorFormShell.svelte` for shared fields

```svelte
<!-- ConnectorFormShell.svelte -->
<script lang="ts">
  let {
    name = $bindable(),
    description = $bindable(),
    secretGroupId = $bindable(),
    isDefault = $bindable(),
    enabled = $bindable(),
    showIsDefault = true,
    showEnabled = false,         // only the edit dialog shows enabled
    secretGroupRequired = false,
    children
  }: { ... } = $props();
</script>

<div class="space-y-2">
  <Label for="name">Name</Label>
  <Input id="name" bind:value={name} />
</div>
<!-- description, secretGroupId, isDefault, enabled fields -->

{@render children?.()}
```

The dialogs compose: `<ConnectorFormShell><ElasticConnectorForm bind:fields={elasticFields} /></ConnectorFormShell>`.

### D4. Three thin dialog components

- **`ConnectorCreateDialog.svelte`** (~120 lines): owns the two-step flow (type picker → form), the test-connection button, and the per-type dispatch table. Internally:
  ```svelte
  {#if step === 'type'}
    <TypePicker bind:selectedType />
  {:else}
    <ConnectorFormShell ...>
      {#if selectedType === 'elastic'}<ElasticConnectorForm bind:this={form} bind:fields={elasticFields} />
      {:else if selectedType === 'aws'}<AWSConnectorForm bind:this={form} bind:fields={awsFields} />
      {:else if ...}
    </ConnectorFormShell>
  {/if}
  ```
  The `form` ref (bound via `bind:this`) gives the dialog a uniform `validate() / buildConfig() / canTest()` surface regardless of type.

- **`ConnectorEditDialog.svelte`** (~80 lines): no type picker; opens with `type` already known from the connector being edited. Otherwise identical dispatch on `type`.

- **`ConnectorDeleteDialog.svelte`** (~30 lines): trivial confirmation dialog.

The 6-arm dispatch table appears in *exactly two places* (create and edit dialog). That's the irreducible minimum. There's no way to drop below 2 without introducing a runtime registry pattern, which would harm static analysis and discoverability for OSS contributors.

### D5. `ConnectorDetail.svelte` extracted as a standalone component

Lines 662-1086 of the current page (the `{#if selectedConnector}` block) are a self-contained read-only display: overview tab, features list, rule listing (for Elastic), connector metadata. Extract as `ConnectorDetail.svelte` accepting `connector: Connector` as a prop plus an `onClose` callback. Zero internal state mutation; pure presentation.

### D6. What stays in `+page.svelte`

- Page layout wrapper
- Connector list/table rendering (this is small in the current page; ~80 lines)
- Dialog open/close orchestration (`createDialogOpen`, `editDialogOpen`, `deleteDialogOpen`, `selectedConnector`)
- Top-level handlers that call API + reload store: `handleCreate`, `handleUpdate`, `handleDelete` (which the dialogs invoke via callback props)
- `onMount` that calls `loadConnectors()` and `loadSecrets()`

Net: ~250 lines.

## Risks / Trade-offs

- [**Risk**: silent regression in the create or edit form — a missing field, wrong default, broken validation.] → Mitigation: extract components one type at a time (D1 sequencing); after each extraction, manually exercise create-dialog and edit-dialog for that type on the dev server before moving to the next.
- [**Risk**: Svelte 5 rune semantics with `bind:this` to call exported functions are slightly unusual.] → Mitigation: prototype the pattern on the simplest type (AWS — only one field) before applying to the others. If the imperative-API approach causes friction, fall back to having the form components emit `{onValidate, onBuildConfig}` callbacks set during mount.
- [**Risk**: 11 new files for one page feels like over-fragmentation.] → Accepted. The current file's failure mode is that nobody can find the AWS form fields without scrolling 1,400 lines. Eleven small files each named for what they do is the right tradeoff.
- [**Trade-off**: per-type form components are not reusable outside the connectors page.] → Accepted — they aren't *meant* to be reusable. They're meant to be *findable* and *modifiable in isolation*. Reuse is not a goal.
- [**Trade-off**: the create and edit dialogs still each contain a 6-arm `{#if selectedType === ...}` dispatch.] → Accepted as the irreducible minimum (D4). Adding a 7th type still requires editing both dispatch lists, but that's two edits in two adjacent components rather than the current 24 edits scattered across 658 lines of script and 1490 lines of template.

## Migration Plan

No data migration; no API change. Deploy is a static asset swap (frontend build artifact). Rollback is the previous build. No feature flag needed.
