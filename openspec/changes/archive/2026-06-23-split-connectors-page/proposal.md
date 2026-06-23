## Why

`web/frontend/src/routes/connectors/+page.svelte` is 2,148 lines — by far the largest file in the codebase. The problem is not raw size but structural: it encodes a 6×4 matrix of (connector type × concern) flattened into one file. Adding or modifying a connector type today requires touching ~24 separate locations: the create-dialog form JSX, the edit-dialog form JSX (a near-duplicate of the create form), and 4 parallel per-type branches in `buildConfigForType`, `isConfigValid`, `canTestConnection`, and `openEdit`. We're preparing for OSS release and "open `connectors/+page.svelte` to learn how connectors work" is the most likely path an outside contributor will take — currently they'll see one of the worst-organized files in the repo.

## What Changes

- Extract per-connector-type form components: `ElasticConnectorForm`, `AWSConnectorForm`, `GCPConnectorForm`, `AzureConnectorForm`, `KubernetesConnectorForm`, `SSHConnectorForm`. Each owns its own JSX, validation (`validate()`), config builder (`buildConfig()`), and connector→form populator (`populate()`).
- Extract the create/edit dialogs into `ConnectorCreateDialog`, `ConnectorEditDialog`, `ConnectorDeleteDialog`. Both create and edit dialogs use the same per-type form components — eliminating the current full duplication of form JSX.
- Extract `ConnectorDetail.svelte` from the existing ~425-line `selectedConnector` detail block.
- Extract a shared `ConnectorFormShell.svelte` for the always-present fields (name, description, secret group) plus a slot for type-specific content.
- Split the flat `ConnectorFormFields` interface into 6 per-type interfaces, so each component is typed against only the fields it actually uses.
- Shrink `routes/connectors/+page.svelte` to ~250 lines covering only the connector list/table and top-level dialog orchestration.
- No behavior changes. No backend changes. No HTTP contract changes. Pure presentational refactor.

## Capabilities

### New Capabilities
<!-- None. -->

### Modified Capabilities
- `connectors`: ADD a requirement about UI–backend type parity — the connector-administration UI must provide create/edit forms for every recognized connector type via a single per-type frontend component, so a backend type addition requires exactly one frontend addition and cannot silently bypass the create form.

## Impact

- **Code touched**:
  - `web/frontend/src/routes/connectors/+page.svelte` — shrinks from 2,148 to ~250 lines
  - New: `web/frontend/src/lib/components/connectors/ElasticConnectorForm.svelte` (~140), `AWSConnectorForm.svelte` (~50), `GCPConnectorForm.svelte` (~150), `AzureConnectorForm.svelte` (~150), `KubernetesConnectorForm.svelte` (~110), `SSHConnectorForm.svelte` (~110)
  - New: `ConnectorFormShell.svelte` (~80), `ConnectorCreateDialog.svelte` (~120), `ConnectorEditDialog.svelte` (~80), `ConnectorDeleteDialog.svelte` (~30), `ConnectorDetail.svelte` (~430)
  - `web/frontend/src/lib/types.ts` (or wherever `ConnectorFormFields` lives if internal to the page — currently inline) — split into 6 per-type interfaces
- **Tests**: no current tests on this page; this change does not require adding any. Manual smoke test via the dev server (open create dialog → switch type → fill fields → submit; same for edit; same for delete; same for detail view).
- **APIs**: no HTTP contract changes, no store changes, no backend type changes, no schema changes.
- **Dependencies**: no new npm deps; no removed deps.
- **Risk**: low — purely structural and presentational. The main risk is silently breaking parity between create-form and edit-form behavior; mitigated by both dialogs sharing the same per-type form component.
