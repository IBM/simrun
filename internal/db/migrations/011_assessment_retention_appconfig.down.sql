DELETE FROM app_config WHERE key IN (
    'assessment_log_retention_enabled',
    'assessment_log_retention_days',
    'assessment_retention_enabled',
    'assessment_retention_days'
);
