-- NULL distinguishes old cost-only buckets during a rolling upgrade.
SET LOCAL lock_timeout = '5s';
ALTER TABLE usage_group_daily_rollups
    ADD COLUMN IF NOT EXISTS input_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS cache_creation_tokens BIGINT,
    ADD COLUMN IF NOT EXISTS cache_read_tokens BIGINT;

DROP TRIGGER IF EXISTS usage_logs_group_rollup_invalidate_update ON usage_logs;
CREATE TRIGGER usage_logs_group_rollup_invalidate_update
AFTER UPDATE OF created_at, group_id, actual_cost, input_tokens, cache_creation_tokens, cache_read_tokens ON usage_logs
FOR EACH ROW
WHEN (
    (OLD.created_at IS DISTINCT FROM NEW.created_at
     OR OLD.group_id IS DISTINCT FROM NEW.group_id
     OR OLD.actual_cost IS DISTINCT FROM NEW.actual_cost
     OR OLD.input_tokens IS DISTINCT FROM NEW.input_tokens
     OR OLD.cache_creation_tokens IS DISTINCT FROM NEW.cache_creation_tokens
     OR OLD.cache_read_tokens IS DISTINCT FROM NEW.cache_read_tokens)
    AND (OLD.group_id IS NOT NULL OR NEW.group_id IS NOT NULL)
)
EXECUTE FUNCTION invalidate_group_usage_rollup_state();
