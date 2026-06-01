## ADDED Requirements

### Requirement: Concurrent Pack Operations Are Serialized Per Pack
The system SHALL serialize mutating filesystem operations on a single
pack's cache directory so that concurrent operations cannot corrupt,
truncate, or remove a binary mid-write. Operations that SHALL be
serialized per pack name are: remote download/extract, upload binary
write, and delete cleanup. The
serialization SHALL be process-global (shared across all runner-factory
and resolver instances, which are constructed per request/detonation),
keyed by pack name. Operations on different pack names SHALL remain
concurrent.

A cache-hit read (the binary already exists on disk) SHALL NOT acquire
the lock. When an operation acquires the lock to perform a write, it
SHALL re-check the cache inside the critical section and reuse an
existing binary rather than re-download or re-install it.

#### Scenario: Concurrent installs of the same pack do not corrupt the cache
- **WHEN** two requests install the same remote pack at the same version concurrently
- **THEN** the downloads are serialized, the second request observes the cached binary written by the first, and the cached binary is complete and runnable

#### Scenario: Concurrent installs of different packs run in parallel
- **WHEN** two requests install two differently-named packs concurrently
- **THEN** neither blocks the other and both complete

#### Scenario: Delete cannot interleave with an in-progress install of the same pack
- **WHEN** a delete of pack `p` is requested while an install of pack `p` is writing its cache directory
- **THEN** the delete waits for the install's critical section to complete (or the install waits for the delete) and never removes a half-written directory out from under the other operation

#### Scenario: Cache hit is lock-free
- **WHEN** a pack binary is already present on disk and a runner resolves it
- **THEN** resolution returns the cached path without acquiring the per-pack lock
