-- Polling paths are already structured. Avoid the bounded-repeat path regex
-- on every row; retain the original body matcher for generation endpoints.
CREATE OR REPLACE FUNCTION ops_media_task_id_fast(path text, body text) RETURNS text
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

