// lotteryd is a standalone lottery sidecar for Sub2API, modeled after the
// appcatalogd deployment pattern: an independent process with SQLite storage
// that talks to the main service over HTTP APIs.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"lotteryd/internal/app"
	"lotteryd/internal/server"
	"lotteryd/internal/store"
	"lotteryd/internal/sub2api"
)

var Version = "dev"

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func main() {
	var (
		listen     = flag.String("listen", envOr("LOTTERYD_LISTEN", "127.0.0.1:18100"), "HTTP listen address")
		sqlitePath = flag.String("sqlite-path", envOr("LOTTERYD_SQLITE_PATH", "data/lottery.db"), "SQLite database path")
		sub2apiURL = flag.String("sub2api-url", envOr("LOTTERYD_SUB2API_URL", "http://127.0.0.1:8080"), "Sub2API base URL")
		adminKey   = flag.String("admin-api-key", envOr("LOTTERYD_ADMIN_API_KEY", ""), "Sub2API admin API key (x-api-key)")
		jwtSecret  = flag.String("jwt-secret", envOr("LOTTERYD_JWT_SECRET", ""), "Sub2API jwt.secret for local token validation (not required in introspect mode)")
		authMode   = flag.String("auth-mode", envOr("LOTTERYD_AUTH_MODE", "local"), "Auth mode: local (shared jwt.secret) or introspect (verify via /auth/me, no secret needed)")
		corsRaw    = flag.String("cors-origins", envOr("LOTTERYD_CORS_ORIGINS", ""), "Comma-separated allowed CORS origins (empty = same-origin only)")
		syncEvery  = flag.Duration("sync-interval", envDuration("LOTTERYD_SYNC_INTERVAL", time.Hour), "Usage data sync interval")
		drawEvery  = flag.Duration("draw-interval", envDuration("LOTTERYD_DRAW_INTERVAL", 30*time.Second), "Draw scheduler interval")
		backfill   = flag.Int("backfill-days", 35, "Days of usage data to backfill when no watermark exists")
		showVer    = flag.Bool("version", false, "Show version information")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("lotteryd %s\n", Version)
		return
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	if *authMode != "introspect" && *jwtSecret == "" {
		logger.Error("jwt-secret is required in local auth mode (flag, LOTTERYD_JWT_SECRET, or -auth-mode=introspect)")
		os.Exit(1)
	}

	st, err := store.Open(*sqlitePath)
	if err != nil {
		logger.Error("open sqlite", "err", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	client := sub2api.NewClient(*sub2apiURL, *adminKey)
	ap := app.New(st, client, app.Config{BackfillDays: *backfill})
	// 恢复管理页保存的运行时设置（优先级高于启动参数）
	ap.LoadRuntimeSettings()
	if ap.Sub2API.AdminAPIKey() == "" {
		logger.Warn("admin api key not configured: usage sync and prize fulfillment will fail until set in 抽奖管理 → 设置")
	}
	// 恢复管理页保存的运行时设置（优先级高于启动参数）
	ap.LoadRuntimeSettings()

	if raw := strings.TrimSpace(*corsRaw); raw != "" {
		var origins []string
		for _, o := range strings.Split(raw, ",") {
			if o = strings.TrimSpace(o); o != "" {
				origins = append(origins, o)
			}
		}
		server.SetCORSOrigins(origins)
	}

	srv := server.New(ap, *jwtSecret, Version)
	srv.SetAuthMode(*authMode)
	if *authMode == "introspect" && *jwtSecret != "" {
		logger.Info("auth mode introspect: jwt-secret is ignored, identity verified via /auth/me")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Usage data sync loop.
	go func() {
		ticker := time.NewTicker(*syncEvery)
		defer ticker.Stop()
		if err := ap.SyncTokens(ctx, time.Now()); err != nil {
			slog.Warn("initial usage sync failed", "err", err)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := ap.SyncTokens(ctx, time.Now()); err != nil {
					slog.Warn("usage sync failed", "err", err)
				}
			}
		}
	}()

	// WebSocket 推送循环：资格变化时实时通知在线用户。
	go srv.RunWSHub(ctx)

	// Draw scheduler loop.
	go func() {
		ticker := time.NewTicker(*drawEvery)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := ap.DrawDueActivities(ctx, time.Now()); err != nil {
					slog.Warn("draw scheduler failed", "err", err)
				}
				// 日常定时抽奖：到达每日开启时刻自动创建当天的活动。
				if err := ap.CreateDueDailyActivity(time.Now()); err != nil {
					slog.Warn("daily activity scheduler failed", "err", err)
				}
				// 任务结算：到达结算时刻自动结算前一日消耗并发放奖励。
				ap.SettleDueTasks(ctx, time.Now())
			}
		}
	}()

	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		logger.Info("lotteryd listening", "addr", *listen, "sqlite", *sqlitePath, "sub2api", *sub2apiURL)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}

func envDuration(key string, fallback time.Duration) time.Duration {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return fallback
}
