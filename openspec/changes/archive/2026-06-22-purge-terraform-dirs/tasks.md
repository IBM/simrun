## 1. Implementation

- [x] 1.1 In `internal/web/run_deletion.go`, in the existing `GetScenarioResults` loop of `deleteRunWithArtifacts`, collect each result's `ExecutionID` alongside the collected `.ndjson` paths.
- [x] 1.2 After the collected-`.ndjson` removal block, add a best-effort Terraform-dir removal step: for each collected `execution_id`, skip it when `strings.TrimSpace(id) == ""` or it contains a path separator (`/` or `filepath.Separator`); otherwise `os.RemoveAll(filepath.Join(dataDir, "terraform", id))`, logging a warning with `run_id` and path on any error that is not `os.IsNotExist`.
- [x] 1.3 Update the `deleteRunWithArtifacts` doc comment to include the Terraform working directories in the enumerated artifact set it removes.

## 2. Tests

- [x] 2.1 Add a test (table-driven where it fits the existing style) verifying that deleting a run removes `<dataDir>/terraform/<executionID>/` for each scenario result's execution id, using a temp `dataDir` with seeded dirs.
- [x] 2.2 Add a test asserting a blank or path-separator-containing `execution_id` is skipped and the `<dataDir>/terraform/` base directory is left intact.
- [x] 2.3 Add a test asserting an already-missing Terraform directory does not fail the delete (best-effort contract).

## 3. Verify

- [x] 3.1 Run `go test ./internal/web/...` and confirm the new and existing deletion tests pass.
- [x] 3.2 Run `mise run lint` and confirm no new findings in `run_deletion.go`.
