package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

func TestAppCatalogServiceListAndHealth(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok", "version": "test", "uptime_seconds": 1})
	})
	mux.HandleFunc("/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"items": []any{}})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	svc := NewAppCatalogService(&config.Config{AppCatalog: config.AppCatalogConfig{BaseURL: srv.URL}})
	health := svc.GetAgentHealth(context.Background())
	if !health.Enabled || health.Status != "ok" {
		t.Fatalf("health=%+v", health)
	}
	items, err := svc.ListApps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("items=%d", len(items))
	}
}

func TestAppCatalogServiceUnavailable(t *testing.T) {
	svc := NewAppCatalogService(&config.Config{AppCatalog: config.AppCatalogConfig{BaseURL: "http://127.0.0.1:1"}})
	health := svc.GetAgentHealth(context.Background())
	if health.Enabled {
		t.Fatal("expected unavailable")
	}
	if err := svc.EnsureEnabled(context.Background()); err == nil {
		t.Fatal("expected error")
	}
}
