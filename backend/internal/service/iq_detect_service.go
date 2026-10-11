package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DefaultIQDetectBaseURL         = "http://127.0.0.1:18100"
	IQDetectAgentUnavailableReason = "IQ_DETECT_AGENT_UNAVAILABLE"
)

// IQDetectService is the main-server client for the iqdetectd sidecar. More
// proxied endpoints (models, runs, scores) are added alongside the IQ
// detection admin page.
type IQDetectService struct {
	baseURL string
	client  *http.Client
}

func NewIQDetectService(cfg *config.Config) *IQDetectService {
	base := DefaultIQDetectBaseURL
	if cfg != nil && strings.TrimSpace(cfg.IQDetect.BaseURL) != "" {
		base = strings.TrimRight(strings.TrimSpace(cfg.IQDetect.BaseURL), "/")
	}
	return &IQDetectService{
		baseURL: base,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *IQDetectService) BaseURL() string {
	if s == nil || s.baseURL == "" {
		return DefaultIQDetectBaseURL
	}
	return s.baseURL
}

type IQDetectAgentHealth struct {
	Enabled    bool   `json:"enabled"`
	Reason     string `json:"reason,omitempty"`
	BaseURL    string `json:"base_url"`
	Status     string `json:"status,omitempty"`
	Version    string `json:"version,omitempty"`
	UptimeSecs int64  `json:"uptime_seconds,omitempty"`
}

func (s *IQDetectService) GetAgentHealth(ctx context.Context) IQDetectAgentHealth {
	health := IQDetectAgentHealth{BaseURL: s.BaseURL()}
	var payload struct {
		Status        string `json:"status"`
		Version       string `json:"version"`
		UptimeSeconds int64  `json:"uptime_seconds"`
	}
	if err := s.getJSON(ctx, "/health", &payload); err != nil {
		health.Reason = IQDetectAgentUnavailableReason
		return health
	}
	health.Enabled = true
	health.Status = payload.Status
	health.Version = payload.Version
	health.UptimeSecs = payload.UptimeSeconds
	return health
}

func (s *IQDetectService) EnsureEnabled(ctx context.Context) error {
	health := s.GetAgentHealth(ctx)
	if health.Enabled {
		return nil
	}
	return infraerrors.ServiceUnavailable(IQDetectAgentUnavailableReason, "iq detect service is unavailable").
		WithMetadata(map[string]string{"base_url": s.BaseURL()})
}

func (s *IQDetectService) getJSON(ctx context.Context, path string, dest any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.BaseURL()+path, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return infraerrors.ServiceUnavailable(IQDetectAgentUnavailableReason, err.Error()).
			WithMetadata(map[string]string{"base_url": s.BaseURL()})
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(raw))
		if msg == "" {
			msg = fmt.Sprintf("iq detect service HTTP %d", resp.StatusCode)
		}
		return infraerrors.InternalServer(IQDetectAgentUnavailableReason, msg)
	}
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
