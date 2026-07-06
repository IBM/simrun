# Packs Specification (Install Lifecycle)

## Purpose
Manages the install lifecycle of attack-simulation packs: discovery,
fetching, persistence, removal, and parameter configuration. Packs are
external binaries that follow the simrun pack protocol (manifest /
detonate / cleanup over stdin/stdout). This spec covers everything the
operator does via `/api/packs/*` to make a pack available for use.
Pack-runtime behavior (how packs are invoked during a scenario run,
terraform lifecycle, parameter injection) is in `pack-execution`.

## Requirements

### Requirement: Pack Resource
The system SHALL persist packs in the `packs` table with fields:
`id` (UUID), `name` (unique, non-empty), `type`, `source`, `version`
(nullable), `status`, `parameters` (JSONB), `installed_by`,
`created_at`, `updated_at`.

#### Scenario: Unique pack name
- **WHEN** a pack named `aws-attacks` is installed and a second install with the same name is requested
- **THEN** the second install is treated as an upsert, replacing the existing row

### Requirement: Recognized Pack Types
The system SHALL accept three pack types: `local`, `remote`, and
`upload`. Each type SHALL be installed via a type-specific source
resolution path; once installed, the persisted record carries the
binary path that runtime resolves. The system SHALL reject any other
`type` value at install time with a validation error and SHALL NOT
persist a row for it.

#### Scenario: Type stored
- **WHEN** a remote pack is installed
- **THEN** the row has `type = "remote"` and `source` set to the GitHub repo path

#### Scenario: Unrecognized type rejected at install
- **WHEN** a client posts POST /api/packs/install with type: "go-remote"
- **THEN** the install is rejected with a validation error naming the allowed types (local, remote, upload) and no packs row is created

### Requirement: Install Derives Identity From Manifest
The system SHALL derive a pack's `name` and `version` from the binary's
`manifest` command (`pack.name` and `pack.version`) at install time, for all pack
types. The system SHALL ignore any `name` supplied in the install request. The
install SHALL run the manifest only after the binary is available (downloaded for
`remote`, present on disk for `local`, written to a temp location for `upload`),
and SHALL fail without creating a DB record if the manifest command fails.

#### Scenario: Name comes from manifest, not request
- **WHEN** a client posts an install request whose body contains `name: "operator-typed"` and the resolved binary's manifest reports `pack.name: "real-pack"`
- **THEN** the persisted row's `name` is `real-pack` and the request's `name` is ignored

#### Scenario: Version pinned from manifest or resolved tag
- **WHEN** an install completes successfully
- **THEN** the persisted `version` reflects the resolved release tag (remote) or the manifest's `pack.version` (local/upload), never an operator-typed value beyond an optional remote version pin

#### Scenario: Manifest failure aborts install
- **WHEN** the `manifest` command fails for the resolved binary
- **THEN** the install fails with an error and no DB record is created

### Requirement: Local Pack Install
The system SHALL accept `local` packs whose `source` is an absolute filesystem
path to an existing binary. The install endpoint SHALL NOT copy the binary; it
SHALL reference the path as-is at run time. At install time the system SHALL
verify the path exists and SHALL run the binary's `manifest` command to derive
the pack's identity (see "Install Derives Identity From Manifest"). An install
SHALL fail if the path does not exist or the manifest command fails.

#### Scenario: Path stored verbatim
- **WHEN** a client installs a local pack with `source: "/opt/packs/my-pack"` that exists and returns a valid manifest
- **THEN** the persisted row has `source = "/opt/packs/my-pack"` and the name/version come from the manifest

#### Scenario: Non-existent path rejected at install
- **WHEN** a client installs a local pack whose `source` path does not exist
- **THEN** the install fails with an error and no DB record is created

