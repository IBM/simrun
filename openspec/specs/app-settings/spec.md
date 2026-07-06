# App Settings Specification

## Purpose
Defines app-level admin settings stored in the `app_config` table and
carried by the existing `GET/PUT /api/config` endpoints. Currently
covers the org-wide `default_tags` setting: its storage shape, config
API validation, and how inherited tags are surfaced in the pack
parameters dialog. The runtime merge of org default tags into pack
parameters is specified in `pack-execution`.

## Requirements

### Requirement: Org-wide Default Tags Setting
The system SHALL store an org-wide default tags value as a string→string
map under the `default_tags` key in the `app_config` table, defaulting to
an empty map, exposed through the typed `AppConfig` as `DefaultTags`. The
key SHALL be backfilled by migration so it appears in `GET /api/config`
responses.

#### Scenario: Default value present after migration
- **WHEN** the server starts against a database without a `default_tags` row
- **THEN** migrations create the row with value `{}` and `GET /api/config` includes `default_tags`

#### Scenario: Survives pack lifecycle
- **WHEN** org default tags are set, and a pack is removed and reinstalled
- **THEN** the org default tags are unchanged and still apply to the reinstalled pack's detonations

### Requirement: Default Tags Update Validation
`PUT /api/config` with key `default_tags` SHALL reject any value that is
not a JSON object whose values are all strings, returning HTTP 400 and
not persisting the value. Valid string→string objects (including the
empty object) SHALL be stored.

#### Scenario: Non-object rejected
- **WHEN** a client sends `PUT /api/config` with key `default_tags` and value `"owner=secops"`
- **THEN** the server responds 400 and the stored value is unchanged

#### Scenario: Non-string tag value rejected
- **WHEN** a client sends `PUT /api/config` with key `default_tags` and value `{"owner": 123}`
- **THEN** the server responds 400 and the stored value is unchanged

#### Scenario: Valid map stored
- **WHEN** a client sends `PUT /api/config` with key `default_tags` and value `{"owner": "secops", "simulated": "true"}`
- **THEN** the server responds 204 and `GET /api/config` returns the new map

### Requirement: Inherited Tags Visible in Pack Parameters Dialog
The pack parameters dialog SHALL display org-wide default tags as
read-only inherited entries within the `default_tags` field, visually
distinct from the pack's own entries and attributed to app settings.
When a pack-level entry uses the same key as an inherited entry, the
dialog SHALL indicate that the inherited value is overridden. Saving the
dialog SHALL persist only the pack's own entries to `packs.parameters`,
never the merged result.

#### Scenario: Inherited tags shown read-only
- **WHEN** org default tags are `{"owner": "secops"}` and a user opens a pack's parameters dialog
- **THEN** `owner: secops` is shown as a non-editable inherited entry in the `default_tags` section

#### Scenario: Override indicated
- **WHEN** org default tags contain `owner: secops` and the pack's own `default_tags` contain `owner: red-team`
- **THEN** the dialog indicates the inherited `owner` value is overridden by the pack-level entry

#### Scenario: Merged values never persisted per-pack
- **WHEN** org default tags are `{"owner": "secops"}`, the pack's own `default_tags` are `{"team": "red"}`, and the user saves the dialog without edits
- **THEN** `packs.parameters.default_tags` contains only `{"team": "red"}`
