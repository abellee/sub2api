-- Pending/not-found polls must not be rechecked for every later failure.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_ops_media_terminal_sla
ON ops_error_logs (ops_media_task_id(request_path, error_body), user_id, created_at, id)
WHERE ops_media_task_id(request_path, error_body) IS NOT NULL
  AND status_code >= 400 AND NOT is_business_limited AND NOT is_count_tokens
  AND ops_media_task_failure(status_code, error_body);
