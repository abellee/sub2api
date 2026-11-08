// Package server 装配媒体中转服务，保存媒体文件，不参与扣费。
package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"mediad/internal/config"
	"mediad/internal/relay"
)

type Server struct {
	httpServer *http.Server
	tracker    *relay.Tracker
	store      *relay.MemoryStore
	logger     *log.Logger
	dataDir    string
	settings   *config.SettingsStore
}

func New(cfg config.Config, logger *log.Logger) (*Server, error) {
	if logger == nil {
		logger = log.Default()
	}
	settingsPath := os.Getenv("MEDIAD_CONFIG_PATH")
	if settingsPath == "" {
		settingsPath = filepath.Join(cfg.DataDir, "settings.json")
	}
	settings, err := config.OpenSettings(settingsPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	cfg = settings.Active().Config()
	if !cfg.PersistCredentials {
		return nil, fmt.Errorf("durable recovery requires MEDIAD_PERSIST_CREDENTIALS=true; protect the data directory")
	}
	store, err := relay.NewDiskStore(filepath.Join(cfg.DataDir, "tasks"))
	if err != nil {
		return nil, err
	}
	// 旧快照迁移支持中途退出后继续；完成后改名，避免已清理任务被再次导入。
	legacy := filepath.Join(cfg.DataDir, "state.json")
	if _, err := os.Stat(legacy); err == nil {
		if _, err := relay.LoadSnapshot(legacy, store); err != nil {
			return nil, err
		}
		if err := os.Rename(legacy, legacy+".migrated"); err != nil {
			return nil, err
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	registry := relay.NewSchemaRegistry(nil)
	if err := registry.Load(filepath.Join(cfg.DataDir, "schemas.json")); err != nil {
		return nil, err
	}
	tracker := relay.NewTracker(store, func(id string) (*relay.Schema, bool) { return registry.Get(id) }, &http.Client{Timeout: 0}, logger)
	if err := tracker.LoadCallback(filepath.Join(cfg.DataDir, "callback.json")); err != nil {
		tracker.Stop()
		return nil, err
	}
	resultDir := cfg.ResultDir
	if resultDir == "" {
		resultDir = filepath.Join(cfg.DataDir, "results")
	}
	tracker.ConfigureFiles(resultDir, cfg.MaxFileBytes)
	tracker.SetCreateTimeout(time.Duration(cfg.MaxSyncSeconds) * time.Second)
	runtime := settings.Active()
	tracker.ConfigureDownloads(runtime.DownloadBaseURL)
	tracker.ConfigureRuntime(runtime.MaxConcurrent, runtime.PollIntervalSeconds, runtime.MaxPollSeconds, runtime.DownloadTimeoutSeconds, runtime.DownloadAttempts)
	relayHandler := relay.NewRelayHandler(registry, store, tracker, cfg.RelaySecret, logger)
	mux := http.NewServeMux()
	relayHandler.Register(mux)
	management := relay.NewManagementHandler(registry, store, tracker, cfg.ManagementToken)
	management.Settings = settings
	management.Register(mux)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200); _, _ = w.Write([]byte("ok")) })
	return &Server{httpServer: &http.Server{Addr: cfg.Listen, Handler: mux, ReadHeaderTimeout: 15 * time.Second, IdleTimeout: 120 * time.Second}, tracker: tracker, store: store, logger: logger, dataDir: cfg.DataDir, settings: settings}, nil
}

func (s *Server) Run(ctx context.Context) error {
	s.tracker.ResumeAll()
	defer s.tracker.Stop()
	errCh := make(chan error, 1)
	go func() { errCh <- s.httpServer.ListenAndServe() }()
	gc := time.NewTicker(time.Duration(s.settings.Active().GCMinutes) * time.Minute)
	defer gc.Stop()
	for {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			return s.httpServer.Shutdown(shutdown)
		case <-gc.C:
			if n, err := s.store.GCFiles(s.settings.FileRetentionHours(), s.settings.Active().ResultDir); err != nil {
				s.logger.Printf("[mediad] file cleanup: %v", err)
			} else if n > 0 {
				s.logger.Printf("[mediad] expired media cleaned: %d tasks", n)
			}
			s.store.GC(time.Duration(s.settings.Active().TaskRetentionHours) * time.Hour)
		case err := <-errCh:
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return err
		}
	}
}

func (s *Server) EnsureDataDir() error { return os.MkdirAll(s.dataDir, 0700) }
