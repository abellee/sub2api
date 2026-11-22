-- Refresh only hours touched by retained media query logs. Preserve successes,
-- request/error volume, billing, latency samples and the original error records.
CREATE TEMP TABLE ops_media_sla_hours ON COMMIT DROP AS
SELECT DISTINCT date_trunc('hour', created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket_start
FROM ops_error_logs WHERE ops_media_task_id(request_path, error_body) IS NOT NULL;

WITH corrected AS (
  SELECT date_trunc('hour', e.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC' AS bucket_start,
    CASE WHEN GROUPING(e.platform) = 1 THEN NULL ELSE e.platform END AS platform,
    CASE WHEN GROUPING(e.group_id) = 1 THEN NULL ELSE e.group_id END AS group_id,
    COUNT(*) FILTER (WHERE e.status_code >= 400 AND NOT e.is_business_limited AND e.is_sla_sample) AS sla,
    COUNT(*) FILTER (WHERE e.error_owner = 'provider' AND NOT e.is_business_limited AND e.is_sla_sample
      AND COALESCE(e.upstream_status_code, e.status_code, 0) NOT IN (429,529)) AS upstream,
    COUNT(*) FILTER (WHERE e.error_owner = 'provider' AND NOT e.is_business_limited AND e.is_sla_sample
      AND COALESCE(e.upstream_status_code, e.status_code, 0) = 429) AS upstream429,
    COUNT(*) FILTER (WHERE e.error_owner = 'provider' AND NOT e.is_business_limited AND e.is_sla_sample
      AND COALESCE(e.upstream_status_code, e.status_code, 0) = 529) AS upstream529
  FROM (SELECT created_at, COALESCE(platform, 'unknown') AS platform, group_id,
    status_code, upstream_status_code, error_owner, is_business_limited, is_count_tokens, is_sla_sample
    FROM ops_error_log_metrics) e
  JOIN ops_media_sla_hours h ON e.created_at >= h.bucket_start AND e.created_at < h.bucket_start + interval '1 hour'
  WHERE NOT e.is_count_tokens
  GROUP BY GROUPING SETS ((date_trunc('hour', e.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
    (date_trunc('hour', e.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC', e.platform),
    (date_trunc('hour', e.created_at AT TIME ZONE 'UTC') AT TIME ZONE 'UTC', e.platform, e.group_id))
  HAVING GROUPING(e.group_id) = 1 OR e.group_id IS NOT NULL
)
UPDATE ops_metrics_hourly h SET error_count_sla = c.sla,
  upstream_error_count_excl_429_529 = c.upstream,
  upstream_429_count = c.upstream429, upstream_529_count = c.upstream529,
  computed_at = now()
FROM corrected c WHERE h.bucket_start = c.bucket_start
  AND h.platform IS NOT DISTINCT FROM c.platform
  AND h.group_id IS NOT DISTINCT FROM c.group_id;

WITH corrected AS (
  SELECT (h.bucket_start AT TIME ZONE 'UTC')::date AS bucket_date, h.platform, h.group_id,
    SUM(h.error_count_sla) AS sla, SUM(h.upstream_error_count_excl_429_529) AS upstream,
    SUM(h.upstream_429_count) AS upstream429, SUM(h.upstream_529_count) AS upstream529
  FROM ops_metrics_hourly h
  WHERE (h.bucket_start AT TIME ZONE 'UTC')::date IN
    (SELECT (bucket_start AT TIME ZONE 'UTC')::date FROM ops_media_sla_hours)
  GROUP BY 1,2,3
)
UPDATE ops_metrics_daily d SET error_count_sla = c.sla,
  upstream_error_count_excl_429_529 = c.upstream,
  upstream_429_count = c.upstream429, upstream_529_count = c.upstream529,
  computed_at = now()
FROM corrected c WHERE d.bucket_date = c.bucket_date
  AND d.platform IS NOT DISTINCT FROM c.platform
  AND d.group_id IS NOT DISTINCT FROM c.group_id;
