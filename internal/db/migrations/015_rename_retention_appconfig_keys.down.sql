-- Reverse 015: restore the assessment_(log_)retention_* key names, carrying
-- forward the current values, then remove the run_(log_)retention_* keys.
INSERT INTO app_config (key, value)
SELECT replace(key, 'run_', 'assessment_'), value
  FROM app_config
 WHERE key IN ('run_log_retention_enabled', 'run_log_retention_days',
               'run_retention_enabled', 'run_retention_days')
ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value;

DELETE FROM app_config
 WHERE key IN ('run_log_retention_enabled', 'run_log_retention_days',
               'run_retention_enabled', 'run_retention_days');
