-- Polling paths are already structured. Avoid the bounded-repeat path regex
-- on every row; retain the original body matcher for generation endpoints.
CREATE OR REPLACE FUNCTION ops_media_task_id(path text, body text) RETURNS text
LANGUAGE plpgsql IMMUTABLE PARALLEL SAFE AS $$
DECLARE
  clean_path text := split_part(path, '?', 1);
  candidate text;
BEGIN
  IF clean_path IN ('/v1/images/generations','/v1/images/edits','/v1/videos','/v1/videos/generations') THEN
    RETURN substring(body FROM '"task_id"[[:space:]]*:[[:space:]]*"(mtask_[A-Za-z0-9_-]{1,128})"');
  END IF;
  IF starts_with(clean_path, '/v1/images/tasks/') THEN
    candidate := substr(clean_path, length('/v1/images/tasks/') + 1);
  ELSIF starts_with(clean_path, '/media/tasks/') THEN
    candidate := substr(clean_path, length('/media/tasks/') + 1);
  ELSIF starts_with(clean_path, '/v1/videos/') THEN
    candidate := substr(clean_path, length('/v1/videos/') + 1);
  END IF;
  IF starts_with(candidate, 'mtask_') AND length(candidate) BETWEEN 7 AND 134
    AND candidate !~ '[^A-Za-z0-9_-]' THEN
    RETURN candidate;
  END IF;
  RETURN NULL;
END;
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
