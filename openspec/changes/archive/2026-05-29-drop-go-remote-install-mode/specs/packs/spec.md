## MODIFIED Requirements

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
- **WHEN** a client posts `POST /api/packs/install` with `type: "go-remote"`
- **THEN** the install is rejected with a validation error naming the allowed types (`local`, `remote`, `upload`) and no `packs` row is created

### Requirement: Source Format Validation
The system SHALL validate `remote` source against the
`github.com/<org>/<repo>` form (with optional `http://` / `https://`
prefix stripped). Any other source format SHALL fail the install. The
`local` and `upload` source values SHALL be accepted verbatim without
URL-format validation.

#### Scenario: Bad remote source rejected
- **WHEN** a client posts `type: "remote", source: "not-a-github-path"`
- **THEN** the install fails with a parse error

## REMOVED Requirements

### Requirement: Go-Remote Pack Install
**Reason**: `go-remote` required a full Go toolchain on the server,
offered no supply-chain guarantees (no checksum verification), and
relied on a deferred-install path whose failures surfaced only at
scenario run time. It is redundant with `remote` (checksum-verified
release artifacts for distribution) and `upload`/`local` (for local
development).

**Migration**: None. `go-remote` was never used in any deployment, so
no installed packs need reinstalling. Going forward, distribute packs as
checksum-verified GitHub releases (`type: "remote"`) or build them
locally (`type: "upload"` / `type: "local"`).
