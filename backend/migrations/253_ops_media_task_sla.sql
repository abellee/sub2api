-- Keep query logs intact while counting a terminal media failure once per user/task.
CREATE OR REPLACE FUNCTION ops_media_task_id(path text, body text) RETURNS text
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
  SELECT COALESCE(substring(split_part(path, '?', 1)
    FROM '^/(?:v1/images/tasks|media/tasks|v1/videos)/(mtask_[A-Za-z0-9_-]{1,128})$'),
    CASE WHEN split_part(path, '?', 1) IN ('/v1/images/generations','/v1/images/edits','/v1/videos','/v1/videos/generations')
      THEN substring(body FROM '"task_id"[[:space:]]*:[[:space:]]*"(mtask_[A-Za-z0-9_-]{1,128})"') END)
$$;

CREATE OR REPLACE FUNCTION ops_media_task_failure(code integer, body text) RETURNS boolean
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
  -- Legacy 410 responses lost the task status. A settled success is excluded below.
  SELECT COALESCE(code = 410 OR body ~ '"status"[[:space:]]*:[[:space:]]*"(failed|recovery_required|interrupted)"'
    OR body ~ '"type"[[:space:]]*:[[:space:]]*"generation_(failed|busy|unavailable)"', false)
$$;

CREATE OR REPLACE VIEW ops_error_log_metrics AS
SELECT e.*,
  CASE
    WHEN ops_media_task_id(e.request_path, e.error_body) IS NULL THEN true
    WHEN NOT ops_media_task_failure(e.status_code, e.error_body) THEN false
    -- Expired/deleted successful results are retrieval errors, not failed generations.
    WHEN EXISTS (
      SELECT 1 FROM usage_logs u
      WHERE u.request_id = 'media_task_settle:' || ops_media_task_id(e.request_path, e.error_body)
    ) THEN false
    ELSE NOT EXISTS (
      SELECT 1 FROM ops_error_logs previous
      WHERE ops_media_task_id(previous.request_path, previous.error_body) = ops_media_task_id(e.request_path, e.error_body)
        AND previous.user_id IS NOT DISTINCT FROM e.user_id
        AND previous.status_code >= 400
        AND NOT previous.is_business_limited AND NOT previous.is_count_tokens
        AND ops_media_task_failure(previous.status_code, previous.error_body)
        AND (previous.created_at, previous.id) < (e.created_at, e.id)
    )
  END AS is_sla_sample
FROM ops_error_logs e;
