## ADDED Requirements

### Requirement: UI–Backend Connector Type Parity
The connector-administration UI SHALL provide create, edit, and delete capabilities for every connector type recognized by the backend. The per-type configuration form for each recognized type SHALL be implemented in exactly one frontend component file, so that adding a new backend-recognized type requires exactly one frontend file addition plus a registration in the create-dialog and edit-dialog dispatch tables — no other frontend changes SHALL be required for type parity.

#### Scenario: Every recognized type is configurable in the UI
- **WHEN** the backend recognizes connector types `elastic`, `datadog`, `aws`, `gcp`, `azure`, `kubernetes`, `ssh`
- **THEN** the create dialog presents each as a selectable type
- **AND** the edit dialog renders the appropriate form fields when a connector of that type is opened

#### Scenario: Adding a new type is a one-component addition
- **WHEN** a contributor adds a new connector type `X` to the backend
- **THEN** adding UI support requires creating exactly one new component file `web/frontend/src/lib/components/connectors/XConnectorForm.svelte`
- **AND** registering it in the create-dialog dispatch and the edit-dialog dispatch
- **AND** no other frontend changes are required for the type to be fully configurable
