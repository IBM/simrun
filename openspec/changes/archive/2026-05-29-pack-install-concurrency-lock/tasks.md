## 1. Keyed-lock helper

- [x] 1.1 Create `simrun/internal/packs/locks/keyedmutex.go` with a package-global keyed mutex backed by `sync.Map[string]*sync.Mutex` and an `Acquire(key string) (release func())` function that loads-or-stores the mutex, locks it, and returns an unlock closure
- [x] 1.2 Add `keyedmutex_test.go` verifying: (a) same key serializes (concurrent goroutines never overlap inside the critical section), (b) different keys run concurrently, (c) `release` unlocks so the next `Acquire(sameKey)` proceeds

## 2. Guard the resolver download path

- [x] 2.1 In `resolver.Resolve`, keep the initial lock-free `isCached` fast path; when a download is needed, acquire `locks.Acquire(cfg.Name)`, then re-check the cache inside the critical section (double-checked locking) before calling `download`, releasing via `defer`
- [x] 2.2 Add a resolver test that runs two concurrent `Resolve` calls for the same pack against a stub HTTP server and asserts the archive is downloaded once and the resulting binary is complete

## 3. Guard the go-remote install path

- [x] 3.1 ~~In `factory.resolveGoRemote`, ... run the `go install` / rename / cleanup block under the lock~~ — **N/A: obsolete.** The go-remote install path was removed in commit 9103f6b (`refactor: drop go-remote pack install mode`); the factory no longer runs `go install`. Remote packs now resolve through `resolver.Resolve`, which is already guarded by task 2.1.

## 4. Guard the upload and delete handlers

- [x] 4.1 In `web.HandleUploadPack`, acquire `locks.Acquire(name)` around the binary write + `manifest` validation + DB upsert so a concurrent upload/delete of the same name cannot interleave
- [x] 4.2 In `web.HandleDeletePack`, acquire `locks.Acquire(name)` around the `os.RemoveAll` cleanup so it cannot run concurrently with an install/upload of the same pack

## 5. Verify

- [x] 5.1 Run `go build ./...` and `go test ./simrun/internal/packs/... ./simrun/internal/web/...`
- [x] 5.2 Run `go test -race` on the packs and web packages to confirm the new lock has no data races and serializes as designed
