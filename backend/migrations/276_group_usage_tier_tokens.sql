-- Money thresholds cannot be converted into token thresholds. Preserve the
-- original rules, and require an explicit token configuration before enabling.
ALTER TABLE group_usage_tier_rules ADD COLUMN legacy_cost_config JSONB;
UPDATE group_usage_tier_rules
SET legacy_cost_config = config,
    config = (config - 'tiers') || '{"enabled":false,"tiers":[],"models":[]}'::jsonb,
    revision = revision + 1,
    started_at = NOW()
WHERE EXISTS (SELECT 1 FROM jsonb_array_elements(config->'tiers') AS tier WHERE tier ? 'daily_cost');

CREATE TABLE user_group_usage_tier_token_daily (
    group_id BIGINT NOT NULL REFERENCES group_usage_tier_rules(group_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL,
    day DATE NOT NULL,
    total_tokens BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, user_id, revision, day)
);

DROP TRIGGER accumulate_group_usage_tier_spend ON usage_logs;
DROP FUNCTION accumulate_group_usage_tier_spend();

CREATE FUNCTION accumulate_group_usage_tier_tokens() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    entry RECORD;
    rule_revision BIGINT;
BEGIN
    -- Subtract the old token contribution on correction, then add the new one.
    -- Deleting raw logs intentionally does not erase the retained daily total.
    FOR entry IN
        SELECT OLD.group_id AS group_id, OLD.user_id AS user_id, OLD.created_at AS created_at,
            COALESCE(NULLIF(OLD.requested_model, ''), OLD.model) AS model,
            -(OLD.input_tokens::bigint + OLD.output_tokens + OLD.cache_creation_tokens + OLD.cache_read_tokens) AS tokens
        WHERE TG_OP = 'UPDATE'
        UNION ALL
        SELECT NEW.group_id, NEW.user_id, NEW.created_at,
            COALESCE(NULLIF(NEW.requested_model, ''), NEW.model),
            NEW.input_tokens::bigint + NEW.output_tokens + NEW.cache_creation_tokens + NEW.cache_read_tokens
    LOOP
        IF entry.tokens = 0 OR entry.group_id IS NULL THEN CONTINUE; END IF;
        SELECT revision INTO rule_revision FROM group_usage_tier_rules
        WHERE group_id = entry.group_id AND config->>'enabled' = 'true'
          AND entry.created_at >= started_at
          AND (COALESCE(jsonb_array_length(config->'models'), 0) = 0 OR config->'models' ? entry.model)
        FOR SHARE;
        IF NOT FOUND THEN CONTINUE; END IF;
        INSERT INTO user_group_usage_tier_token_daily(group_id,user_id,revision,day,total_tokens)
        VALUES(entry.group_id,entry.user_id,rule_revision,(entry.created_at AT TIME ZONE 'Asia/Shanghai')::date,entry.tokens)
        ON CONFLICT(group_id,user_id,revision,day) DO UPDATE
        SET total_tokens = user_group_usage_tier_token_daily.total_tokens + EXCLUDED.total_tokens;
    END LOOP;
    RETURN NEW;
END;
$$;

CREATE TRIGGER accumulate_group_usage_tier_tokens
AFTER INSERT OR UPDATE OF input_tokens,output_tokens,cache_creation_tokens,cache_read_tokens,model,requested_model,group_id,user_id,created_at ON usage_logs
FOR EACH ROW EXECUTE FUNCTION accumulate_group_usage_tier_tokens();
