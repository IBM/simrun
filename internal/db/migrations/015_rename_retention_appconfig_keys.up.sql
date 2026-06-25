-- Retention keys gate the deletion of runs (executions), so they shorten to
-- run_(log_)retention_*. Carry forward any operator-set values from the former
-- assessment_(log_)retention_* keys, then remove the old keys.
INSERT INTO app_config (key, value)
SELECT replace(key, 'assessment_', 'run_'), value
  FROM app_config
 WHERE key IN ('assessment_log_retention_enabled', 'assessment_log_retention_days',
               'assessment_retention_enabled', 'assessment_retention_days')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

DELETE FROM app_config
 WHERE key IN ('assessment_log_retention_enabled', 'assessment_log_retention_days',
               'assessment_retention_enabled', 'assessment_retention_days');

-- Ensure defaults exist for fresh installs (idempotent). Aligned with DefaultAppConfig().
INSERT INTO app_config (key, value) VALUES
    ('run_log_retention_enabled', 'true'::jsonb),
    ('run_log_retention_days',    '7'::jsonb),
    ('run_retention_enabled',     'false'::jsonb),
    ('run_retention_days',        '30'::jsonb)
ON CONFLICT (key) DO NOTHING;
