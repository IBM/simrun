## MODIFIED Requirements

### Requirement: Delete Run Cascades to Results and Log File
The system SHALL delete the `runs` row on `DELETE /api/runs/{runId}`,
cascade-delete all `scenario_results` rows via the FK, best-effort remove the
run's JSONL log file, and best-effort remove every collected `.ndjson` file
referenced by that run's `scenario_results.collected_log_path`. Failure to
delete any on-disk file SHALL NOT fail the request.

#### Scenario: Successful delete
- **WHEN** a client deletes a run with 3 results
- **THEN** the `runs` row, all 3 `scenario_results` rows, the JSONL log file, and any collected `.ndjson` files for those results are removed

#### Scenario: Collected log file missing
- **WHEN** a run is deleted and one of its `collected_log_path` files no longer exists on disk
- **THEN** the deletion still succeeds and the request returns HTTP 204
