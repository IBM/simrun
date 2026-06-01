## 1. Schema removal and parser regeneration

- [x] 1.1 Delete `simrun/schemas/localDetonator.schema.json`.
- [x] 1.2 Delete `simrun/schemas/remoteDetonator.schema.json`.
- [x] 1.3 In `simrun/schemas/simrun.schema.json`, remove the two enum entries (`"localDetonator"`, `"remoteDetonator"`) from the detonate `oneOf` discriminator and remove their `properties.localDetonator` / `properties.remoteDetonator` `$ref` entries.
- [x] 1.4 Run `mise run parser` to regenerate `simrun/internal/parser/parser.go`. Verify the generated file no longer declares types referencing `LocalDetonator` or `RemoteDetonator`.

## 2. Go source removal

- [x] 2.1 Delete `simrun/internal/detonators/local_command_detonator.go`.
- [x] 2.2 Delete `simrun/internal/detonators/ssh_command_detonator.go`.
- [x] 2.3 Delete `simrun/internal/detonators/command_detonator.go`.
- [x] 2.4 Delete `simrun/internal/detonators/mocks/CommandDetonator.go`.
- [x] 2.5 In `simrun/internal/parser/main.go::createDetonator`, remove the `if detonate.LocalDetonator != nil` branch and the `if detonate.RemoteDetonator != nil` branch. Remove the now-unused `strings` import if nothing else uses it.
- [x] 2.6 Run `go generate ./...` to confirm no stale mock for `CommandDetonator` is regenerated. Search the repo for any leftover `//go:generate` directive that referenced the removed interface and delete it.
- [x] 2.7 Run `go build ./...`. Resolve any compile errors that surface only after the deletions (likely none beyond steps 2.1–2.5).

## 3. Test cleanup

- [x] 3.1 In `simrun/internal/parser/parser_test.go`, remove the test cases that assert `LocalCommandExecutor` / `SSHCommandExecutor` / `NewCommandDetonator` (around lines 35–57). Keep tests for surviving detonators.
- [x] 3.2 In `simrun/internal/web/api_scenarios_test.go`, replace the `localDetonator:` fixture (around line 18) with an `awsCliDetonator: { script: "true" }` block, or with an `inject:` block if the test does not need a detonate.
- [x] 3.3 In `simrun/internal/web/coverage_test.go`, replace the `localDetonator:` fixture (around line 38) using the same approach as 3.2.
- [x] 3.4 Run `go test ./...` and confirm green.

## 4. Frontend cleanup

- [x] 4.1 In `web/frontend/src/lib/utils/yaml-parser.ts`, remove the `det.localDetonator` and `det.remoteDetonator` references from the builder-support detection (around line 43). Builder support remains gated on the presence of `simrunDetonator` (or absence of any non-simrun detonator), matching what the builder UI can actually round-trip.
- [x] 4.2 In `web/frontend/`, run `npm run check` and `npm run build`; resolve any TypeScript errors introduced by the simplification.
- [x] 4.3 Run `mise run build-frontend` to refresh embedded assets if the frontend build output is committed.

## 5. Dependency hygiene

- [x] 5.1 Search for remaining users of `golang.org/x/crypto/ssh` (`grep -rn "golang.org/x/crypto/ssh" simrun/`). If `simrun/pack/ssh.go` is the only remaining importer, leave the dependency. If no users remain, run `go mod tidy` to drop it from `go.mod` / `go.sum`.

## 6. Documentation

- [x] 6.1 In `CLAUDE.md`, delete the bullet `- LocalCommandExecutor / SSHCommandExecutor - Command execution backends (local or SSH)` under Detonators. Adjust the surrounding bullet list if it leaves a dangling group.
- [x] 6.2 Verify no other docs reference `localDetonator`, `remoteDetonator`, `CommandDetonator`, `LocalCommandExecutor`, or `SSHCommandExecutor` (`grep -rn` across `docs/`, `README*`, `web/frontend/README*`).

## 7. Spec sync

- [x] 7.1 Delete `openspec/specs/command-executors/spec.md` (and the empty `command-executors/` directory) when the change is archived.
- [x] 7.2 Apply the `MODIFIED` deltas in `openspec/changes/remove-unused-command-executors/specs/detonators/spec.md` and `specs/parser/spec.md` to their canonical specs in `openspec/specs/` at archive time.

## 8. Verification

- [x] 8.1 `mise run build` (full server + frontend build) succeeds.
- [x] 8.2 `go test ./...` is green.
- [ ] 8.3 Run a manual smoke: start `simrun`, run a saved scenario that uses `simrunDetonator`, and a scenario that uses `awsCliDetonator`; confirm both reach the matching phase.
- [ ] 8.4 Lint a YAML containing `localDetonator` via `POST /api/scenarios/lint` and confirm it now returns `{valid: false}` with an unknown-field message.
