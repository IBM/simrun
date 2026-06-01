## Why

Pack install/resolve/delete operations share an on-disk cache directory
(`<DataDir>/packs/<name>/<version>/`) but have zero concurrency control.
In a multi-user web app, two concurrent installs of the same pack write
the same files with `O_TRUNC`, and a delete or re-upload during an
active run can yank the binary out from under a running detonation. The
result is torn binaries, half-extracted archives, and
`ETXTBSY`/"binary not found" failures that are intermittent and hard to
reproduce.

## What Changes

- Introduce a per-pack named lock that serializes mutating filesystem
  operations on a pack's cache directory (remote download/extract,
  upload write, and delete cleanup).
- The lock is **process-global** keyed by pack name, because the runner
  factory and resolver are constructed fresh per HTTP request and per
  detonation (4 call sites) — an instance-scoped lock would not protect
  across those independent instances.
- Cache-hit reads (the common path when a binary is already present)
  stay lock-free; only the install/delete/extract critical sections are
  guarded.
- No API surface, request/response shape, or DB schema changes.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `packs`: add a requirement that concurrent install/delete operations
  on the same pack are serialized so they cannot corrupt or remove a
  shared cache directory mid-write.

## Impact

- Code: `simrun/internal/packs/resolver/resolver.go` (download/extract
  critical section), `simrun/internal/web/packs_handler.go`
  (`HandleUploadPack`, `HandleDeletePack`). A small new package-level
  keyed-lock helper.
- Behavior: concurrent operations on the *same* pack name now run
  sequentially; operations on *different* packs remain fully parallel.
- No dependencies, APIs, or migrations affected.
