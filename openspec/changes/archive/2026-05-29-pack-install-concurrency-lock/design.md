## Context

Pack binaries are cached on disk under `<DataDir>/packs/<name>/<version>/<name>`.
Three code paths mutate or remove this tree, all without coordination:

- `resolver.Resolve` → `download` → `extractAndCache` writes the binary
  with `os.OpenFile(..., O_CREATE|O_WRONLY|O_TRUNC, 0755)` (remote packs).
- `web.HandleUploadPack` writes `<DataDir>/packs/<name>/upload/<name>`
  with `O_TRUNC`, then validates by running `manifest`.
- `web.HandleDeletePack` `os.RemoveAll`s the upload dir.

Critically, **none of these share an object instance**. `runner.NewFactory`
and `resolver.NewResolver` are called fresh at several sites
(`packs_handler.go`, `simrun_detonator.go`, `parser/main.go`), once per
HTTP request and once per detonation. There is no singleton resolver, hub,
or registry through which a lock could be threaded without a wider refactor.

Concurrent failure modes today:
- Two installs of the same remote pack: both download into memory, both
  `O_TRUNC`-write the same path → a reader can observe a torn binary.
- Delete or re-upload while a detonation has the binary open → `ETXTBSY`
  on exec or a vanished file mid-run.

## Goals / Non-Goals

**Goals:**
- Serialize mutating filesystem operations on a single pack's cache dir
  (remote download/extract, upload write, delete cleanup) so they cannot
  interleave and corrupt each other.
- Keep operations on *different* packs fully concurrent.
- Keep the cache-hit fast path (binary already present) lock-free.
- Minimal, surgical change — no new dependencies, no API/DB changes, no
  refactor of how factories/resolvers are constructed.

**Non-Goals:**
- Fully preventing deletion of a binary that an in-flight detonation has
  *already resolved and is executing*. Guarding that would require holding
  a read-lock across the entire run lifetime (a reader/writer scheme
  threaded through the detonator), which is a larger change. We document
  this residual risk rather than solve it here.
- Cross-process / multi-replica coordination (e.g. file locks). simrun is
  a single binary; in-process locking is sufficient for the current
  deployment.
- Changing checksum/verification or install semantics.

## Decisions

### Decision 1: Package-global keyed lock, not an instance field

The user's framing was "sync.Map of mutexes keyed by name." That is the
right primitive, but it **must live at package scope**, not as a field on
`Resolver` or `Factory`. Because those structs are reconstructed per
request/detonation, an instance-scoped `sync.Map` would always start empty
and two concurrent requests would hold two different maps → no mutual
exclusion. A package-level `var` shared by all callers is the only way the
lock is observed across the independent instances.

Introduce a tiny helper (e.g. `simrun/internal/packs/locks`, a
`keyedmutex` type) exposing:

```go
func Acquire(key string) (release func())
```

backed by `sync.Map[string]*sync.Mutex`. `Acquire` loads-or-stores the
mutex for `key`, locks it, and returns an unlock closure. This is the
standard Go keyed-mutex pattern; no third-party dep.

**Alternative considered — make the resolver a singleton** and hang the
map off it. Rejected: requires plumbing one resolver instance through the
web server, detonator, and parser construction — exactly the kind of wide
refactor Rule 3 (surgical changes) tells us to avoid for a targeted fix.

### Decision 2: Lock key = pack name

Keying by `name` (not `name/version`) is the conservative choice and
matches the user's ask. Upload and delete operate at the `<name>/` level
(`<name>/upload`, and a future full-tree delete), so version-scoped keys
would let a delete of `<name>` race a version install. Name-level keying
makes all operations on a pack mutually exclusive at the cost of
serializing installs of two *different versions* of the same pack — a rare
and cheap-to-serialize case. We accept the coarser granularity for
correctness simplicity.

### Decision 3: Lock only the mutate sections, leave cache-hit reads free

`Resolve` checks `isCached` first; on a hit it returns immediately with no
lock. Only when a download is needed do we acquire the lock, then
**re-check the cache inside the critical section** (double-checked
locking) so a pack downloaded by the request we waited on is reused
instead of re-downloaded.

### Decision 4: Where each call site acquires

- `resolver.Resolve`: acquire around the `download` branch (after the
  initial cache check), re-check cache inside.
- `HandleUploadPack`: acquire around the binary write + manifest-validate
  + DB upsert so a concurrent delete/upload of the same name can't
  interleave with validation.
- `HandleDeletePack`: acquire around the `RemoveAll` cleanup.

All releases via `defer`.

## Risks / Trade-offs

- **Residual race: delete vs. an already-executing detonation** → Out of
  scope (see Non-Goals). The lock prevents install/delete from corrupting
  each other and from clobbering a download in progress, which is the
  "worst class of bug." Killing the execute-time race needs a run-scoped
  read lock and is deferred. Documented, not silently ignored (Rule 11).
- **Coarser name-level key serializes different versions of one pack** →
  Acceptable; multi-version concurrent installs of the same pack are rare,
  and serializing them is cheap.
- **Unbounded growth of the mutex map** → Negligible: one `*sync.Mutex`
  per distinct pack name ever installed; bounded by the pack catalog size.
  Not worth the complexity of reference-counted eviction.
- **Lock held across the network download** → This can be slow, so a
  second concurrent install of the same pack waits for the first. This is
  intended (it dedupes the work) and only affects same-pack concurrency.

## Open Questions

- None blocking. If the execute-time delete race proves to bite in
  practice, a follow-up change can promote the keyed mutex to a keyed
  `sync.RWMutex` and have detonations hold the read lock for the run
  duration.
