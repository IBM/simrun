## Why

`go-remote` is a redundant pack install mode that sits awkwardly between the two modes that actually matter: `remote` (checksum-verified GitHub release artifacts — the right answer for distribution) and `upload`/`local` (the right answer for local development). `go-remote` installs packs by shelling out to `go install <module>@<version>`, which requires a full Go toolchain on the server, provides no supply-chain guarantees (no checksum verification), and adds a deferred-install code path whose failures only surface at scenario run time. Removing it shrinks the surface area and forces packs through the verified distribution path.

## What Changes

- **BREAKING**: Remove `go-remote` as a recognized pack type. The system will accept only `local`, `remote`, and `upload`.
- Remove the `PackTypeGoRemote` constant and `IsGoRemote()` helper from config.
- Remove the `resolveGoRemote()` runner-factory path (the `go install` shell-out) and its `case config.PackTypeGoRemote` branch.
- Reject `POST /api/packs/install` requests with `type: "go-remote"` at install time with a clear error, instead of accepting them and failing later at run time.
- Remove `go-remote` from the frontend: install-dialog type dropdown, `Pack.type` TypeScript union, and the `PackCard` badge variant switch.

`go-remote` has never been used in any deployment, so there are no installed packs to migrate and no backward-compatibility handling is needed.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `packs`: Drop the "Go-Remote Pack Install" requirement; change "Recognized Pack Types" from four types to three; update "Source Format Validation" to stop referencing `go-remote` verbatim-source handling and to reject `go-remote` installs.

## Impact

- **Backend code**:
  - `simrun/internal/config/config.go` — remove `PackTypeGoRemote`, `IsGoRemote()`, doc comments.
  - `simrun/internal/packs/runner/factory.go` — remove `case config.PackTypeGoRemote` and the ~82-line `resolveGoRemote()` function.
  - `simrun/internal/web/packs_handler.go` — reject `go-remote` in the install handler.
- **API**: `POST /api/packs/install` now rejects `type: "go-remote"` (previously accepted).
- **Frontend**: `web/frontend/src/routes/packs/+page.svelte`, `web/frontend/src/lib/components/PackCard.svelte`, `web/frontend/src/lib/types/index.ts`.
- **Data**: `packs.type` is `TEXT` with no enum constraint, so no schema migration is required. No `go-remote` rows exist (the type was never used), so no data cleanup is needed.
- **Specs**: `openspec/specs/packs/spec.md`.
- **Dependencies**: removes the runtime requirement for a Go toolchain on the server.
