## MODIFIED Requirements

### Requirement: Delete Run Cascades to Results and Log File
The system SHALL delete the `runs` row on `DELETE /api/runs/{runId}`,
cascade-delete all `scenario_results` rows via the FK, and best-effort
remove the run's on-disk artifacts: the run's JSONL log file, every collected
`.ndjson` file referenced by the run's `scenario_results.collected_log_path`,
and, for each scenario result with a non-empty `execution_id`, the run's
Terraform working directory at `<DataDir>/terraform/<execution_id>/`. The
system SHALL skip Terraform-directory removal for any `execution_id` that is
blank/whitespace or contains a path separator, so cleanup can never escape or
remove the `<DataDir>/terraform/` base directory. Failure to remove any on-disk
artifact (log file, collected `.ndjson`, or Terraform directory) SHALL be logged
and SHALL NOT fail the request.

#### Scenario: Successful delete
- **WHEN** a client deletes a run with 3 results
- **THEN** the `runs` row, all 3 `scenario_results` rows, and the JSONL log file are removed

#### Scenario: Terraform directories removed
- **WHEN** a client deletes a run whose scenario results have execution IDs `E1` and `E2`
- **THEN** the directories `<DataDir>/terraform/E1/` and `<DataDir>/terraform/E2/` are removed along with the row and log file

#### Scenario: Missing Terraform directory does not fail delete
- **WHEN** a run is deleted but its `<DataDir>/terraform/<execution_id>/` directory is already gone
- **THEN** the delete succeeds and the missing directory is ignored

#### Scenario: Unsafe execution id skipped
- **WHEN** a scenario result has a blank `execution_id` or one containing a path separator
- **THEN** no Terraform directory is removed for that result and the `<DataDir>/terraform/` base directory is left intact
