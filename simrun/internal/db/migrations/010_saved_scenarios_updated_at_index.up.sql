-- Support ORDER BY updated_at DESC + LIMIT/OFFSET on /api/scenarios.
CREATE INDEX IF NOT EXISTS idx_saved_scenarios_updated_at ON saved_scenarios (updated_at DESC);
