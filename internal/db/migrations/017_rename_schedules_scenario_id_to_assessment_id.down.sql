ALTER INDEX idx_schedules_assessment_id RENAME TO idx_schedules_scenario_id;
ALTER TABLE schedules RENAME COLUMN assessment_id TO scenario_id;
