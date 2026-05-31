## Context

The command-execution detonator family — `LocalCommandExecutor`, `SSHCommandExecutor`, and the `CommandDetonator` interface/`CommandDetonatorImpl` wrapper — was the original mechanism for running raw shell commands as attack simulations. Today the running platform exclusively detonates via `SimrunDetonator` (terraform-driven packs), `AWSCLIDetonator` (bash AWS CLI script), and `AWSDetonator` (programmatic AWS SDK). No scenario YAML in tree, no installed pack, and no UI flow exercises `localDetonator` or `remoteDetonator`. The team has decided to re-implement command-based detonation with a different design (likely with cleaner env propagation, host-key verification, and a per-command timeout — three known gaps documented in the current `command-executors` spec).

The current code costs maintenance because:
- The mockery contract (`detonators/mocks/CommandDetonator.go`) regenerates on every `go generate ./...` and shows up in diffs that touch the detonators package.
- The schema surface (`localDetonator.schema.json`, `remoteDetonator.schema.json`) is referenced from the main schema and from the lint endpoint's behavior contract.
- `simrun/internal/parser/main.go::createDetonator` carries two branches that nothing reaches.
- The frontend `yaml-parser.ts` builder-detection includes `localDetonator`/`remoteDetonator` for a feature path that no user can produce from the UI.

The SSH connector (`simrun/internal/web/connector_handlers.go`, `connectors-ssh` spec) and the `ssh_logging_enabled` config flag are kept untouched. The connector's stored configuration may be re-used by the next iteration; `ssh_logging_enabled` separately drives `SR_SSH_LOG_DIR` for terraform pack SSH logging in `simrun/pack/ssh.go`, which is unrelated to `CommandDetonator`.

## Goals / Non-Goals

**Goals:**
- Delete the three executor types and their mock from `simrun/internal/detonators/`.
- Remove the parser branches and YAML schema entries that reference them so the parser cannot accept `localDetonator`/`remoteDetonator` YAML.
- Remove or trim openspec specs that document the deleted behavior.
- Keep the SSH connector type, its CRUD/handlers, the `connectors-ssh` spec, and `ssh_logging_enabled`.
- Leave the build green: `go build ./...`, `go test ./...`, `mise run build`, and the frontend `npm run check` all pass after the change.

**Non-Goals:**
- Designing or stubbing the replacement command-executor — explicitly deferred.
- Removing the `targets.ssh` resolution branch in `scenarios.go` (`resolveConnectorCreds` case `"ssh"`) and its temp-key-file materialization. Those become orphaned after this change but are kept verbatim for the planned re-implementation. They are exercised only when a SSH connector is selected via `targets.ssh`, and no in-tree code path will reach that selection once `remoteDetonator` is gone.
- Removing the SSH connector "is_default" check or the unsupported-test-connection behavior.
- Removing `ssh_logging_enabled` from `app_config` or its UI control.
- Migrating any data — there is none to migrate. Saved scenarios that contain `localDetonator`/`remoteDetonator` would fail to parse, but none exist in the seed packs or test fixtures and the tool is not yet at GA.
- Re-implementing or re-vendoring the SSH client library. `golang.org/x/crypto/ssh` will be dropped from imports automatically; `go mod tidy` decides whether it can leave `go.mod`.

## Decisions

**Decision 1: Delete the executor capability spec entirely rather than mark it deprecated.**

Alternatives considered:
- Mark `command-executors/spec.md` as deprecated with a banner.
- Keep the spec and add a "Currently Unimplemented" requirement.

Rationale: The capability is going away from the codebase, not just being paused. Keeping a spec for code that no longer exists means specs and code can drift. When the replacement lands, it will get its own fresh capability spec with a different design (host-key verification, env propagation, timeouts), so reviving this spec verbatim would be misleading. A future change-proposal can introduce the new spec at that time.

**Decision 2: Trim, don't delete, the `detonators` and `parser` capability specs.**

