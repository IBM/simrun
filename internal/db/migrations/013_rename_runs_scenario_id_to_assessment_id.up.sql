-- The run -> definition FK becomes assessment_id. The runs table name is
-- unchanged; only the column (and its index) are renamed. The FK constraint
-- already references the renamed assessments table.
ALTER TABLE runs RENAME COLUMN scenario_id TO assessment_id;
ALTER INDEX idx_runs_scenario_id RENAME TO idx_runs_assessment_id;
