-- Backfill app_config with default values for v4 typed AppConfig fields.
-- Idempotent — only inserts if the key is missing.
INSERT INTO app_config (key, value) VALUES
    ('parallelism',         '5'::jsonb),
    ('terraform_version',   '""'::jsonb),
    ('pack_logs_enabled',   'true'::jsonb),
    ('ssh_logging_enabled', 'false'::jsonb)
ON CONFLICT (key) DO NOTHING;

-- Add `ssh` to the set of connector types that support is_default
-- (mirrors 004_kubernetes_connector pattern).
DROP INDEX IF EXISTS idx_connectors_default_per_type;
CREATE UNIQUE INDEX idx_connectors_default_per_type
ON connectors(type)
WHERE is_default = true AND type IN ('aws', 'gcp', 'azure', 'kubernetes', 'ssh');
