ALTER INDEX idx_runs_assessment_id RENAME TO idx_runs_scenario_id;
ALTER TABLE runs RENAME COLUMN assessment_id TO scenario_id;
