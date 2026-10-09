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

	"github.com/Wei-Shaw/sub2api/internal/appcatalog"
)

var Version = "dev"

func main() {
	listen := flag.String("listen", appcatalog.DefaultListen, "HTTP listen address")
	sqlitePath := flag.String("sqlite-path", "data/appcatalog.db", "SQLite database path")
	pricingPath := flag.String("pricing-path", appcatalog.DefaultProviderPricingPath, "Provider pricing JSON path, read on each request")
	flag.Float64("pricing-multiplier", appcatalog.DefaultProviderPricingMultiplier, "Deprecated and ignored; edit the pricing JSON instead")
	showVersion := flag.Bool("version", false, "Show version information")
	flag.Parse()

	if *showVersion {
		fmt.Printf("appcatalogd %s\n", Version)
		return
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	store, err := appcatalog.OpenStore(*sqlitePath)
	if err != nil {
		logger.Error("open sqlite", "err", err)
		os.Exit(1)
	}
	defer func() { _ = store.Close() }()

	srv := &appcatalog.Server{
		Store:           store,
		Fetcher:         appcatalog.NewFetcher(),
		ProviderPricing: appcatalog.NewProviderPricingService(*pricingPath),
		StartedAt:       time.Now().UTC(),
		Version:         Version,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := &http.Server{
		Addr:              *listen,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		logger.Info("appcatalogd listening", "addr", *listen, "sqlite", *sqlitePath)
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
