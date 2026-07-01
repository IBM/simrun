## 1. Resolver: GitHub Releases API + manifest-driven resolution

- [x] 1.1 Add a GitHub Releases API client call in `internal/packs/resolver/resolver.go`: `GET /repos/<org>/<repo>/releases/latest` when version is empty, else `/repos/<org>/<repo>/releases/tags/v<version>`; return the concrete tag and the asset list (name + browser_download_url). Map not-found and rate-limit responses to clear errors.
- [x] 1.2 Replace `buildDownloadURLs` (hand-built `{name}_{version}_{os}_{arch}.tar.gz`) with asset selection from the release: pick the asset matching `*_<GOOS>_<GOARCH>.tar.gz`; error on 0 matches (name os/arch + tag) and on >1 matches (list candidates). Locate the `checksums.txt` asset from the same list.
- [x] 1.3 Update `validatePackConfig`: remove the "version required" check; keep `source` required for remote.
- [x] 1.4 Update `extractTarGz` / extraction: derive the expected in-tarball binary name from the selected asset's filename prefix; fall back to the single executable in the archive; error if neither resolves. Cache at `<DataDir>/packs/<name>/<tag>/<binary>` using a temp-stage-then-relocate flow so the path can use the manifest-derived name.
- [x] 1.5 Keep checksum verification against `checksums.txt` for the selected asset.

## 2. Install pipeline: resolve → manifest → derive identity → upsert

- [x] 2.1 Add a shared helper (e.g. in `internal/packs/` or the handler) that, given an available binary path, runs the pack `manifest` command and returns `pack.name` / `pack.version` (`pack/protocol.go` `PackInfo`). Surface manifest failures as install errors. (Reused existing `resolver.GetManifest`.)
- [x] 2.2 Rework `HandleInstallPack` in `internal/web/packs_handler.go` into an eager pipeline: for `remote` download+verify+extract to temp; for `local` verify the path exists; then run the manifest, derive name+version, relocate (remote), and upsert. No DB row on any failure.
- [x] 2.3 Rework the upload handler: write the uploaded binary to temp, run the manifest, relocate to `<DataDir>/packs/<name>/upload/<binary>`, upsert with manifest name+version; fail without a row on manifest error.
- [x] 2.4 Make `InstallPackRequest.Name` in `internal/web/types.go` optional/ignored; ensure the type validation (local/remote/upload) still runs.
- [x] 2.5 Confirm `Factory.CreateRunner` → `Resolver.Resolve` still finds the install-time cached binary by `name`+`version` (cache hit, no re-download); adjust only if the relocate path changed the cache key. (No change: relocate path matches the `cachedBinary` scan dir.)

## 3. Frontend install dialog

- [x] 3.1 In `web/frontend/src/routes/packs/+page.svelte` remove the name input and make inputs type-dependent: remote = source + optional version, local = absolute path, upload = file picker. (Per decision: dialog offers remote + upload only; no local option.)
- [x] 3.2 Update the `installPack` client call / payload to stop sending `name`; surface install-time errors (bad repo, no asset, checksum, manifest) in the existing error UI.

## 4. Tests

- [x] 4.1 Resolver tests: latest resolution (empty version → concrete tag), tag resolution, asset matching (0 / 1 / >1 matches), checksum mismatch, repo/tag not found, binary-name derivation from asset prefix and single-executable fallback.
- [x] 4.2 Install handler tests: name ignored from request and taken from manifest; version pinned from resolved tag (remote) and manifest (local/upload); no DB row on download/manifest failure; local non-existent path rejected.
- [x] 4.3 Run `go test ./...` and `mise run lint`; build with `mise run build`.

## 5. Spec sync

- [ ] 5.1 After implementation, apply the spec delta to `openspec/specs/packs/spec.md` (handled at archive time) and confirm scenarios match the shipped behavior.
