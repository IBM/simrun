-- The schedule -> definition FK becomes assessment_id, matching the runs table
-- rename in migration 013. The schedules table name is unchanged; only the
-- column (and its index) are renamed. The FK already references assessments.
ALTER TABLE schedules RENAME COLUMN scenario_id TO assessment_id;
ALTER INDEX idx_schedules_scenario_id RENAME TO idx_schedules_assessment_id;
