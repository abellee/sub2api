package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// SQLStateDatabaseNotExist PostgreSQL 数据库不存在（invalid_catalog_name）。
const SQLStateDatabaseNotExist = "3D000"

// EnsureDatabase 当 DSN 指向的数据库不存在时，连到同实例的 postgres 库
// 执行 CREATE DATABASE。库已存在或无权限建库时返回 nil / 原始错误。
func EnsureDatabase(dsn string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	err = db.PingContext(ctx)
	_ = db.Close()
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != SQLStateDatabaseNotExist {
		return err
	}

	adminDSN, dbname, err := adminDSNFor(dsn)
	if err != nil {
		return err
	}
	admin, err := sql.Open("pgx", adminDSN)
	if err != nil {
		return err
	}
	defer func() { _ = admin.Close() }()
	if _, err := admin.ExecContext(ctx,
		`CREATE DATABASE `+quoteIdent(dbname)); err != nil {
		// 并发创建等场景下已存在即视为成功。
		var dup *pgconn.PgError
		if errors.As(err, &dup) && dup.Code == "42P04" {
			return nil
		}
		return fmt.Errorf("create database %s: %w", dbname, err)
	}
	return nil
}

// adminDSNFor 把 DSN 的 dbname 换成 postgres 引导库，返回原库名。
func adminDSNFor(dsn string) (string, string, error) {
	if strings.HasPrefix(strings.TrimSpace(dsn), "postgres://") || strings.HasPrefix(strings.TrimSpace(dsn), "postgresql://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return "", "", err
		}
		if u.Path == "" || u.Path == "/" {
			return "", "", fmt.Errorf("dsn has no database name: %s", dsn)
		}
		dbname := strings.TrimPrefix(u.Path, "/")
		u.Path = "/postgres"
		return u.String(), dbname, nil
	}
	// keyword=value 形式：替换 dbname=xxx。
	if v := urlValue(dsn, "dbname"); v != "" {
		return replaceURLValue(dsn, "dbname", "postgres"), v, nil
	}
	return "", "", fmt.Errorf("dsn has no database name: %s", dsn)
}

func urlValue(dsn, key string) string {
	for _, part := range strings.Fields(dsn) {
		if kv := strings.SplitN(part, "=", 2); len(kv) == 2 && kv[0] == key {
			return kv[1]
		}
	}
	return ""
}

func replaceURLValue(dsn, key, val string) string {
	parts := strings.Fields(dsn)
	for i, part := range parts {
		if kv := strings.SplitN(part, "=", 2); len(kv) == 2 && kv[0] == key {
			parts[i] = key + "=" + val
		}
	}
	return strings.Join(parts, " ")
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
