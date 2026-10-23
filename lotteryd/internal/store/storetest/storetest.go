// Package storetest 提供 PostgreSQL 支撑的测试用 Store。
// 设置 LOTTERYD_TEST_PG_DSN 才会运行，否则测试跳过。
// 每个测试进程基于该 DSN 创建一个独立的临时库（lottery_test_<随机后缀>），
// 避免并行跑的包共用一个库互相清表；结束后删除。
package storetest

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"lotteryd/internal/store"
)

// Open 返回一个指向本次运行专用临时库的 Store。
// 未配置 LOTTERYD_TEST_PG_DSN 或 PostgreSQL 不可达时跳过测试。
func Open(t testing.TB) *store.Store {
	t.Helper()
	baseDSN := os.Getenv("LOTTERYD_TEST_PG_DSN")
	if baseDSN == "" {
		t.Skip("set LOTTERYD_TEST_PG_DSN to run PostgreSQL-backed tests")
	}

	dbname := fmt.Sprintf("lottery_test_%d", rand.Int63())
	adminDSN, err := swapDatabase(baseDSN, "postgres")
	if err != nil {
		t.Fatalf("derive admin dsn: %v", err)
	}
	testDSN, err := swapDatabase(baseDSN, dbname)
	if err != nil {
		t.Fatalf("derive test dsn: %v", err)
	}

	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	defer func() { _ = admin.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := admin.PingContext(ctx); err != nil {
		t.Skipf("test PostgreSQL unreachable: %v", err)
	}
	if _, err := admin.ExecContext(ctx, `CREATE DATABASE `+quoteIdent(dbname)); err != nil {
		t.Skipf("create temp test database: %v", err)
	}

	st, err := store.Open(testDSN)
	if err != nil {
		_, _ = admin.ExecContext(ctx, `DROP DATABASE IF EXISTS `+quoteIdent(dbname))
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() {
		_ = st.Close()
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer dropCancel()
		drop, err := sql.Open("pgx", adminDSN)
		if err == nil {
			_, _ = drop.ExecContext(dropCtx, `DROP DATABASE IF EXISTS `+quoteIdent(dbname)+` WITH (FORCE)`)
			_ = drop.Close()
		}
	})
	return st
}

// swapDatabase 把 URL 形式 DSN 的库名换成 target，返回新 DSN。
func swapDatabase(dsn, target string) (string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", err
	}
	u.Path = "/" + target
	return u.String(), nil
}

func testDSNName(dsn string) string {
	u, _ := url.Parse(dsn)
	return strings.TrimPrefix(u.Path, "/")
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
