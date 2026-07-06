# Settings Page Delta

## ADDED Requirements

### Requirement: Tabbed Settings Layout
The Settings page SHALL organize settings into four tabs — General,
Default tags, Retention, and About — with General as the default tab.
General SHALL contain parallelism, terraform version, pack logs, and
SSH session logging. Retention SHALL contain run log retention and run
retention. About SHALL contain read-only version information (version,
commit, build date, Go version).

#### Scenario: Landing on General
- **WHEN** a user navigates to the Settings page
- **THEN** the General tab is active, showing parallelism, terraform version, pack logs, and SSH session logging controls

#### Scenario: About is read-only
- **WHEN** a user opens the About tab
- **THEN** version, commit, build date, and Go version are displayed with no editable controls

### Requirement: Typed Controls with Human Labels
Each known setting SHALL render with a human-readable label and
description and a control matching its type: switches for booleans,
number inputs for integer settings, text inputs for strings. Raw
`app_config` keys SHALL NOT be used as labels. The system SHALL NOT
render settings through a generic key/value loop and SHALL NOT apply
sensitive-key masking.

#### Scenario: Boolean is a switch
- **WHEN** the General tab renders the pack logs setting (`pack_logs_enabled`)
- **THEN** it is a switch with a human label, not a text input containing "true"

#### Scenario: Unknown keys not rendered
- **WHEN** `app_config` contains a key not known to the frontend
- **THEN** the Settings page does not render a control for it

### Requirement: Save Behavior
Switch settings SHALL persist immediately on toggle via
`PUT /api/config`. Tabs containing text or number inputs SHALL provide
a single Save action that persists only the dirty keys of that tab.
Failed saves SHALL surface an error without discarding the user's
edits.

#### Scenario: Toggle saves immediately
- **WHEN** a user toggles SSH session logging
- **THEN** the value is persisted via `PUT /api/config` without a separate Save action

#### Scenario: Per-tab save of dirty keys
- **WHEN** a user edits parallelism on the General tab and clicks Save
- **THEN** only the changed key is written; unchanged keys are not re-persisted

#### Scenario: Save failure keeps edits
- **WHEN** a save request fails
- **THEN** an error is shown and the edited values remain in the form

### Requirement: Retention Enable/Days Pairing
Each retention setting SHALL render as an enable switch paired with a
days input, and the days input SHALL be disabled while its switch is
off.

#### Scenario: Days disabled when retention off
- **WHEN** run retention is toggled off
- **THEN** the run retention days input is disabled

### Requirement: Default Tags Tab
The Default tags tab SHALL edit the org-wide `default_tags` setting
with a key/value editor and SHALL describe where the tags apply and
that packs can override individual tags. Blank keys or values SHALL be
rejected before saving.

#### Scenario: Editing org tags
- **WHEN** a user adds `owner: secops` in the Default tags tab and saves
- **THEN** `PUT /api/config` persists `default_tags` as `{"owner": "secops"}`

#### Scenario: Blank entry rejected client-side
- **WHEN** a user adds an entry with an empty key and saves
- **THEN** a validation message is shown and no request is sent

### Requirement: Shared String-Map Editor
The key/value map editor SHALL be a single shared component used by
both the pack parameters dialog (via `SchemaForm`) and the Default tags
tab, preserving the existing editor behavior (add/remove entries,
blank-entry validation) and supporting read-only inherited rows for the
pack dialog's inheritance display.

#### Scenario: One editor, two surfaces
- **WHEN** a user edits `default_tags` in the pack parameters dialog and in the Settings Default tags tab
- **THEN** both use the same key/value editor component with identical add/remove/validation behavior
