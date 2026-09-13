package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/appcatalog"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DefaultAppCatalogBaseURL         = "http://127.0.0.1:18099"
	AppCatalogAgentUnavailableReason = "APP_CATALOG_AGENT_UNAVAILABLE"
)

type AppCatalogService struct {
	baseURL string
	client  *http.Client
}

func NewAppCatalogService(cfg *config.Config) *AppCatalogService {
	base := DefaultAppCatalogBaseURL
	if cfg != nil && strings.TrimSpace(cfg.AppCatalog.BaseURL) != "" {
		base = strings.TrimRight(strings.TrimSpace(cfg.AppCatalog.BaseURL), "/")
	}
	return &AppCatalogService{
		baseURL: base,
		client:  &http.Client{Timeout: 90 * time.Second},
	}
}

func (s *AppCatalogService) BaseURL() string {
	if s == nil || s.baseURL == "" {
		return DefaultAppCatalogBaseURL
	}
	return s.baseURL
}

type AppCatalogAgentHealth struct {
	Enabled    bool   `json:"enabled"`
	Reason     string `json:"reason,omitempty"`
	BaseURL    string `json:"base_url"`
	Status     string `json:"status,omitempty"`
	Version    string `json:"version,omitempty"`
	UptimeSecs int64  `json:"uptime_seconds,omitempty"`
}

func (s *AppCatalogService) GetAgentHealth(ctx context.Context) AppCatalogAgentHealth {
	health := AppCatalogAgentHealth{BaseURL: s.BaseURL()}
	var payload struct {
		Status        string `json:"status"`
		Version       string `json:"version"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}
	if err := s.getJSON(ctx, "/health", &payload); err != nil {
		health.Reason = AppCatalogAgentUnavailableReason
		return health
	}
	health.Enabled = true
	health.Status = payload.Status
	health.Version = payload.Version
	health.UptimeSecs = payload.UptimeSeconds
	return health
}

func (s *AppCatalogService) EnsureEnabled(ctx context.Context) error {
	health := s.GetAgentHealth(ctx)
	if health.Enabled {
		return nil
	}
	return infraerrors.ServiceUnavailable(AppCatalogAgentUnavailableReason, "app catalog service is unavailable").
		WithMetadata(map[string]string{"base_url": s.BaseURL()})
}

func (s *AppCatalogService) ListApps(ctx context.Context) ([]appcatalog.App, error) {
	var payload struct {
		Items []appcatalog.App `json:"items"`
	}
	if err := s.getJSON(ctx, "/v1/apps", &payload); err != nil {
		return nil, err
	}
	if payload.Items == nil {
		return []appcatalog.App{}, nil
	}
	return payload.Items, nil
}

func (s *AppCatalogService) CreateApp(ctx context.Context, input appcatalog.CreateAppInput) (*appcatalog.App, error) {
	var app appcatalog.App
	if err := s.doJSON(ctx, http.MethodPost, "/v1/apps", input, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func (s *AppCatalogService) UpdateApp(ctx context.Context, id int64, input appcatalog.CreateAppInput) (*appcatalog.App, error) {
	var app appcatalog.App
	if err := s.doJSON(ctx, http.MethodPut, fmt.Sprintf("/v1/apps/%d", id), input, &app); err != nil {
		return nil, err
	}
	return &app, nil
}

func (s *AppCatalogService) FetchPreview(ctx context.Context, input appcatalog.CreateAppInput) (*appcatalog.FetchResult, error) {
	var result appcatalog.FetchResult
	if err := s.doJSON(ctx, http.MethodPost, "/v1/fetch", input, &result); err != nil {
		return nil, err
	}
	if result.Screenshots == nil {
		result.Screenshots = []string{}
	}
	return &result, nil
}

func (s *AppCatalogService) DeleteApp(ctx context.Context, id int64) error {
	return s.doJSON(ctx, http.MethodDelete, fmt.Sprintf("/v1/apps/%d", id), nil, nil)
}

func (s *AppCatalogService) getJSON(ctx context.Context, path string, dest any) error {
	return s.doJSON(ctx, http.MethodGet, path, nil, dest)
}

func (s *AppCatalogService) doJSON(ctx context.Context, method, path string, body any, dest any) error {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, method, s.BaseURL()+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return infraerrors.ServiceUnavailable(AppCatalogAgentUnavailableReason, err.Error()).
			WithMetadata(map[string]string{"base_url": s.BaseURL()})
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		var envelope struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(raw, &envelope)
		msg := strings.TrimSpace(envelope.Error)
		if msg == "" {
			msg = strings.TrimSpace(string(raw))
		}
		if msg == "" {
			msg = fmt.Sprintf("app catalog service HTTP %d", resp.StatusCode)
		}
		if resp.StatusCode == http.StatusBadRequest {
			return infraerrors.BadRequest("APP_CATALOG_INVALID", msg)
		}
		if resp.StatusCode == http.StatusNotFound {
			return infraerrors.NotFound("APP_CATALOG_NOT_FOUND", msg)
		}
		if resp.StatusCode == http.StatusServiceUnavailable {
			return infraerrors.ServiceUnavailable(AppCatalogAgentUnavailableReason, msg)
		}
		return infraerrors.InternalServer("APP_CATALOG_ERROR", msg)
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
