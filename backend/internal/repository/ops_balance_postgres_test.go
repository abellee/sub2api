package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserBalancePeriodsPostgres(t *testing.T) {
	dsn := os.Getenv("OPS_SLA_TEST_DSN")
	if dsn == "" {
		t.Skip("set OPS_SLA_TEST_DSN to a disposable local database")
	}
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer db.Close()
	schema := fmt.Sprintf("balance_errors_%d", time.Now().UnixNano())
	_, err = db.Exec("CREATE SCHEMA " + schema)
	require.NoError(t, err)
	defer db.Exec("DROP SCHEMA " + schema + " CASCADE")
	conn, err := db.Conn(context.Background())
	require.NoError(t, err)
	defer conn.Close()
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		_, e := conn.ExecContext(ctx, query, args...)
		require.NoError(t, e)
	}
	exec("SET search_path TO " + schema)
	exec(`CREATE TABLE users(id bigint PRIMARY KEY,balance numeric NOT NULL);
 CREATE TABLE ops_error_logs(id bigserial PRIMARY KEY,user_id bigint,created_at timestamptz NOT NULL,
 error_type text NOT NULL,error_phase text NOT NULL DEFAULT 'request',error_owner text NOT NULL DEFAULT 'client',
 is_business_limited boolean NOT NULL DEFAULT false);
 INSERT INTO users VALUES (2,0),(3,0);`)
	migration, err := os.ReadFile(filepath.Join("..", "..", "migrations", "260_user_balance_error_periods.sql"))
	require.NoError(t, err)
	exec(string(migration))
	at := time.Now().Add(-time.Hour)
	insert := `INSERT INTO ops_error_logs(user_id,created_at,error_type) VALUES ($1,$2,'insufficient_balance')`
	exec(insert, 2, at)
	exec(insert, 2, at.Add(time.Second))
	exec(insert, 3, at)
	count := func(want int) {
		t.Helper()
		var n int
		require.NoError(t, conn.QueryRowContext(ctx, "SELECT count(*) FROM ops_error_logs WHERE user_id=2").Scan(&n))
		require.Equal(t, want, n)
	}
	count(1)
	exec("DELETE FROM ops_error_logs WHERE user_id=2")
	exec("UPDATE users SET balance=-1 WHERE id=2")
	exec(insert, 2, at.Add(2*time.Second))
	count(0) // retention/debits do not reset
	exec("UPDATE users SET balance=10 WHERE id=2")
	exec(insert, 2, at.Add(3*time.Second))
	count(0) // delayed pre-credit log
	var wg sync.WaitGroup
	failures := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, e := db.Conn(ctx)
			if e != nil {
				failures <- e
				return
			}
			defer c.Close()
			_, e = c.ExecContext(ctx, "SET search_path TO "+schema)
			if e == nil {
				_, e = c.ExecContext(ctx, insert, 2, time.Now())
			}
			failures <- e
		}()
	}
	wg.Wait()
	close(failures)
	for e := range failures {
		require.NoError(t, e)
	}
	count(1)
	exec("UPDATE users SET balance=20 WHERE id=2")
	exec(insert, 2, time.Now())
	count(2)
	var nonBusiness int
	require.NoError(t, conn.QueryRowContext(ctx, "SELECT count(*) FROM ops_error_logs WHERE NOT is_business_limited").Scan(&nonBusiness))
	require.Zero(t, nonBusiness)
	exec(`INSERT INTO ops_error_logs(user_id,created_at,error_type,error_phase,error_owner) VALUES (2,now(),'insufficient_balance','upstream','provider')`)
	count(3)
	// Test the real repository's single/batch insert contract when a trigger skips rows.
	exec(`ALTER TABLE ops_error_logs ADD request_id text, ADD client_request_id text, ADD api_key_id bigint,
 ADD account_id bigint, ADD group_id bigint, ADD client_ip text, ADD platform text, ADD model text,
 ADD request_path text, ADD stream boolean, ADD inbound_endpoint text, ADD upstream_endpoint text,
 ADD requested_model text, ADD upstream_model text, ADD request_type smallint, ADD user_agent text,
 ADD severity text, ADD status_code int, ADD is_count_tokens boolean, ADD error_message text,
 ADD error_body text, ADD error_source text, ADD upstream_status_code int, ADD upstream_error_message text,
 ADD upstream_error_detail text, ADD upstream_errors text, ADD auth_latency_ms bigint,
 ADD routing_latency_ms bigint, ADD upstream_latency_ms bigint, ADD response_latency_ms bigint,
 ADD time_to_first_token_ms bigint, ADD api_key_prefix text;`)
	// A second pool pins search_path for all repository transactions.
	repoDB, err := sql.Open("postgres", dsn+" search_path="+schema)
	require.NoError(t, err)
	defer repoDB.Close()
	repo := NewOpsRepository(repoDB)
	user := int64(2)
	input := &service.OpsInsertErrorLogInput{UserID: &user, CreatedAt: time.Now(), ErrorType: service.OpsUserBalanceErrorType, ErrorPhase: "request", ErrorOwner: "client"}
	id, err := repo.InsertErrorLog(ctx, input)
	require.NoError(t, err)
	require.Zero(t, id)
	n, err := repo.BatchInsertErrorLogs(ctx, []*service.OpsInsertErrorLogInput{input, input})
	require.NoError(t, err)
	require.Zero(t, n)
	count(3)
}
