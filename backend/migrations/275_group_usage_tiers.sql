CREATE TABLE group_usage_tier_rules (
    group_id BIGINT PRIMARY KEY REFERENCES groups(id) ON DELETE CASCADE,
    config JSONB NOT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Keep qualifying spend even when raw usage logs are removed by retention jobs.
CREATE TABLE user_group_usage_tier_daily (
    group_id BIGINT NOT NULL REFERENCES group_usage_tier_rules(group_id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    revision BIGINT NOT NULL,
    day DATE NOT NULL,
    actual_cost NUMERIC(30, 10) NOT NULL DEFAULT 0,
    PRIMARY KEY (group_id, user_id, revision, day)
);

CREATE FUNCTION accumulate_group_usage_tier_spend() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    rule_revision BIGINT;
    charge NUMERIC;
BEGIN
    charge := NEW.actual_cost;
    IF TG_OP = 'UPDATE' THEN
        charge := NEW.actual_cost - OLD.actual_cost;
    END IF;
    IF charge = 0 OR NEW.group_id IS NULL THEN RETURN NEW; END IF;
    SELECT revision INTO rule_revision FROM group_usage_tier_rules
      WHERE group_id = NEW.group_id AND config->>'enabled' = 'true'
      AND NEW.created_at >= started_at;
    IF NOT FOUND THEN RETURN NEW; END IF;
    INSERT INTO user_group_usage_tier_daily(group_id,user_id,revision,day,actual_cost)
      VALUES(NEW.group_id,NEW.user_id,rule_revision,(NEW.created_at AT TIME ZONE 'Asia/Shanghai')::date,charge)
      ON CONFLICT(group_id,user_id,revision,day) DO UPDATE
      SET actual_cost = user_group_usage_tier_daily.actual_cost + EXCLUDED.actual_cost;
    RETURN NEW;
END;
$$;

CREATE TRIGGER accumulate_group_usage_tier_spend
AFTER INSERT OR UPDATE OF actual_cost ON usage_logs
FOR EACH ROW EXECUTE FUNCTION accumulate_group_usage_tier_spend();
