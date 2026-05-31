## 1. Backend — config

- [x] 1.1 Remove the `PackTypeGoRemote` constant from `simrun/internal/config/config.go` and its doc-comment line in the `PackConfig` type comment.
- [x] 1.2 Remove the `IsGoRemote()` helper method.

## 2. Backend — runner factory

- [x] 2.1 Remove the `case config.PackTypeGoRemote` branch from `Factory.CreateRunner` in `simrun/internal/packs/runner/factory.go`.
- [x] 2.2 Delete the `resolveGoRemote()` function and any imports (e.g. `os/exec`) that become unused after its removal.

## 3. Backend — install handler validation

- [x] 3.1 In `simrun/internal/web/packs_handler.go` `HandleInstallPack`, reject any `type` outside `{local, remote, upload}` with HTTP 400 and a message naming the allowed types, before persisting.
- [x] 3.2 Add/extend a handler test in `simrun/internal/web/api_packs_test.go` asserting `type: "go-remote"` install returns 400 and creates no row.

## 4. Frontend

- [x] 4.1 Remove the `'go-remote'` entry from `typeOptions` in `web/frontend/src/routes/packs/+page.svelte`.
- [x] 4.2 Remove `'go-remote'` from the `Pack.type` union in `web/frontend/src/lib/types/index.ts`.
- [x] 4.3 Remove the `case 'go-remote'` from `typeVariant` in `web/frontend/src/lib/components/PackCard.svelte` (keep the existing `default` fallback).

## 5. Verification

- [x] 5.1 Run `go build ./...` and `go test ./...`; confirm no references to `go-remote`/`GoRemote`/`resolveGoRemote` remain (`grep -rni "go-remote\|goremote" simrun web/frontend`).
- [x] 5.2 Build the frontend (`mise run build-frontend`) and confirm the install dialog shows only Remote and Upload options.
- [x] 5.3 Update `openspec/specs/packs/spec.md` per the delta (done at archive time via `/opsx:archive`).
