CREATE TABLE runs (
    id            UUID PRIMARY KEY,
    status        TEXT NOT NULL DEFAULT 'running',
    start_time    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    end_time      TIMESTAMPTZ,
    total         INT NOT NULL DEFAULT 0,
    succeeded     INT NOT NULL DEFAULT 0,
    failed        INT NOT NULL DEFAULT 0,
    schedule_id   UUID,
    schedule_name TEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE scenario_results (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id             UUID NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    name               TEXT NOT NULL,
    is_success         BOOLEAN,
    error_message      TEXT,
    duration_secs      DOUBLE PRECISION,
    matching_dur_secs  DOUBLE PRECISION,
    time_executed      TIMESTAMPTZ,
    executor_name      TEXT,
    executor_type      TEXT,
    execution_id       TEXT,
    simulation_id      TEXT,
    assertions         JSONB,
    indicators         JSONB,
    metadata           JSONB,
    collected_log_path TEXT,
    collected_doc_count INT DEFAULT 0,
    status             TEXT NOT NULL DEFAULT 'completed',
    phase              TEXT,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_scenario_results_run_id ON scenario_results(run_id);

CREATE TABLE saved_scenarios (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    yaml       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE packs (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL UNIQUE,
    type       TEXT NOT NULL,
    source     TEXT NOT NULL,
    version    TEXT,
    status     TEXT NOT NULL DEFAULT 'installed',
    parameters JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE app_config (
    key        TEXT PRIMARY KEY,
    value      JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE secret_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    entries     JSONB NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    scenario_id     UUID NOT NULL UNIQUE REFERENCES saved_scenarios(id) ON DELETE CASCADE,
    cron_expression TEXT NOT NULL,
    enabled         BOOLEAN NOT NULL DEFAULT true,
    last_run_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_schedules_scenario_id ON schedules(scenario_id);
CREATE INDEX idx_schedules_enabled ON schedules(enabled) WHERE enabled = true;

-- Add FK now that schedules table exists
ALTER TABLE runs ADD CONSTRAINT fk_runs_schedule FOREIGN KEY (schedule_id) REFERENCES schedules(id) ON DELETE SET NULL;
CREATE INDEX idx_runs_schedule_id ON runs(schedule_id);

CREATE TABLE connectors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL UNIQUE,
    type            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    secret_group_id UUID REFERENCES secret_groups(id) ON DELETE SET NULL,
    config          JSONB NOT NULL DEFAULT '{}',
    enabled         BOOLEAN NOT NULL DEFAULT true,
    is_default      BOOLEAN NOT NULL DEFAULT false,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_connectors_type ON connectors(type);

CREATE UNIQUE INDEX idx_connectors_default_per_type
ON connectors(type)
WHERE is_default = true AND type IN ('aws', 'gcp', 'azure');

CREATE TABLE auth_sessions (
    id         TEXT PRIMARY KEY,
    email      TEXT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    picture    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_auth_sessions_expires ON auth_sessions(expires_at);
