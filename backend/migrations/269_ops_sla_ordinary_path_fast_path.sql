-- Ordinary API errors need no media task lookup or PL/pgSQL call.
CREATE OR REPLACE VIEW ops_error_log_metrics AS
SELECT e.*, sample.is_sla_sample
FROM ops_error_logs e
CROSS JOIN LATERAL (
  SELECT CASE
    WHEN split_part(e.request_path, '?', 1) IN
      ('/v1/images/generations','/v1/images/edits','/v1/videos','/v1/videos/generations')
      OR split_part(e.request_path, '?', 1) ~ '^/(?:v1/images/tasks|media/tasks|v1/videos)/(mtask_[A-Za-z0-9_-]{1,128})$'
    THEN ops_media_task_sla_sample(e.request_path, e.error_body, e.status_code,
      e.user_id, e.created_at, e.id)
    ELSE true
  END AS is_sla_sample
  OFFSET 0
) sample;
