package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestGroupCacheUsagePostgres(t *testing.T) {
	dsn := os.Getenv("OPS_SLA_TEST_DSN")
	if dsn == "" {
		t.Skip("set OPS_SLA_TEST_DSN to a disposable PostgreSQL database")
	}
	useGroupUsageRepositoryTestTimezone(t, "America/New_York")
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()
	schema := fmt.Sprintf("group_cache_%d", time.Now().UnixNano())
	_, err = db.ExecContext(ctx, "CREATE SCHEMA "+schema+"; SET search_path TO "+schema+"; SET TIME ZONE 'America/New_York'")
	require.NoError(t, err)
	t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DROP SCHEMA "+schema+" CASCADE") })
	exec := func(query string, args ...any) {
		t.Helper()
		_, err := db.ExecContext(ctx, query, args...)
		require.NoError(t, err)
	}
	exec(`CREATE TABLE groups(id bigint PRIMARY KEY);
CREATE TABLE usage_logs(id bigserial PRIMARY KEY,group_id bigint,created_at timestamptz NOT NULL,actual_cost numeric NOT NULL DEFAULT 1,
input_tokens bigint NOT NULL DEFAULT 0, cache_creation_tokens bigint NOT NULL DEFAULT 0,cache_read_tokens bigint NOT NULL DEFAULT 0);
CREATE INDEX ON usage_logs(created_at);
INSERT INTO groups VALUES(7),(8);`)
	for _, name := range []string{"222_group_usage_daily_rollups.sql", "223_group_usage_rollup_timezone.sql", "251_group_usage_rollup_nonblocking_build.sql", "267_group_cache_usage_rollups.sql"} {
		body, err := migrations.FS.ReadFile(name)
		require.NoError(t, err)
		exec("BEGIN;" + string(body) + "COMMIT;")
	}
	now := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)
	today := service.GroupUsageTodayStart(now)
	since24, since7 := now.Add(-24*time.Hour), now.Add(-7*24*time.Hour)
	for i, at := range []time.Time{since7.Add(-time.Microsecond), since7, since24.Add(-time.Microsecond), since24, today.Add(-time.Microsecond), today, now} {
		exec("INSERT INTO usage_logs(group_id,created_at,input_tokens,cache_creation_tokens,cache_read_tokens) VALUES(7,$1,$2,$3,$4)", at, i+1, 2*(i+1), 3*(i+1))
	}
	exec("INSERT INTO usage_logs(group_id,created_at,input_tokens) VALUES(NULL,$1,999)", now)
	r := newUsageLogRepositoryWithSQL(nil, db)
	rollups := newDashboardAggregationRepositoryWithSQL(db)
	check := func() {
		t.Helper()
		rows, err := r.GetGroupCacheUsageSummary(ctx, since24, since7)
		require.NoError(t, err)
		require.Len(t, rows, 2)
		var want usagestats.GroupCacheUsageSummary
		want.GroupID = 7
		for _, window := range []struct {
			start time.Time
			out   *usagestats.GroupCacheUsageWindow
		}{{today, &want.Today}, {since24, &want.Last24h}, {since7, &want.Last7d}, {time.Unix(0, 0), &want.Total}} {
			err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(input_tokens),0),COALESCE(SUM(cache_creation_tokens),0),COALESCE(SUM(cache_read_tokens),0) FROM usage_logs WHERE group_id=7 AND created_at >= $1`, window.start).Scan(&window.out.InputTokens, &window.out.CacheCreationTokens, &window.out.CacheReadTokens)
			require.NoError(t, err)
			window.out.CacheHitRate = cacheHitRate(window.out.InputTokens, window.out.CacheCreationTokens, window.out.CacheReadTokens)
		}
		require.Equal(t, want, rows[0])
		require.Equal(t, usagestats.GroupCacheUsageSummary{GroupID: 8}, rows[1])
	}
	check() // Unbuilt watermark safely uses source records.
	require.NoError(t, rollups.SyncGroupUsageRollups(ctx, today))
	check()
	// Old-version workers can republish cost-only buckets during rollout.
	exec("UPDATE usage_group_daily_rollups SET input_tokens=NULL,cache_creation_tokens=NULL,cache_read_tokens=NULL")
	check()
	require.NoError(t, rollups.SyncGroupUsageRollups(ctx, today))
	check()
	var missing int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT count(*) FROM usage_group_daily_rollups WHERE input_tokens IS NULL").Scan(&missing))
	require.Zero(t, missing)
	for _, mutation := range []string{
		"UPDATE usage_logs SET input_tokens=input_tokens+100,cache_read_tokens=cache_read_tokens+30 WHERE id=2",
		"DELETE FROM usage_logs WHERE id=3",
		"INSERT INTO usage_logs(group_id,created_at,input_tokens,cache_creation_tokens,cache_read_tokens) VALUES(7,'2026-03-03 12:00:00+00',40,50,60)",
	} {
		exec(mutation)
		check()
		require.NoError(t, rollups.SyncGroupUsageRollups(ctx, today))
		check()
	}
	// Retention and a configured timezone change must not reuse invalid buckets.
	exec("DELETE FROM usage_logs WHERE created_at < $1", since7)
	check()
	require.NoError(t, rollups.SyncGroupUsageRollups(ctx, today))
	check()
	exec("UPDATE usage_group_rollup_state SET timezone_name='UTC'")
	check()
	require.NoError(t, rollups.SyncGroupUsageRollups(ctx, today))
	check()
}
