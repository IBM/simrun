-- Backfill app_config with the org-wide default_tags key.
-- Idempotent — only inserts if the key is missing. Aligned with DefaultAppConfig().
INSERT INTO app_config (key, value) VALUES
    ('default_tags', '{}'::jsonb)
ON CONFLICT (key) DO NOTHING;
