package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Uses an isolated schema in a disposable local PostgreSQL database.
func TestMediaTaskSLAPostgres(t *testing.T) {
	dsn := os.Getenv("OPS_SLA_TEST_DSN")
	if dsn == "" {
		t.Skip("set OPS_SLA_TEST_DSN to a disposable PostgreSQL database")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	schema := fmt.Sprintf("ops_sla_%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema)
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	_, err = db.ExecContext(ctx, `
CREATE TABLE ops_error_logs (
 id bigserial PRIMARY KEY, created_at timestamptz NOT NULL,
 user_id bigint, api_key_id bigint, group_id bigint, platform text,
 request_path text, error_body text, status_code int, upstream_status_code int,
 error_owner text DEFAULT 'provider', is_business_limited boolean DEFAULT false,
 is_count_tokens boolean DEFAULT false);
CREATE TABLE usage_logs (id bigserial PRIMARY KEY, request_id text, group_id bigint,
 account_id bigint, created_at timestamptz, duration_ms int, first_token_ms int,
 input_tokens int DEFAULT 0, output_tokens int DEFAULT 0,
 cache_creation_tokens int DEFAULT 0, cache_read_tokens int DEFAULT 0);
CREATE INDEX usage_request_id ON usage_logs(request_id);
CREATE TABLE groups (id bigint PRIMARY KEY, platform text);
INSERT INTO groups VALUES (98,'openai');
CREATE TABLE ops_metrics_hourly (
 bucket_start timestamptz NOT NULL, platform text, group_id bigint,
 success_count bigint DEFAULT 0, ttft_sample_count bigint DEFAULT 0,
 error_count_total bigint DEFAULT 0, business_limited_count bigint DEFAULT 0,
 error_count_sla bigint DEFAULT 0, upstream_error_count_excl_429_529 bigint DEFAULT 0,
 upstream_429_count bigint DEFAULT 0, upstream_529_count bigint DEFAULT 0,
 token_consumed bigint DEFAULT 0, computed_at timestamptz,
 duration_p50_ms int, duration_p90_ms int, duration_p95_ms int, duration_p99_ms int,
 duration_avg_ms double precision, duration_max_ms int,
 ttft_p50_ms int, ttft_p90_ms int, ttft_p95_ms int, ttft_p99_ms int,
 ttft_avg_ms double precision, ttft_max_ms int);
CREATE UNIQUE INDEX hourly_dim ON ops_metrics_hourly(bucket_start,COALESCE(platform,''),COALESCE(group_id,0));
CREATE TABLE ops_metrics_daily (LIKE ops_metrics_hourly);
ALTER TABLE ops_metrics_daily RENAME COLUMN bucket_start TO bucket_date;
ALTER TABLE ops_metrics_daily ALTER COLUMN bucket_date TYPE date;
CREATE UNIQUE INDEX daily_dim ON ops_metrics_daily(bucket_date,COALESCE(platform,''),COALESCE(group_id,0));
`)
	require.NoError(t, err)
	migrate := func(name string, transaction bool) {
		t.Helper()
		body, err := os.ReadFile(filepath.Join("..", "..", "migrations", name))
		require.NoError(t, err)
		if transaction {
			body = []byte("BEGIN;\n" + string(body) + "\nCOMMIT;")
		}
		_, err = db.ExecContext(ctx, string(body))
		require.NoError(t, err, name)
	}
	migrate("253_ops_media_task_sla.sql", true)
	migrate("254_ops_media_task_sla_index_notx.sql", false)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(3 * time.Hour)
	insert := func(at time.Time, user int, path, body string, code int, limited, probe bool) {
		t.Helper()
		_, err := db.ExecContext(ctx, `INSERT INTO ops_error_logs
 (created_at,user_id,api_key_id,group_id,platform,request_path,error_body,status_code,is_business_limited,is_count_tokens)
 VALUES ($1,$2,$2,98,'openai',$3,$4,$5,$6,$7)`, at, user, path, body, code, limited, probe)
		require.NoError(t, err)
	}
	failure := `{"status":"failed","error":{"message":"safe failure"}}`
	// Retries cross hours, paths and status codes. A later query window must not count it again.
	insert(start, 1213, "/v1/images/generations", `{"task_id":"mtask_first","error":{"type":"generation_failed","message":"safe"}}`, 400, false, false)
	for i := 0; i < 100; i++ {
		insert(start.Add(time.Hour), 1213, "/media/tasks/mtask_first?response_format=state", failure, 502, false, false)
	}
	// Earlier auth/probe/business errors must not consume the task's first failure.
	insert(start, 1213, "/media/tasks/mtask_second", `{"error":{"message":"not found"}}`, 404, false, false)
	insert(start, 1213, "/media/tasks/mtask_second", failure, 429, true, false)
	insert(start, 1213, "/media/tasks/mtask_second", failure, 400, false, true)
	insert(start.Add(time.Hour), 1213, "/v1/images/tasks/mtask_second", `{"status":"recovery_required"}`, 502, false, false)
	// Same timestamp is resolved by row ID, and different users don't suppress each other.
	insert(start.Add(time.Hour), 1213, "/media/tasks/mtask_second", failure, 400, false, false)
	insert(start.Add(time.Hour), 1214, "/media/tasks/mtask_second", failure, 400, false, false)
	// Legacy incorrect 410: one failure. A settled successful/expired task contributes zero.
	insert(start.Add(time.Hour), 1213, "/v1/images/tasks/mtask_legacy", `{"status":"deleted"}`, 410, false, false)
	insert(start.Add(2*time.Hour), 1213, "/v1/images/tasks/mtask_legacy", `{"status":"deleted"}`, 410, false, false)
	insert(start.Add(time.Hour), 1213, "/v1/images/tasks/mtask_success", `{"status":"deleted"}`, 410, false, false)
	_, err = db.ExecContext(ctx, `INSERT INTO usage_logs(request_id,group_id,created_at) VALUES ('media_task_settle:mtask_success',98,$1)`, start)
	require.NoError(t, err)
	// Non-task errors retain request semantics, even with identical paths/bodies.
	for i := 0; i < 2; i++ {
		insert(start.Add(time.Hour), 1213, "/v1/images/generations", failure, 400, false, false)
	}
	// The CPU fix must preserve every row's classification, not just totals.
	_, err = db.ExecContext(ctx, `CREATE TEMP TABLE original_sla AS SELECT id,is_sla_sample FROM ops_error_log_metrics`)
	require.NoError(t, err)
	migrate("266_ops_media_task_sla_cpu.sql", true)
	migrate("268_ops_media_terminal_sla_index_notx.sql", false)
	migrate("269_ops_sla_ordinary_path_fast_path.sql", true)
	original, err := os.ReadFile(filepath.Join("..", "..", "migrations", "253_ops_media_task_sla.sql"))
	require.NoError(t, err)
	originalID := strings.SplitN(string(original), "$$;", 2)[0] + "$$;"
	_, err = db.ExecContext(ctx, strings.Replace(originalID, "ops_media_task_id(", "ops_media_task_id_original(", 1))
	require.NoError(t, err)
	migrate("270_ops_media_task_id_fast_path.sql", true)
	for _, path := range []any{nil, "", "/v1/chat/completions", "/media/tasks/mtask_a", "/v1/images/tasks/mtask_a?response_format=b64_json", "/v1/videos/mtask_a", "/media/tasks/mtask_", "/media/tasks/mtask_a/b", "/media/tasks/mtask_中文", "/media/tasks/mtask_" + strings.Repeat("a", 128), "/media/tasks/mtask_" + strings.Repeat("a", 129), "/v1/images/generations", "/v1/images/edits", "/v1/videos", "/v1/videos/generations"} {
		for _, body := range []any{nil, "", `{"task_id":"mtask_valid"}`, `{"task_id":"mtask_` + strings.Repeat("a", 129) + `","next":{"task_id":"mtask_valid"}}`} {
			var equal bool
			require.NoError(t, db.QueryRowContext(ctx, `SELECT ops_media_task_id($1,$2) IS NOT DISTINCT FROM ops_media_task_id_original($1,$2)`, path, body).Scan(&equal))
			require.True(t, equal, "task ID extraction differs for path %v", path)
		}
	}
	var changed int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM original_sla a JOIN ops_error_log_metrics b USING(id)
WHERE a.is_sla_sample IS DISTINCT FROM b.is_sla_sample`).Scan(&changed))
	require.Zero(t, changed)
	repo := &opsRepository{db: db}
	filter := &service.OpsDashboardFilter{StartTime: start, EndTime: end}
	total, limited, sla, upstream, _, _, err := repo.queryErrorCounts(ctx, filter, start, end)
	require.NoError(t, err)
	require.EqualValues(t, 111, total)
	require.EqualValues(t, 1, limited)
	require.EqualValues(t, 6, sla)
	require.EqualValues(t, 6, upstream)
	_, _, laterSLA, _, _, _, err := repo.queryErrorCounts(ctx, filter, start.Add(2*time.Hour), end)
	require.NoError(t, err)
	require.Zero(t, laterSLA)
	trend, err := repo.GetErrorTrend(ctx, filter, 3600)
	require.NoError(t, err)
	var trendSLA int64
	for _, point := range trend.Points {
		trendSLA += point.ErrorCountSLA
	}
	require.Equal(t, sla, trendSLA)
	distribution, err := repo.GetErrorDistribution(ctx, filter)
	require.NoError(t, err)
	var distributionSLA int64
	for _, item := range distribution.Items {
		distributionSLA += item.SLA
	}
	require.Equal(t, sla, distributionSLA)
	require.NoError(t, repo.UpsertHourlyMetrics(ctx, start, end))
	require.NoError(t, repo.UpsertDailyMetrics(ctx, start, start.Add(24*time.Hour)))
	assertAggregates := func() {
		t.Helper()
		for _, table := range []string{"ops_metrics_hourly", "ops_metrics_daily"} {
			var gotSLA, gotTotal, success int64
			err := db.QueryRowContext(ctx, "SELECT SUM(error_count_sla),SUM(error_count_total),SUM(success_count) FROM "+table+" WHERE platform IS NULL AND group_id IS NULL").Scan(&gotSLA, &gotTotal, &success)
			require.NoError(t, err)
			require.Equal(t, sla, gotSLA, table)
			require.Equal(t, total, gotTotal, table)
			require.EqualValues(t, 1, success, table)
		}
	}
	assertAggregates()
	// Simulate old aggregates, then verify migration repairs history without deleting logs or volume.
	_, err = db.ExecContext(ctx, `UPDATE ops_metrics_hourly SET error_count_sla=error_count_total-business_limited_count;
UPDATE ops_metrics_daily SET error_count_sla=error_count_total-business_limited_count;`)
	require.NoError(t, err)
	migrate("255_ops_media_task_sla_backfill.sql", true)
	assertAggregates()
	migrate("255_ops_media_task_sla_backfill.sql", true)
	assertAggregates()
	var logs int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM ops_error_logs").Scan(&logs))
	require.Equal(t, 112, logs)
	// NULL owners still deduplicate, separately from an authenticated user.
	_, err = db.ExecContext(ctx, `INSERT INTO ops_error_logs(created_at,user_id,request_path,error_body,status_code)
VALUES (now(),NULL,'/media/tasks/mtask_null','{"status":"failed"}',502),
       (now(),NULL,'/media/tasks/mtask_null','{"status":"failed"}',502),
       (now(),2,'/media/tasks/mtask_null','{"status":"failed"}',502)`)
	require.NoError(t, err)
	var samples int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM ops_error_log_metrics
WHERE request_path='/media/tasks/mtask_null' AND is_sla_sample`).Scan(&samples))
	require.Equal(t, 2, samples)
	// Earlier nonterminal polls must not be rescanned for each later terminal failure.
	_, err = db.ExecContext(ctx, `INSERT INTO ops_error_logs(created_at,user_id,request_path,error_body,status_code)
SELECT now()-interval '5 minutes',2,'/media/tasks/mtask_load_'||(n%100),'{"status":"pending"}',404
FROM generate_series(1,10000) n;`)
	require.NoError(t, err)
	// Many polling errors must stay cheap when all four SLA metrics use the view.
	_, err = db.ExecContext(ctx, `INSERT INTO ops_error_logs(created_at,user_id,request_path,error_body,status_code)
SELECT now(),2,'/media/tasks/mtask_load_'||(n%100),'{"status":"failed"}',502
FROM generate_series(1,10000) n; ANALYZE ops_error_logs;`)
	require.NoError(t, err)
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, _, loadSLA, _, _, _, err := repo.queryErrorCounts(queryCtx, filter, time.Now().Add(-time.Hour), time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.EqualValues(t, 102, loadSLA)
	// Fast-path guards preserve classification for ordinary, NULL and malformed paths.
	_, err = db.ExecContext(ctx, `INSERT INTO ops_error_logs(created_at,user_id,request_path,error_body,status_code)
 VALUES(now(),2,'/v1/chat/completions','{"task_id":"mtask_load_1","status":"failed"}',502),
 (now(),2,NULL,'{"status":"failed"}',502),
 (now(),2,'/media/tasks/not-a-media-id','{"status":"failed"}',502)`)
	require.NoError(t, err)
	var mismatches int
	require.NoError(t, db.QueryRowContext(ctx, `SELECT count(*) FROM ops_error_log_metrics WHERE is_sla_sample IS DISTINCT FROM ops_media_task_sla_sample(request_path,error_body,status_code,user_id,created_at,id)`).Scan(&mismatches))
	require.Zero(t, mismatches)
}
