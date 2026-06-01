-- Add user tracking columns to all relevant tables.
-- Existing rows get 'anonymous' as the default value.

ALTER TABLE runs ADD COLUMN created_by TEXT NOT NULL DEFAULT 'anonymous';

ALTER TABLE saved_scenarios ADD COLUMN created_by TEXT NOT NULL DEFAULT 'anonymous';
ALTER TABLE saved_scenarios ADD COLUMN updated_by TEXT NOT NULL DEFAULT 'anonymous';

ALTER TABLE packs ADD COLUMN installed_by TEXT NOT NULL DEFAULT 'anonymous';

ALTER TABLE schedules ADD COLUMN created_by TEXT NOT NULL DEFAULT 'anonymous';
ALTER TABLE schedules ADD COLUMN updated_by TEXT NOT NULL DEFAULT 'anonymous';

ALTER TABLE secret_groups ADD COLUMN created_by TEXT NOT NULL DEFAULT 'anonymous';
ALTER TABLE secret_groups ADD COLUMN updated_by TEXT NOT NULL DEFAULT 'anonymous';

ALTER TABLE connectors ADD COLUMN created_by TEXT NOT NULL DEFAULT 'anonymous';
ALTER TABLE connectors ADD COLUMN updated_by TEXT NOT NULL DEFAULT 'anonymous';