### Requirement: Remote Pack Install
The system SHALL fetch `remote` packs through the GitHub Releases API. When
the request omits a version, the system SHALL resolve the release via
`GET /repos/<org>/<repo>/releases/latest`; when a version is provided, via
`GET /repos/<org>/<repo>/releases/tags/v<version>`. The concrete resolved tag
SHALL be persisted as the pack's `version`. From the release's assets the system
SHALL select the archive whose name matches `*_<os>_<arch>.tar.gz`, fetch the
release's `checksums.txt` asset, verify the archive's SHA-256 against it, extract
the pack binary, and cache it at `<DataDir>/packs/<name>/<tag>/<binary>` — where
`<name>` is derived from the manifest (see "Install Derives Identity From
Manifest"). The in-tarball binary name SHALL be derived from the selected asset's
filename prefix; if the archive contains a single executable, that file SHALL be
used. The system SHALL NOT require the operator to supply the artifact filename
or the binary name.

#### Scenario: Successful download of a pinned version
- **WHEN** a client installs a `remote` pack from `github.com/org/repo` with version `1.2.3`
- **THEN** the release at tag `v1.2.3` is resolved, the matching `*_<os>_<arch>.tar.gz` asset is checksum-verified and extracted, the binary is cached under `<DataDir>/packs/<manifest-name>/1.2.3/`, and the row's `version` is `1.2.3`

#### Scenario: Latest resolution when version omitted
- **WHEN** a client installs a `remote` pack from `github.com/org/repo` with no version
- **THEN** the system resolves the latest release via the Releases API, persists the row's `version` as that release's concrete tag, and downloads that release's platform asset

#### Scenario: Checksum mismatch
- **WHEN** the downloaded archive's SHA-256 does not match the entry in `checksums.txt`
- **THEN** the install fails with an error and no DB record is created

#### Scenario: No asset for the current platform
- **WHEN** the resolved release contains no asset matching `*_<os>_<arch>.tar.gz`
- **THEN** the install fails with an error naming the `<os>/<arch>` and the resolved tag, and no DB record is created

#### Scenario: Multiple matching assets
- **WHEN** the resolved release contains more than one asset matching `*_<os>_<arch>.tar.gz`
- **THEN** the install fails with an error listing the candidate asset names, and no DB record is created

#### Scenario: Repository or release not found
- **WHEN** the GitHub Releases API returns not-found for the repo or the requested tag
- **THEN** the install fails with an error and no DB record is created

#### Scenario: Source format
- **WHEN** a client provides `source: "https://github.com/org/repo"` or `"github.com/org/repo"`
- **THEN** both forms are accepted (HTTP/HTTPS prefixes are stripped)

### Requirement: Upload Pack Install
The system SHALL accept binary uploads via `POST /api/packs/upload`
(multipart form). The system SHALL write the binary to a temporary location, run
its `manifest` command to derive the pack's identity (see "Install Derives
Identity From Manifest"), relocate the binary to
`<DataDir>/packs/<name>/upload/<binary>`, and persist the `source` as that path.
Runtime SHALL treat `upload` packs identically to `local` packs. An install SHALL
fail if the manifest command fails, and no DB record SHALL be created.

#### Scenario: Successful upload
- **WHEN** a client uploads a binary that returns a valid manifest as multipart form data
- **THEN** the binary is written to `<DataDir>/packs/<manifest-name>/upload/<binary>`, a `packs` row is created with `type = "upload"`, and the name/version come from the manifest

#### Scenario: Manifest failure rejects upload
- **WHEN** an uploaded binary's `manifest` command fails
- **THEN** the install fails with an error and no DB record is created

### Requirement: Install Is Idempotent By Name
The system SHALL upsert by the manifest-derived `name` on `POST /api/packs/install`
and `POST /api/packs/upload`. Subsequent installs that resolve to the same
manifest `name` replace the existing row's type, source, and version. Cached
binaries from previous installs SHALL NOT be cleaned up by the upsert.
**Note:** flagged — disk usage grows when versions change repeatedly.

#### Scenario: Reinstall replaces row
- **WHEN** a pack whose manifest reports name `p1` is installed at v1.0.0 then a newer release reporting the same name `p1` is installed at v2.0.0
- **THEN** the row's `version` is `2.0.0`, but the v1.0.0 binary remains under `<DataDir>/packs/p1/1.0.0/`

### Requirement: List Packs
The system SHALL return all packs from `GET /api/packs` ordered by name,
always as a JSON array.

#### Scenario: Listing
- **WHEN** a client requests `/api/packs`
- **THEN** the response is a name-sorted array of pack records

### Requirement: Delete By Name
The system SHALL delete packs by name on `DELETE /api/packs/{name}`,
returning HTTP 204 idempotently. Cached binaries on disk SHALL NOT be
removed. **Note:** flagged as orphaned-disk-content behavior.

#### Scenario: Delete keeps binary
- **WHEN** a pack with cached binary at `<DataDir>/packs/p/1.0.0/p` is deleted
- **THEN** the DB row is removed, the response is 204, and the cached binary is left in place

### Requirement: Manifest Endpoint
The system SHALL invoke the pack's `manifest` command via stdin/stdout,
passing current parameters as `{"parameters": {...}}`, and return the
parsed manifest from `GET /api/packs/{name}/manifest`. If the pack
binary is not on disk, the request SHALL fail with an error indicating
the binary was not found.

#### Scenario: Manifest fetched
- **WHEN** a client requests the manifest for an installed pack
- **THEN** the response is the manifest JSON parsed from the pack binary's stdout

#### Scenario: Binary missing
- **WHEN** the cached/local binary path no longer exists
- **THEN** the response is an error with a "binary not found" message

### Requirement: Pack Parameters Storage
The system SHALL store a pack's parameters as a JSONB map. Storage
itself SHALL accept arbitrary JSON values; validation is enforced at
the API layer against the pack's declared schema, not at the column
level. **Note:** invalid Terraform variable names will fail at run time
when passed as `TF_VAR_*` env vars.

#### Scenario: Storage accepts arbitrary JSON
- **WHEN** a row is updated with `{ "region": "us-east-1", "size": 3 }`
- **THEN** the row's `parameters` JSONB stores the values verbatim

### Requirement: Get and Update Parameters
The system SHALL respond to `GET /api/packs/{name}/parameters` with the
current `parameters` JSONB and to `PUT /api/packs/{name}/parameters`
with a full replacement (no merge). Replacement SHALL drop keys absent
from the request. On `PUT`, the system SHALL fetch the pack's
`params_schema` (via the manifest command) and SHALL strict-validate
every request key that matches a declared schema property: declared
keys SHALL pass type check, enum membership check, and the required-key
check. Unknown keys (present in the request but absent from the
schema) SHALL be persisted alongside declared keys without rejection.
The response body SHALL include both the persisted parameters and a
list of unknown keys so the client can render a soft warning.

#### Scenario: Replace not merge
- **WHEN** the existing parameters are `{a:1,b:2}` and a client PUTs `{a:3}`
- **THEN** the persisted parameters are `{a:3}` (key `b` is removed)

#### Scenario: Strict validation rejects type mismatch on declared key
- **WHEN** the pack declares `aws_region` as a string and a client
  PUTs `{"aws_region": 5}`
- **THEN** the request is rejected with a structured validation error
  naming `aws_region` and the expected type

#### Scenario: Strict validation rejects enum violation
- **WHEN** the pack declares `aws_region` with `enum: ["us-east-1", "us-west-2"]`
  and a client PUTs `{"aws_region": "eu-west-9"}`
- **THEN** the request is rejected with a structured validation error
  naming `aws_region` and listing the allowed values

#### Scenario: Strict validation rejects missing required custom param
- **WHEN** the pack declares `vpc_id` with `required: true` and a
  client PUTs a body without `vpc_id`
- **THEN** the request is rejected with a structured validation error
  naming `vpc_id`

#### Scenario: Unknown keys are kept and surfaced
- **WHEN** the pack's schema lists `aws_region` and the client PUTs
  `{"aws_region": "us-east-1", "legacy_key": "x"}`
- **THEN** the persisted parameters contain both keys, and the
  response body includes `"unknown_keys": ["legacy_key"]`

#### Scenario: Pack with no schema falls back to permissive storage
- **WHEN** a pack's manifest returns no `params_schema` and a client
  PUTs `{"anything": "goes"}`
- **THEN** the request is accepted, the value is persisted verbatim,
  and the response's `unknown_keys` list contains every key in the
  request

### Requirement: Source Format Validation
The system SHALL validate `remote` source against the
`github.com/<org>/<repo>` form (with optional `http://` / `https://`
prefix stripped). Any other source format SHALL fail the install. The
`local` and `upload` source values SHALL be accepted
verbatim without URL-format validation.

#### Scenario: Bad remote source rejected
- **WHEN** a client posts `type: "remote", source: "not-a-github-path"`
- **THEN** the install fails with a parse error

### Requirement: User Attribution
The system SHALL set `installed_by` from the session email at install
and upload time. When auth is disabled it SHALL be the empty string.

#### Scenario: Authenticated install
- **WHEN** an authenticated user installs a pack
- **THEN** the row's `installed_by` is the session email

### Requirement: Concurrent Pack Operations Are Serialized Per Pack
The system SHALL serialize mutating filesystem operations on a single
pack's cache directory so that concurrent operations cannot corrupt,
truncate, or remove a binary mid-write. Operations that SHALL be
serialized per pack name are: remote download/extract, upload binary
write, and delete cleanup. The
serialization SHALL be process-global (shared across all runner-factory
and resolver instances, which are constructed per request/detonation),
keyed by pack name. Operations on different pack names SHALL remain
concurrent.

A cache-hit read (the binary already exists on disk) SHALL NOT acquire
the lock. When an operation acquires the lock to perform a write, it
SHALL re-check the cache inside the critical section and reuse an
existing binary rather than re-download or re-install it.

#### Scenario: Concurrent installs of the same pack do not corrupt the cache
- **WHEN** two requests install the same remote pack at the same version concurrently
- **THEN** the downloads are serialized, the second request observes the cached binary written by the first, and the cached binary is complete and runnable

#### Scenario: Concurrent installs of different packs run in parallel
- **WHEN** two requests install two differently-named packs concurrently
- **THEN** neither blocks the other and both complete

#### Scenario: Delete cannot interleave with an in-progress install of the same pack
- **WHEN** a delete of pack `p` is requested while an install of pack `p` is writing its cache directory
- **THEN** the delete waits for the install's critical section to complete (or the install waits for the delete) and never removes a half-written directory out from under the other operation

#### Scenario: Cache hit is lock-free
- **WHEN** a pack binary is already present on disk and a runner resolves it
- **THEN** resolution returns the cached path without acquiring the per-pack lock
