-- Support ORDER BY created_at DESC + LIMIT/OFFSET on /api/runs.
CREATE INDEX IF NOT EXISTS idx_runs_created_at ON runs (created_at DESC);
