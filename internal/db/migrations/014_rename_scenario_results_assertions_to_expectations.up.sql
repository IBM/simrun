-- Drop "assertion" from the vocabulary: the per-expectation outcome column
-- becomes "expectations". The scenario_results table name is unchanged.
ALTER TABLE scenario_results RENAME COLUMN assertions TO expectations;
