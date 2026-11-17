CREATE TABLE IF NOT EXISTS ops_latency_collections (
    sequence BIGSERIAL PRIMARY KEY,
    id UUID NOT NULL UNIQUE,
    user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    account_id BIGINT NOT NULL,
    created_by BIGINT NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    stopped_at TIMESTAMPTZ,
    stop_reason TEXT NOT NULL DEFAULT '',
    record_count INTEGER NOT NULL DEFAULT 0 CHECK (record_count BETWEEN 0 AND 10000)
);
CREATE UNIQUE INDEX IF NOT EXISTS ops_latency_collections_active_scope
    ON ops_latency_collections(user_id, group_id, account_id) WHERE stopped_at IS NULL;

CREATE TABLE IF NOT EXISTS ops_latency_records (
    id BIGSERIAL PRIMARY KEY,
    collection_id UUID NOT NULL REFERENCES ops_latency_collections(id) ON DELETE CASCADE,
    request_id TEXT NOT NULL,
    client_request_id TEXT NOT NULL DEFAULT '',
    detail JSONB NOT NULL
);
CREATE INDEX IF NOT EXISTS ops_latency_records_collection ON ops_latency_records(collection_id, id DESC);
CREATE INDEX IF NOT EXISTS ops_latency_records_request ON ops_latency_records(collection_id, request_id, id DESC);
CREATE INDEX IF NOT EXISTS ops_latency_records_client ON ops_latency_records(collection_id, client_request_id, id DESC);
