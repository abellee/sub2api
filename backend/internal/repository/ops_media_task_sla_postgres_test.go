package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
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
}
