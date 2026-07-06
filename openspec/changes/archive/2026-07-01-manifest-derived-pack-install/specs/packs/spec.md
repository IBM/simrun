## MODIFIED Requirements

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

## ADDED Requirements

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
