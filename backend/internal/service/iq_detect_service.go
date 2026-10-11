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

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/iqdetectd"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	DefaultIQDetectBaseURL         = "http://127.0.0.1:18102"
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

// ListAccountCredentials returns the per-account upstream credentials stored
// in the sidecar (API keys are write-only and never included).
func (s *IQDetectService) ListAccountCredentials(ctx context.Context) ([]iqdetectd.AccountCredential, error) {
	var payload struct {
		Items []iqdetectd.AccountCredential `json:"items"`
	}
	if err := s.getJSON(ctx, "/v1/account-credentials", &payload); err != nil {
		return nil, err
	}
	if payload.Items == nil {
		return []iqdetectd.AccountCredential{}, nil
	}
	return payload.Items, nil
}

func (s *IQDetectService) UpsertAccountCredential(ctx context.Context, input iqdetectd.UpsertAccountCredentialInput) (*iqdetectd.AccountCredential, error) {
	var credential iqdetectd.AccountCredential
	if err := s.doJSON(ctx, http.MethodPut, "/v1/account-credentials", input, &credential); err != nil {
		return nil, err
	}
	return &credential, nil
}

// GetDetectionSelection returns the persisted group/account/model selection.
func (s *IQDetectService) GetDetectionSelection(ctx context.Context) (*iqdetectd.DetectionSelection, error) {
	var selection iqdetectd.DetectionSelection
	if err := s.getJSON(ctx, "/v1/selection", &selection); err != nil {
		return nil, err
	}
	if selection.Groups == nil {
		selection.Groups = []iqdetectd.SelectionGroup{}
	}
	return &selection, nil
}

func (s *IQDetectService) SaveDetectionSelection(ctx context.Context, groups []iqdetectd.SelectionGroup) (*iqdetectd.DetectionSelection, error) {
	var selection iqdetectd.DetectionSelection
	if err := s.doJSON(ctx, http.MethodPut, "/v1/selection", iqdetectd.DetectionSelection{Groups: groups}, &selection); err != nil {
		return nil, err
	}
	if selection.Groups == nil {
		selection.Groups = []iqdetectd.SelectionGroup{}
	}
	return &selection, nil
}

func (s *IQDetectService) GetDetectionModels(ctx context.Context) (*iqdetectd.DetectionModelSettings, error) {
	var payload iqdetectd.DetectionModelSettings
	if err := s.getJSON(ctx, "/v1/detection-models", &payload); err != nil {
		return nil, err
	}
	if payload.Items == nil {
		payload.Items = []iqdetectd.DetectionModel{}
	}
	return &payload, nil
}

func (s *IQDetectService) ReplaceDetectionModels(ctx context.Context, input iqdetectd.PutDetectionModelsInput) (*iqdetectd.DetectionModelSettings, error) {
	var payload iqdetectd.DetectionModelSettings
	if err := s.doJSON(ctx, http.MethodPut, "/v1/detection-models", input, &payload); err != nil {
		return nil, err
	}
	if payload.Items == nil {
		payload.Items = []iqdetectd.DetectionModel{}
	}
	return &payload, nil
}

// TriggerTestRun launches a detection test run on the sidecar and returns the
// per-model plan immediately (calls execute in the background).
func (s *IQDetectService) TriggerTestRun(ctx context.Context) (*iqdetectd.TestRunPlan, error) {
	var plan iqdetectd.TestRunPlan
	if err := s.doJSON(ctx, http.MethodPost, "/v1/test-runs", nil, &plan); err != nil {
		return nil, err
	}
	if plan.Items == nil {
		plan.Items = []iqdetectd.TestRunPlanItem{}
	}
	return &plan, nil
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

func (s *IQDetectService) doJSON(ctx context.Context, method, path string, body any, dest any) error {
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
		return infraerrors.ServiceUnavailable(IQDetectAgentUnavailableReason, err.Error()).
			WithMetadata(map[string]string{"base_url": s.BaseURL()})
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
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
			msg = fmt.Sprintf("iq detect service HTTP %d", resp.StatusCode)
		}
		return infraerrors.InternalServer(IQDetectAgentUnavailableReason, msg)
	}
	if dest == nil || len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dest)
}
