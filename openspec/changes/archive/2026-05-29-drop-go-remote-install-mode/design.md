## Context

The pack install system supports four types defined in
`simrun/internal/config/config.go`: `local`, `remote`, `go-remote`,
`upload`. All four resolve to a local binary path and run via the same
`BinaryRunner`. They differ only in how that binary is sourced:

- `local` / `upload` — path already on disk, used directly.
- `remote` — downloaded from a GitHub release, SHA-256-verified against
  `checksums.txt`, cached under `<DataDir>/packs/<name>/<version>/`.
- `go-remote` — `go install <source>@v<version>` shelled out at first
  use (`factory.go:resolveGoRemote`, ~82 lines), cached the same way.

The install handler (`POST /api/packs/install`) does not validate
`type`; it stores whatever string it receives. Type validity is only
checked at run time inside `Factory.CreateRunner`'s switch, so an
invalid type install "succeeds" and fails later when a scenario runs.

`packs.type` is a plain `TEXT` column (migration `001_initial.up.sql`),
so the stored value is unconstrained at the DB level.

## Goals / Non-Goals

**Goals:**
- Remove `go-remote` from the type enum, the runner factory, and the
  frontend.
- Reject `go-remote` (and any unrecognized type) at install time rather
  than at run time.

**Non-Goals:**
- No DB schema migration or data backfill (column stays `TEXT`).
- No backward-compatibility handling for old rows — `go-remote` was
  never used, so none exist.
- No change to `remote`, `upload`, or `local` behavior.
- No new abstraction over install modes.

## Decisions

**1. Reject unrecognized types at the install handler, not only the factory.**
Today an unknown type is accepted and fails at run time (Rule 11 — fail
loud violated). Add a small allow-list check in
`HandleInstallPack` (`packs_handler.go`) that returns HTTP 400 for any
type outside `{local, remote, upload}`. This makes the failure immediate
and is the natural place to enforce the now-narrower enum. The factory's
`default` branch stays as defense-in-depth.
*Alternative considered:* a DB CHECK constraint — rejected because the
column is intentionally unconstrained `TEXT`; keeping it that way avoids
a schema migration for what is purely an application-level enum.

**2. No data migration.**
`go-remote` was never used, so no `packs` rows carry that type. The
column stays unconstrained `TEXT`; nothing to delete or rewrite. The
factory's `default` branch remains as defense-in-depth but is not
expected to ever fire for `go-remote`.

**3. Delete `resolveGoRemote` and its case branch wholesale.**
No callers remain once the enum constant is gone. This also removes the
server's runtime dependency on a Go toolchain. The `pack-install-concurrency-lock`
change references `go-remote` in its spec prose; that is a separate
in-flight change and will be reconciled there, not here.

**4. Frontend: drop the option and the union member.**
Remove `'go-remote'` from the install dialog `typeOptions`, the
`Pack.type` TypeScript union, and the `PackCard` `typeVariant` switch
(the switch keeps its `default` fallback).

## Risks / Trade-offs

- **External callers of the API using `go-remote`** → Now get a 400
  instead of a deferred run-time failure. This is the intended, more
  honest behavior (BREAKING, documented in proposal). Since the type was
  never used, no real caller is expected to be affected.
- **Merge overlap with `pack-install-concurrency-lock`** → That change
  mentions `go-remote` in its serialization list. Whichever lands
  second drops the reference. Low risk; flag at apply time.

## Migration Plan

1. Backend: remove enum constant + helper, factory case + function, add
   install-handler validation.
2. Frontend: remove option, union member, badge case.
3. Update `openspec/specs/packs/spec.md` via the delta at archive time.
4. Rollback: revert the commit; no data changes were made.

## Open Questions

- None.
