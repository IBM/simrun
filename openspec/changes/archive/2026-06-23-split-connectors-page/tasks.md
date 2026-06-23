## 1. Scaffold the components directory and shared shell

- [x] 1.1 Create `web/frontend/src/lib/components/connectors/` directory.
- [x] 1.2 Create `ConnectorFormShell.svelte` with bindable props for `name`, `description`, `secretGroupId`, `isDefault`, `enabled`, plus flags `showIsDefault`, `showEnabled`, `secretGroupRequired`. Render Label/Input pairs for name + description, a `<Select>` for secretGroupId backed by `$lib/stores/secrets`, a Switch for `isDefault`, a Switch for `enabled`. Render `{@render children?.()}` after the shared fields.
- [x] 1.3 `npm run check` clean (Svelte type-check).

## 2. Extract `ConnectorDetail.svelte` (lowest-risk first)

- [x] 2.1 Create `ConnectorDetail.svelte` accepting `{ connector: Connector, onClose: () => void, onEdit: (c: Connector) => void, onDelete: (c: Connector) => void }` props.
- [x] 2.2 Move the block currently at lines 662-1086 of `+page.svelte` (the `{#if selectedConnector}` ... `{/if}` content) into the new component, including the rules-loading sub-flow, the features list, the overview tab, and any helpers it uses (`getElasticConfig`, `connectorFeatures`, `connectorFeatureDescriptions`).
- [x] 2.3 In `+page.svelte`: replace the moved block with `{#if selectedConnector}<ConnectorDetail connector={selectedConnector} onClose={closeDetail} onEdit={openEdit} onDelete={openDelete} />{/if}`.
- [x] 2.4 Manually verify on the dev server (`npm run dev`): click a connector, detail view renders identically, Elastic rules tab still loads, edit/delete buttons still work from inside the detail view.
- [x] 2.5 `npm run check` clean.

## 3. Extract `AWSConnectorForm.svelte` (simplest type — prove the pattern)

- [x] 3.1 Create `AWSConnectorForm.svelte`. In `<script module>`: export `AWSFormFields` interface (`{ roleArn: string }`) and `emptyAWSFields()` factory.
- [x] 3.2 In `<script>`: `let { fields = $bindable() }: { fields: AWSFormFields } = $props();` Export `validate()`, `buildConfig()`, `populate(connector)`, `canTest()` functions mirroring the existing AWS branches in `isConfigValid` / `buildConfigForType` / `openEdit` / `canTestConnection`.
- [x] 3.3 In the template: render the AWS form JSX (currently lines 1331-1367 of `+page.svelte`).
- [x] 3.4 In `+page.svelte`: import the component but do NOT wire it yet. Compile-clean only.
- [x] 3.5 `npm run check` clean.

## 4. Extract `ConnectorCreateDialog.svelte` and wire it for AWS only

- [x] 4.1 Create `ConnectorCreateDialog.svelte` with the two-step flow (type picker → form). For step 2, render `<ConnectorFormShell ...>` containing a `{#if selectedType === 'aws'}<AWSConnectorForm bind:this={form} bind:fields={awsFields} />{/if}` and stubs for the other types (`<div>TODO: {selectedType} form</div>` for now).
- [x] 4.2 Move `handleTestConnection` and the test-result rendering into the dialog.
- [x] 4.3 Move the test-connection button + Create button + Cancel button + step navigation into the dialog.
- [x] 4.4 The dialog accepts `{ open = $bindable(), onCreated: () => void }` props. On submit, it calls the API directly (`createConnector(...)` from `$lib/api/client`) and invokes `onCreated()` so the parent reloads.
- [x] 4.5 In `+page.svelte`: replace the existing create-dialog block (currently around lines 1090-1783) with `<ConnectorCreateDialog bind:open={createDialogOpen} onCreated={loadConnectors} />`. Manually verify: AWS type creation works end-to-end.

## 5. Extract the remaining per-type form components