Those capabilities continue to exist for the surviving detonators (simrun/aws-cli/aws-sdk) and the surviving parser branches. Only the requirements that mention `localDetonator`/`remoteDetonator` are removed:
- `detonators/spec.md` "Execution ID Format Varies By Detonator" — delete the clause about command executors emitting UUIDv4. The remaining text covers simrun/aws-cli/aws-sdk.
- `detonators/spec.md` "Env Vars Threading Differs By Detonator" — drop the "Command-based detonators ... SHALL NOT propagate" sentence, drop the "Env not propagated to local commands" scenario.
- `parser/spec.md` "SSH Detonator Requires Run-Time Env" — delete the requirement and its scenario.
- `parser/spec.md` "Lint Endpoint Behavior" — drop the "cannot validate `remoteDetonator`" qualifier; lint can still not fully validate `simrunDetonator`.

**Decision 3: Regenerate parser code via `mise run parser` after schema removal.**

The generated `simrun/internal/parser/parser.go` contains generated Go types for `LocalDetonator`, `RemoteDetonator`, and the `Detonate` union. Hand-editing the generated file is forbidden by the project workflow (CLAUDE.md describes `mise run parser` as the regeneration command). The change deletes `localDetonator.schema.json` and `remoteDetonator.schema.json`, removes their two `$ref`s and the two enum entries from `simrun.schema.json`, then regenerates. The `createDetonator` function loses references to types that no longer exist and is updated by hand in the same commit.

**Decision 4: Tests that use `localDetonator` are deleted, not migrated to `simrunDetonator`.**

`api_scenarios_test.go` and `coverage_test.go` use `localDetonator` purely as a minimal "any detonator will do" fixture. Migrating them to `simrunDetonator` would require introducing a fake pack (and pack runner factory) into those tests, which is significantly more setup than the tests need. Where a test still needs a detonate block, switch to `awsCliDetonator` (which needs no pack runner and only requires a `script` string). Where the test doesn't actually exercise the detonator, drop the field and use an `inject:` block.

**Decision 5: Frontend yaml-parser.ts simplification is in scope.**

`web/frontend/src/lib/utils/yaml-parser.ts` checks `det.localDetonator || det.remoteDetonator || det.awsCliDetonator` to decide if a YAML is "builder supported." With `localDetonator`/`remoteDetonator` gone from the schema, the check becomes `det.awsCliDetonator || (anything else not simrunDetonator)`. Simplify to: builder is supported only when `detonate.simrunDetonator` is the active detonator (matches existing builder code that only emits `simrunDetonator`). Any non-simrun detonator falls into the YAML-only path, same as today.

## Risks / Trade-offs

- **[Saved-scenario YAML in user databases breaks]** → Mitigation: ASP is not yet at GA. The team operates the only deployments. Confirmed with the user that no saved scenarios in any environment use these fields. If a stray one exists, it surfaces as a parse error on next run, not silent corruption.

- **[`go mod tidy` removes `golang.org/x/crypto/ssh` and breaks an unrelated import elsewhere]** → Mitigation: search the tree for other consumers before tidying; only remove the dependency if no other package imports it. The likely surviving consumer is `simrun/pack/ssh.go`, which uses the package for terraform pack SSH log capture — verify in the apply step.

- **[Regenerating `parser.go` produces an unrelated diff]** → Mitigation: regenerate on a clean workspace and commit the generated diff separately from the hand-written changes so reviewers can audit each.

- **[Future re-implementation needs the deleted spec for context]** → Mitigation: this change is captured in `openspec/changes/remove-unused-command-executors/`; the deleted spec is recoverable from the change archive. Additionally, the design rationale documented here highlights the three gaps (env propagation, host-key verification, per-command timeout) the next iteration must address.

- **[Orphaned `targets.ssh` resolver and temp-key-file code in `scenarios.go` rot]** → Mitigation: out of scope, but flagged. A follow-up issue should remove or guard them if the re-implementation slips. They have no caller after this change but compile fine.

## Migration Plan

1. Land schema + Go + frontend deletions in a single commit set on a feature branch.
2. CI runs `go build ./...`, `go test ./...`, `npm run check`, and `mise run build` — all must pass before merge.
3. Deploy to staging; smoke-test a scenario run that uses `simrunDetonator` and one that uses `awsCliDetonator` to confirm the surviving paths still work.
4. No DB migration needed.
5. Rollback: revert the merge commit. There is no forward-only step (no schema migration, no data migration), so revert is a clean restore.
