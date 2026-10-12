package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/iqdetectd"
)

var Version = "dev"

func main() {
	listen := flag.String("listen", iqdetectd.DefaultListen, "HTTP listen address")
	sqlitePath := flag.String("sqlite-path", "data/iqdetect.db", "SQLite database path")
	upstreamBase := flag.String("api-base-url", iqdetectd.DefaultUpstreamBaseURL, "OpenAI-compatible upstream base URL the probes are sent to")
	apiKey := flag.String("api-key", os.Getenv("IQ_DETECT_API_KEY"), "Bearer key for the upstream API (env IQ_DETECT_API_KEY)")
	timeoutSecs := flag.Int("request-timeout", 60, "Per-probe upstream request timeout in seconds")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("iqdetectd %s\n", Version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	store, err := iqdetectd.OpenStore(*sqlitePath)
	if err != nil {
		logger.Error("open sqlite", "err", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	runner := iqdetectd.NewRunner(store, *upstreamBase, *apiKey, time.Duration(*timeoutSecs)*time.Second, logger)
	testRunner := iqdetectd.NewTestRunner(store, time.Duration(*timeoutSecs)*time.Second, logger)
	srv := &iqdetectd.Server{
		Store:      store,
		Runner:     runner,
		TestRunner: testRunner,
		StartedAt:  time.Now().UTC(),
		Version:    Version,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 启动单调度器协程：每分钟检查各分组间隔是否到期，到期并发执行测试
	testRunner.StartScheduler(ctx)

	// 历史无分组的结果按账号归属回填到当前选择的分组
	if n, err := store.BackfillTestResultGroups(ctx); err != nil {
		logger.Error("iqdetectd: backfill test result groups", "err", err)
	} else if n > 0 {
		logger.Info("iqdetectd: backfilled test result groups", "rows", n)
	}

	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		logger.Info("iqdetectd listening", "addr", *listen, "sqlite", *sqlitePath, "upstream", *upstreamBase)
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