- [x] 5.1 Create `SSHConnectorForm.svelte` (~110 lines). Mirror the AWS pattern: `SSHFormFields` + `emptySSHFields()` + 4 exported functions + JSX. Wire into the create dialog dispatch. Verify on dev server.
- [x] 5.2 Create `ElasticConnectorForm.svelte` (~140 lines). Wire into create dialog. Verify.
- [x] 5.3 Create `KubernetesConnectorForm.svelte` (~110 lines). The K8s form needs `existingConnectors: Connector[]` as a prop so it can validate `cloud_connector` references and render the conditional `resource_group` field for Azure-backed clusters. Wire into create dialog. Verify.
- [x] 5.4 Create `GCPConnectorForm.svelte` (~150 lines). Includes the WIF vs service-account auth-type toggle. Wire into create dialog. Verify.
- [x] 5.5 Create `AzureConnectorForm.svelte` (~150 lines). Includes the WIF vs service-principal auth-type toggle. Wire into create dialog. Verify.
- [x] 5.6 Remove all TODO stubs from the create dialog. All 6 types now render through their per-type components.

## 6. Extract `ConnectorEditDialog.svelte`

- [x] 6.1 Create `ConnectorEditDialog.svelte` accepting `{ open = $bindable(), connector: Connector | null, onUpdated: () => void }` props.
- [x] 6.2 Render `<ConnectorFormShell>` with `showEnabled={true}` (the edit dialog uniquely shows the enabled toggle), and dispatch on `connector.type` to the appropriate per-type form component.
- [x] 6.3 On open (effect on `connector` prop), call the form's `populate(connector)` exported function via `bind:this`.
- [x] 6.4 On submit, call `updateConnector(...)` and invoke `onUpdated()`.
- [x] 6.5 In `+page.svelte`: replace the existing edit-dialog block (currently around lines 1785-2130) with `<ConnectorEditDialog bind:open={editDialogOpen} connector={editTarget} onUpdated={loadConnectors} />`.
- [x] 6.6 Manually verify each type's edit flow: open detail → click Edit → fields prefilled correctly → modify → save → list reflects change.

## 7. Extract `ConnectorDeleteDialog.svelte`

- [x] 7.1 Create `ConnectorDeleteDialog.svelte` accepting `{ open = $bindable(), connector: Connector | null, onDeleted: () => void }`.
- [x] 7.2 Move the delete-confirmation dialog block (currently lines 2132-2148) into the component.
- [x] 7.3 In `+page.svelte`: replace the block with `<ConnectorDeleteDialog bind:open={deleteDialogOpen} connector={deleteTarget} onDeleted={loadConnectors} />`.
- [x] 7.4 Manually verify delete still works and the detail view clears when the open connector is deleted.

## 8. Clean up `+page.svelte`

- [x] 8.1 Delete the now-unused script helpers from `+page.svelte`: `buildConfigForType`, `buildConfig`, `buildEditConfig`, `isConfigValid`, `isCreateFormValid`, `isEditFormValid`, `canTestConnection`, the per-type branches of `openEdit`, `ConnectorFormFields` interface, `emptyFormFields`, `getSecretGroupName`/`getSecretHint`/`getElasticConfig` (move helpers to wherever they're consumed if still needed).
- [x] 8.2 Delete the now-unused state: `createForm`, `editForm`, `createStep`, `selectedType`, `testingConnection`, `testResult`, `newCloudConnectorType`, `editCloudConnectorType` (these move into the dialog components).
- [x] 8.3 `+page.svelte` should now contain: imports + page-level state (`loading`, `error`, `selectedConnector`, the three `*DialogOpen` flags, `editTarget`, `deleteTarget`) + `onMount` + simple open/close handlers + the connector list/table + the four dialog + detail component invocations.
- [x] 8.4 Run `wc -l web/frontend/src/routes/connectors/+page.svelte`. MUST be ≤ 300 lines.

## 9. Final verification

- [x] 9.1 `npm run check` clean — no type errors anywhere.
- [x] 9.2 `npm run lint` clean (if configured).
- [x] 9.3 `npm run build` produces a clean production bundle.
- [x] 9.4 Dev-server smoke test: walk through every connector type's create flow (including test-connection where supported), every type's edit flow, delete flow, and the detail view (with Elastic rules listing). Each must behave identically to before the refactor.
- [x] 9.5 `openspec validate split-connectors-page` passes.
