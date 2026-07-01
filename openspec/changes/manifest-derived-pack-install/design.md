## Context

Today `POST /api/packs/install` is metadata-only: it validates the type and
upserts a row. The binary is never touched until a scenario runs, when
`Factory.CreateRunner` calls `Resolver.Resolve`, which (for remote packs)
hand-builds the artifact URL from the operator-supplied `name` and `version`:

```
archiveName = "{name}_{version}_{GOOS}_{GOARCH}.tar.gz"
url         = "https://github.com/{org}/{repo}/releases/download/v{version}/{archiveName}"
extract     = file whose base name == name
cache       = <DataDir>/packs/{name}/{version}/{name}
```

This couples three independent things to one `name` field and requires an exact
`version`. The pack's own `manifest` command already reports the authoritative
identity (`pack.name`, `pack.version` in `pack/protocol.go`), and the GitHub
Releases API already exposes the real asset names and the latest tag — both are
better sources of truth than operator input.

## Goals / Non-Goals

**Goals:**
- Operator installs a remote pack by pasting only `github.com/org/repo`, with
  version optional (empty → latest).
- Pack identity (`name`, `version`) is derived from the pack's manifest, never
  typed by the operator, for all three pack types.
- Install failures (bad repo, no platform asset, checksum mismatch, manifest
  error) surface at install time, loudly.
- Runtime resolution is unchanged: the binary cached at install is found by the
  existing `name`+`version` cache key.

**Non-Goals:**
- Private repositories / authenticated GitHub access (no `GITHUB_TOKEN`).
- A "check for updates / re-pin to latest" lifecycle action — out of scope.
- Cleaning up stale cached binaries from prior versions (existing flagged
  behavior, untouched).
- Changing the runtime parameter-injection / terraform path.

## Decisions

### 1. Pin "latest" at install time, not run time
Install resolves an empty version to a concrete release tag via the GitHub
Releases API and stores that concrete tag in the row. Runs are therefore
reproducible and the version-keyed cache path stays valid.
**Alternative considered:** resolve "latest" lazily at each run. Rejected — the
pack would silently drift between runs and the cache key for "latest" is
ill-defined.

### 2. Derive name (and version) from the pack manifest for all types
After the binary is on disk, install runs its `manifest` command and uses
`pack.name` / `pack.version` as the persisted identity. The `name` field is
removed from the dialog and ignored in `InstallPackRequest`.
**Alternative considered:** default `name` to the repo name. Rejected — the
manifest is the authoritative identity and unifies all three types behind one
"resolve binary → manifest → upsert" path.

### 3. Resolve remote artifacts via the GitHub Releases API
`GET /releases/latest` (empty version) or `/releases/tags/v{version}` returns the
tag plus the asset list. The platform archive is the asset whose name matches
`*_{GOOS}_{GOARCH}.tar.gz`; the in-tarball binary name is derived from that
asset's filename prefix (and, if the archive contains a single executable, that
file is taken). This removes the artifact-prefix coupling entirely.
**Alternative considered:** keep building the URL by hand but add a separate
`artifact_name` field. Rejected — still makes the operator know the goreleaser
project name; the API already has the answer.

### 4. Install becomes a network/manifest operation with a temp-stage step
Because the name is unknown until the manifest runs, remote/upload download or
write the binary to a temp location, run the manifest, then relocate to the
canonical path (`packs/{name}/{tag}/{binary}` for remote, `packs/{name}/upload/`
for upload). Local references the binary in place and only runs its manifest.

### 5. Upsert-by-name semantics unchanged
The manifest-derived `name` remains the unique DB key. Two different sources that
report the same `pack.name` overwrite each other, exactly as today's name-keyed
upsert does. Not special-cased.

## Risks / Trade-offs

- **GitHub API rate limit (60/hr per IP, unauthenticated)** → Acceptable for
  occasional installs; surface a clear error if a `latest` lookup is rate-limited
  so the operator can retry or pin an explicit version.
- **Local install now requires a runnable binary at install time** (drops the
  "succeeds with a non-existent path" behavior) → This is intentional fail-loud
  behavior; documented as a breaking change in the spec delta.
- **Install can now fail for many new reasons** → Each failure mode returns a
  specific, actionable error (repo not found, no asset for `{os}/{arch}` in
  `{tag}`, >1 matching asset listing candidates, checksum mismatch, manifest
  error). No partial DB row is created on failure.
- **Ambiguous in-tarball binary name** if a release deviates from the goreleaser
  convention → Derive from the asset prefix first; fall back to the single
  executable in the archive; error if neither resolves.
