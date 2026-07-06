## Why

Installing a pack is unnecessarily hard. The single `name` field is overloaded
as three different things — the GitHub release artifact filename prefix
(`{name}_{version}_{os}_{arch}.tar.gz`), the binary name inside the tarball, and
the simrun DB/display identifier. Because a pack repo's goreleaser project/binary
name often differs from its repo name, the operator has to reverse-engineer the
exact artifact name to make the source and name line up. On top of that, `version`
is required and must match a release exactly, with no way to ask for "latest".

## What Changes

- **BREAKING**: Remove `name` from the install flow. `POST /api/packs/install`
  ignores any `name` in the request; the install dialog drops the name field
  entirely. The pack's identity comes from its own manifest instead.
- Install becomes a real operation (today it only writes metadata). For every
  pack type, install now makes the binary available, runs the pack's `manifest`
  command, and persists the row using the manifest's `pack.name` and
  `pack.version`. Failures (bad repo, missing artifact, checksum mismatch,
  manifest error, `min_simrun_version` mismatch) surface at install time, not at
  first run.
- **Remote**: `version` is now optional. Resolution goes through the GitHub
  Releases API — `/releases/latest` when version is empty, otherwise
  `/releases/tags/v{version}`. The platform artifact is selected from the
  release's asset list by matching `*_{os}_{arch}.tar.gz` (so the artifact prefix
  no longer has to equal anything the operator types), checksum-verified against
  `checksums.txt`, extracted, and the resolved concrete tag is pinned into the
  stored row.
- **Local**: install now verifies the path exists and runs its manifest at
  install time (dropping today's "install can succeed with a non-existent path"
  behavior). The binary is still referenced in place, not copied.
- **Upload**: the uploaded binary's manifest is run to derive name + version; the
  binary is relocated under `<DataDir>/packs/{name}/upload/`.
- Remote support stays public-GitHub-only and anonymous (no token). GitHub
  Releases API `latest` lookups are rate-limited to 60/hr per IP unauthenticated,
  which is acceptable for occasional installs.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `packs`: Rework the install lifecycle. Modify "Remote Pack Install"
  (API-based resolution, optional version → latest, manifest-derived name,
  install-time download + verify), "Local Pack Install" (verify + run manifest at
  install; remove the non-existent-path note), "Upload Pack Install"
  (manifest-derived name + version), and "Install Is Idempotent By Name" (name is
  now manifest-derived). Add scenarios for latest resolution, no-asset-for-platform,
  install-fails-on-bad-repo, and manifest-derived name. "Source Format Validation"
  is unchanged.

## Impact

- **Backend**:
  - `internal/packs/resolver/resolver.go` — add GitHub Releases API resolution and
    asset matching; drop the hand-built artifact filename; `version` no longer
    required; derive the in-tarball binary name from the chosen asset.
  - `internal/web/packs_handler.go` — `HandleInstallPack` and the upload handler
    move to an eager resolve → run-manifest → derive-name → upsert pipeline.
  - `internal/web/types.go` — `InstallPackRequest.Name` becomes optional/ignored.
- **Frontend**: `web/frontend/src/routes/packs/+page.svelte` — remove the name
  field; type-dependent inputs (remote = source + optional version, local = path,
  upload = file).
- **API**: `POST /api/packs/install` no longer requires `name`; remote `version`
  optional; install can now fail with download/manifest errors.
- **Runtime**: unchanged — `Factory.CreateRunner` → `Resolver.Resolve` stays
  cache-keyed by `name`+`version` and hits the cache populated at install.
- **Source of truth**: `pack/protocol.go` `PackInfo.Name`/`Version` already exist.
- **Specs**: `openspec/specs/packs/spec.md`.
- **Dependencies**: none added (uses the existing HTTP client against the public
  GitHub API).
