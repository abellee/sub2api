CREATE TABLE IF NOT EXISTS account_matching_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_ids BIGINT[] NOT NULL CHECK (cardinality(group_ids) > 0),
    account_ids BIGINT[] NOT NULL CHECK (cardinality(account_ids) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS account_matching_rules_user_id_idx ON account_matching_rules(user_id);
