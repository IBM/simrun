DROP INDEX IF EXISTS idx_runs_scenario_id;
ALTER TABLE runs DROP COLUMN IF EXISTS scenario_id;
