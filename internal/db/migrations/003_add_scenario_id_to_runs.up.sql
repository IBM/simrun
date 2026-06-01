ALTER TABLE runs ADD COLUMN scenario_id UUID REFERENCES saved_scenarios(id) ON DELETE SET NULL;
CREATE INDEX idx_runs_scenario_id ON runs(scenario_id);
