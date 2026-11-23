CREATE TABLE inflation_monitor_plans (
    group_id BIGINT PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    config JSONB NOT NULL,
    revision TEXT NOT NULL,
    next_run_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_until TIMESTAMPTZ,
    lease_token TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE inflation_monitor_results (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    measured_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    result JSONB NOT NULL
);
CREATE INDEX inflation_monitor_results_group_recent ON inflation_monitor_results(group_id, id DESC);
