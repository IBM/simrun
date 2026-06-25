-- Reverse 012: drop the unique slug constraint and restore the table/index names.
-- Disambiguating suffixes added on the up path are not removed (the original
-- duplicate names cannot be recovered), which is acceptable for a rollback.
ALTER TABLE assessments DROP CONSTRAINT assessments_name_key;
ALTER INDEX idx_assessments_updated_at RENAME TO idx_saved_scenarios_updated_at;
ALTER TABLE assessments RENAME TO saved_scenarios;
