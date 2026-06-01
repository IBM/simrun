# Secrets Specification

## Purpose
Stores credentials referenced by connectors as named "secret groups" of
key/value entries, with values encrypted at rest using a server-managed
symmetric key. Plaintext values are write-only via the API: they may be set
but never returned. Decryption happens only at scenario-run time when
credentials are needed.

## Requirements

### Requirement: Secret Group Resource
The system SHALL persist secret groups in the `secret_groups` table with
fields: `id` (UUID), `name` (unique, non-empty), `description`, `entries`
(JSONB map of key → encrypted value), `created_at`, `updated_at`. The system
SHALL also track `created_by` / `updated_by` set from the authenticated
user's email.

#### Scenario: Group with unique name
- **WHEN** a secret group is created with `name="elastic-prod"` and the name does not yet exist
- **THEN** a row is inserted and assigned a UUID

#### Scenario: Duplicate name rejected
- **WHEN** a second secret group is created with a name already in use
- **THEN** the request fails (DB unique constraint violation)

### Requirement: Encryption at Rest
The system SHALL encrypt every entry value with AES-256-GCM and store it as
`base64(nonce || ciphertext)`. Each value MUST be encrypted with a fresh
random nonce. Entry keys are stored in plaintext; only values are encrypted.

#### Scenario: Encrypted storage
- **WHEN** a secret group is created with entry `{"SR_ELASTIC_API_KEY": "abc123"}`
- **THEN** the persisted JSONB contains the key `SR_ELASTIC_API_KEY` mapped to a base64 string that is not equal to `"abc123"`

### Requirement: Encryption Key File
The system SHALL load the AES key from `SR_ENCRYPTION_KEY_FILE`
(default: `<DataDir>/encryption.key`). When the file does not exist at
startup, the system SHALL generate a new 32-byte random key, write it
base64-encoded to that path with mode `0600` (creating any missing parent
directory with mode `0700`), and use it. When the file exists, the system
SHALL load it as-is. Key rotation is not supported.

#### Scenario: Auto-generated key on first start
- **WHEN** the server starts and `<DataDir>/encryption.key` does not exist
- **THEN** a 32-byte key is generated and written to that path with mode `0600`

#### Scenario: Existing key reused on subsequent starts
- **WHEN** the server starts and `<DataDir>/encryption.key` exists
- **THEN** the existing key is loaded and used; the file is not overwritten

#### Scenario: Invalid key file
- **WHEN** the key file exists but contains a non-base64 value or a value that does not decode to 32 bytes
- **THEN** the server fails to start with a fatal error

### Requirement: Plaintext Values Never Returned
`GET /api/secrets` and `GET /api/secrets/{id}` SHALL return only
`{id, name, description, keys, createdBy, updatedBy, createdAt, updatedAt}`
where `keys` is the list of entry key names. The system MUST NOT return
plaintext or encrypted values from any read endpoint.

#### Scenario: Read returns key names only
- **WHEN** a client requests `GET /api/secrets/<id>` for a group with entries `{"K1":"v1","K2":"v2"}`
- **THEN** the response includes `"keys":["K1","K2"]` (order unspecified) and no field containing `"v1"` or `"v2"`

### Requirement: Create Secret Group
`POST /api/secrets` SHALL accept `{name, description, entries: [{key, value}]}`,
encrypt each value, and insert a new row. Empty `name` SHALL be rejected with
HTTP 400.

#### Scenario: Successful create
- **WHEN** a client posts `{"name":"sg","entries":[{"key":"K","value":"v"}]}`
- **THEN** a new secret group row is inserted with one encrypted entry under key `K`

#### Scenario: Empty name rejected
- **WHEN** a client posts a body with `"name":""`
- **THEN** the response is HTTP 400

### Requirement: Update with Null-Value Preservation
`PUT /api/secrets/{id}` SHALL accept `{name, description, entries: [{key, value}]}`.
For each entry: if `value` is non-null, the entry value is replaced (re-encrypted);
if `value` is null, the existing encrypted value for that key is preserved unchanged.
Keys present in the existing group but absent from the update payload SHALL be
removed.

#### Scenario: Rotate one key, preserve another
- **WHEN** an existing group has entries `{"A":"old","B":"old"}` and the client PUTs entries `[{"key":"A","value":"new"},{"key":"B","value":null}]`
- **THEN** the stored encrypted value for `A` decrypts to `"new"` and the stored encrypted value for `B` decrypts to `"old"`

#### Scenario: Removing a key by omission
- **WHEN** an existing group has `{"A":"old","B":"old"}` and the client PUTs entries `[{"key":"A","value":"new"}]`
- **THEN** the stored group has only key `A`

### Requirement: Delete Secret Group
`DELETE /api/secrets/{id}` SHALL remove the row and return HTTP 204. Connectors
that reference the deleted secret group via `secret_group_id` SHALL have that
foreign key set to NULL by the DB (`ON DELETE SET NULL`); the connectors
themselves are not deleted.

#### Scenario: Cascade nullification on connector
- **WHEN** a secret group referenced by a connector is deleted
- **THEN** the connector row remains but its `secret_group_id` is NULL

### Requirement: Decryption at Run Time
At scenario-run time, the system SHALL decrypt all secret groups into a flat
key→value map and merge it into the per-run environment. When two groups
contain the same key, the value from the alphabetically-later group SHALL
overwrite the earlier (last-write-wins by group name order).
**Note:** documents current behavior; collisions are silent.

#### Scenario: Cross-group key collision
- **WHEN** group `aaa` has `{"X":"v1"}` and group `bbb` has `{"X":"v2"}` and both exist
- **THEN** the run environment contains `X=v2`

### Requirement: Decryption Failure Handling
The system SHALL handle a single-entry decryption failure by logging a warning,
omitting that entry from the run environment, and continuing the run (e.g.,
when the key file has been replaced after encryption).

#### Scenario: One entry fails to decrypt
- **WHEN** the run-time decryption of one entry returns an error
- **THEN** that entry is excluded from the run environment and the run proceeds

### Requirement: User Attribution
The system SHALL set `created_by` to the session email on `POST /api/secrets`
and `updated_by` on `PUT /api/secrets/{id}`. When auth is disabled both
fields SHALL be the empty string.

#### Scenario: Authenticated create
- **WHEN** a user with email `alice@example.com` creates a group
- **THEN** the row's `created_by` is `alice@example.com`
