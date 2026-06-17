-- Backfill app_config with default values for retention settings.
-- Idempotent — only inserts if the key is missing. Aligned with DefaultAppConfig().
INSERT INTO app_config (key, value) VALUES
    ('assessment_log_retention_enabled', 'true'::jsonb),
    ('assessment_log_retention_days',    '7'::jsonb),
    ('assessment_retention_enabled',     'false'::jsonb),
    ('assessment_retention_days',        '30'::jsonb)
ON CONFLICT (key) DO NOTHING;
