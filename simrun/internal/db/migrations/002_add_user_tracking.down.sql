ALTER TABLE connectors DROP COLUMN IF EXISTS updated_by;
ALTER TABLE connectors DROP COLUMN IF EXISTS created_by;

ALTER TABLE secret_groups DROP COLUMN IF EXISTS updated_by;
ALTER TABLE secret_groups DROP COLUMN IF EXISTS created_by;

ALTER TABLE schedules DROP COLUMN IF EXISTS updated_by;
ALTER TABLE schedules DROP COLUMN IF EXISTS created_by;

ALTER TABLE packs DROP COLUMN IF EXISTS installed_by;

ALTER TABLE saved_scenarios DROP COLUMN IF EXISTS updated_by;
ALTER TABLE saved_scenarios DROP COLUMN IF EXISTS created_by;

ALTER TABLE runs DROP COLUMN IF EXISTS created_by;
