## Why

The `LocalCommandExecutor`, `SSHCommandExecutor`, and `CommandDetonator` (interface + `CommandDetonatorImpl`) are wired up but unused in any current scenario or pack, and the team plans to re-implement command-based detonation from scratch with a different design. Keeping the dead implementation costs maintenance (mockery regeneration, schema surface, doc upkeep) and confuses readers about what is actually exercised.

## What Changes

- **BREAKING**: Remove `simrun/internal/detonators/command_detonator.go` (the `CommandDetonator` interface, `CommandDetonatorImpl`, `OSLayerAttackSimulation`, and `NewCommandDetonator`).
- **BREAKING**: Remove `simrun/internal/detonators/local_command_detonator.go` (`LocalCommandExecutor`).
- **BREAKING**: Remove `simrun/internal/detonators/ssh_command_detonator.go` (`SSHCommandExecutor`, `NewSSHCommandExecutor`).
- **BREAKING**: Remove `simrun/internal/detonators/mocks/CommandDetonator.go` (mockery-generated mock).
- **BREAKING**: Remove the `localDetonator` and `remoteDetonator` choices from the YAML scenario schema (`simrun/schemas/simrun.schema.json` and any sibling `*.schema.json` files those `$ref`s point at). Regenerate parser code (`mise run parser`).
- Remove the `LocalDetonator` and `RemoteDetonator` branches from `createDetonator` in `simrun/internal/parser/main.go`. After removal, `createDetonator` handles only `awsCliDetonator` and `simrunDetonator`.
- Drop the corresponding parser test cases in `simrun/internal/parser/parser_test.go` and the test fixtures in `simrun/internal/web/api_scenarios_test.go` and `simrun/internal/web/coverage_test.go` that use `localDetonator`.
- Update `web/frontend/src/lib/utils/yaml-parser.ts` to drop references to `localDetonator` / `remoteDetonator` in the builder-support detection.
- Update `CLAUDE.md` to remove the `LocalCommandExecutor / SSHCommandExecutor` bullet under Detonators.
- The SSH connector type, `connectors-ssh` spec, and `ssh_logging_enabled` config remain untouched — they are preserved for the planned re-implementation. The `targets.ssh` resolution path in `simrun/internal/web/scenarios.go` (resolveConnectorCreds case `"ssh"`) and the temp-key-file materialization also stay; they become orphan code paths until the new command executor lands and are explicitly out of scope for this change.

## Capabilities

### New Capabilities
None.

### Modified Capabilities
- `command-executors`: REMOVED. The entire capability spec is deleted.
- `detonators`: Remove the requirement language and scenarios that mention `localDetonator` / `remoteDetonator` (Execution ID Format Varies By Detonator, Env Vars Threading Differs By Detonator).
- `parser`: Remove the `SSH Detonator Requires Run-Time Env` requirement and the lint note that says lint cannot validate `remoteDetonator`.
- `scenarios`: No requirement-level change; the YAML schema reduction is captured in the `parser` spec via the removal above.

## Impact

- **Affected code**: `simrun/internal/detonators/` (3 source files + mock), `simrun/internal/parser/main.go`, `simrun/internal/parser/parser.go` (regenerated), `simrun/internal/parser/parser_test.go`, `simrun/internal/web/api_scenarios_test.go`, `simrun/internal/web/coverage_test.go`, `simrun/schemas/simrun.schema.json` (+ referenced sub-schemas), `web/frontend/src/lib/utils/yaml-parser.ts`, `CLAUDE.md`.
- **APIs**: No HTTP API contract changes. YAML scenarios that previously used `localDetonator` or `remoteDetonator` will fail to parse with an unknown-field error. No such scenarios exist in tree.
- **DB / migrations**: None.
- **Dependencies**: None removed; `golang.org/x/crypto/ssh` may now be unused — verify with `go mod tidy` and remove from `go.mod` if so.
- **Specs**: `openspec/specs/command-executors/spec.md` deleted; `detonators`, `parser` specs trimmed.
