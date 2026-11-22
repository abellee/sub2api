-- Switch only after the index on the new extractor has been built.
-- Keep task deduplication out of the aggregate expression: SQL-function/view
-- inlining duplicated its correlated lookups for every metric and triggered JIT.
CREATE OR REPLACE FUNCTION ops_media_task_sla_sample(
  path text, body text, code integer, owner_user_id bigint,
  event_at timestamptz, event_id bigint
) RETURNS boolean
LANGUAGE plpgsql STABLE PARALLEL SAFE COST 10 AS $$
DECLARE
  task_id text := ops_media_task_id_fast(path, body);
BEGIN
  IF task_id IS NULL THEN
    RETURN true;
  END IF;
  IF NOT ops_media_task_failure(code, body) THEN
    RETURN false;
  END IF;
  IF EXISTS (SELECT 1 FROM usage_logs u
    WHERE u.request_id = 'media_task_settle:' || task_id) THEN
    RETURN false;
  END IF;

  -- Separate NULL owners so both paths can use the user_id index key.
  IF owner_user_id IS NULL THEN
    RETURN NOT EXISTS (
      SELECT 1 FROM ops_error_logs previous
      WHERE ops_media_task_id_fast(previous.request_path, previous.error_body) = task_id
        AND previous.user_id IS NULL
        AND previous.status_code >= 400
        AND NOT previous.is_business_limited AND NOT previous.is_count_tokens
        AND ops_media_task_failure(previous.status_code, previous.error_body)
        AND (previous.created_at, previous.id) < (event_at, event_id)
    );
  END IF;
  RETURN NOT EXISTS (
    SELECT 1 FROM ops_error_logs previous
    WHERE ops_media_task_id_fast(previous.request_path, previous.error_body) = task_id
      AND previous.user_id = owner_user_id
      AND previous.status_code >= 400
      AND NOT previous.is_business_limited AND NOT previous.is_count_tokens
      AND ops_media_task_failure(previous.status_code, previous.error_body)
      AND (previous.created_at, previous.id) < (event_at, event_id)
  );
END
$$;

CREATE OR REPLACE VIEW ops_error_log_metrics AS
SELECT e.*, sample.is_sla_sample
FROM ops_error_logs e
CROSS JOIN LATERAL (
  SELECT CASE
    WHEN split_part(e.request_path, '?', 1) IN
      ('/v1/images/generations','/v1/images/edits','/v1/videos','/v1/videos/generations')
      OR starts_with(e.request_path, '/media/tasks/')
      OR starts_with(e.request_path, '/v1/images/tasks/')
      OR starts_with(e.request_path, '/v1/videos/')
    THEN ops_media_task_sla_sample(e.request_path, e.error_body, e.status_code,
      e.user_id, e.created_at, e.id)
    ELSE true
  END AS is_sla_sample
  OFFSET 0
) sample;
