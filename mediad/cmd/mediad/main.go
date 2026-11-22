// Command mediad 是组装中转媒体服务:接收 sub2api 的全量转发请求,
// 按 schema 组装后请求上游;同步等待返回,异步轮询至终态并回调 sub2api。
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mediad/internal/config"
	"mediad/internal/server"
)

var Version = "dev"
var Commit = "unknown"

func main() {
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Printf("mediad %s (%s)\n", Version, Commit)
		return
	}
	logger := log.Default()
	cfg := config.FromEnv()

	if cfg.RelaySecret == "" {
		logger.Printf("[mediad] WARN MEDIAD_RELAY_SECRET is empty: relay endpoints will return 503")
	}
	if cfg.ManagementToken == "" {
		logger.Printf("[mediad] WARN MEDIAD_MANAGEMENT_TOKEN is empty: management API will return 503")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	srv, err := server.New(cfg, logger)
	if err != nil {
		logger.Fatalf("[mediad] init failed: %v", err)
	}
	if err := srv.EnsureDataDir(); err != nil {
		logger.Fatalf("[mediad] prepare data dir: %v", err)
	}
	if err := srv.Run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "[mediad] fatal:", err)
		os.Exit(1)
	}
	logger.Printf("[mediad] stopped")
}
