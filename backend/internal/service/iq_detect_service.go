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

// IQDetectAccountSource resolves account credentials from the main database
// (satisfied by repository.AccountRepository).
type IQDetectAccountSource interface {
	GetByIDs(ctx context.Context, ids []int64) ([]*Account, error)
}

// IQDetectService is the main-server client for the iqdetectd sidecar. More
// proxied endpoints (models, runs, scores) are added alongside the IQ
// detection admin page.
type IQDetectService struct {
	baseURL  string
	client   *http.Client
	accounts IQDetectAccountSource
}

func NewIQDetectService(cfg *config.Config, accounts IQDetectAccountSource) *IQDetectService {
	base := DefaultIQDetectBaseURL
	if cfg != nil && strings.TrimSpace(cfg.IQDetect.BaseURL) != "" {
		base = strings.TrimRight(strings.TrimSpace(cfg.IQDetect.BaseURL), "/")
	}
	return &IQDetectService{
		baseURL:  base,
		client:   &http.Client{Timeout: 10 * time.Second},
		accounts: accounts,
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
// per-model plan immediately (calls execute in the background). Before
// forwarding, the saved selection is resolved against the main database and
// the accounts' upstream credentials are injected into the run (scheme B), so
// key rotations in the main console take effect immediately.
func (s *IQDetectService) TriggerTestRun(ctx context.Context, groupID int64) (*iqdetectd.TestRunPlan, error) {
	credentials, err := s.resolveSelectionCredentials(ctx)
	if err != nil {
		return nil, err
	}
	// 回写 sidecar 存储的凭据副本，让分组定时任务（不经主服务）也用上最新 Key
	for _, cred := range credentials {
		_ = s.doJSON(ctx, http.MethodPut, "/v1/account-credentials",
			iqdetectd.UpsertAccountCredentialInput(cred),
			nil)
	}
	var plan iqdetectd.TestRunPlan
	if err := s.doJSON(ctx, http.MethodPost, "/v1/test-runs",
		iqdetectd.TriggerTestRunInput{Credentials: credentials, GroupID: groupID}, &plan); err != nil {
		return nil, err
	}
	if plan.Items == nil {
		plan.Items = []iqdetectd.TestRunPlanItem{}
	}
	return &plan, nil
}

// StopTestRun cancels every in-flight detection group on the sidecar
// (scheduled or manual) and returns how many groups were stopped.
func (s *IQDetectService) StopTestRun(ctx context.Context) (int, error) {
	var payload struct {
		Stopped int `json:"stopped"`
	}
	if err := s.doJSON(ctx, http.MethodPost, "/v1/test-runs/stop", nil, &payload); err != nil {
		return 0, err
	}
	return payload.Stopped, nil
}

// resolveSelectionCredentials reads the saved selection from the sidecar and
// resolves each selected account's upstream URL and API key from the main
// database. Only api_key-type accounts qualify; the sidecar falls back to its
// own stored credentials for anything unresolved.
func (s *IQDetectService) resolveSelectionCredentials(ctx context.Context) ([]iqdetectd.TestRunCredential, error) {
	var selection iqdetectd.DetectionSelection
	if err := s.getJSON(ctx, "/v1/selection", &selection); err != nil {
		return nil, err
	}
	idSet := map[int64]bool{}
	for _, group := range selection.Groups {
		for _, account := range group.Accounts {
			idSet[account.ID] = true
		}
	}
	if len(idSet) == 0 || s.accounts == nil {
		return nil, nil
	}
	ids := make([]int64, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	accounts, err := s.accounts.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	credentials := make([]iqdetectd.TestRunCredential, 0, len(accounts))
	for _, account := range accounts {
		if account == nil || account.Type != AccountTypeAPIKey {
			continue
		}
		apiKey := account.GetCredential("api_key")
		if strings.TrimSpace(apiKey) == "" {
			continue
		}
		apiURL := strings.TrimRight(strings.TrimSpace(account.GetCredential("base_url")), "/")
		if apiURL == "" {
			apiURL = strings.TrimRight(account.GetOpenAIBaseURL(), "/")
		}
		if apiURL == "" {
			continue
		}
		credentials = append(credentials, iqdetectd.TestRunCredential{
			AccountID: account.ID,
			APIURL:    apiURL,
			APIKey:    apiKey,
		})
	}
	return credentials, nil
}

// ListTestResults returns the newest detection result pairs (without the
// pelican HTML body; fetch it via GetTestResultPelican). A non-zero groupID
// filters to that group's rows.
func (s *IQDetectService) ListTestResults(ctx context.Context, limit, offset, groupID int64) ([]iqdetectd.TestResult, int, error) {
	var payload struct {
		Items []iqdetectd.TestResult `json:"items"`
		Total int                    `json:"total"`
	}
	path := fmt.Sprintf("/v1/test-results?limit=%d&offset=%d", limit, offset)
	if groupID > 0 {
		path += fmt.Sprintf("&group_id=%d", groupID)
	}
	if err := s.getJSON(ctx, path, &payload); err != nil {
		return nil, 0, err
	}
	if payload.Items == nil {
		payload.Items = []iqdetectd.TestResult{}
	}
	return payload.Items, payload.Total, nil
}

// GetTestResultByID returns one full result pair from the sidecar.
func (s *IQDetectService) GetTestResultByID(ctx context.Context, id int64) (*iqdetectd.TestResult, error) {
	var result iqdetectd.TestResult
	if err := s.getJSON(ctx, fmt.Sprintf("/v1/test-results/%d", id), &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetTestResultPelican returns the raw pelican HTML of one result pair. The
// stored body can reach 2MB, so this fetches past the 1MB JSON proxy limit.
func (s *IQDetectService) GetTestResultPelican(ctx context.Context, id int64) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		fmt.Sprintf("%s/v1/test-results/%d/pelican", s.BaseURL(), id), nil)
	if err != nil {
		return "", err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return "", infraerrors.ServiceUnavailable(IQDetectAgentUnavailableReason, err.Error()).
			WithMetadata(map[string]string{"base_url": s.BaseURL()})
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		msg := strings.TrimSpace(string(raw))
		if len(msg) > 200 {
			msg = msg[:200]
		}
		if msg == "" {
			msg = fmt.Sprintf("iq detect service HTTP %d", resp.StatusCode)
		}
		return "", infraerrors.InternalServer(IQDetectAgentUnavailableReason, msg)
	}
	return string(raw), nil
}

// UpdateTestResultReview records the manual review verdict of one probe on a
// test result pair.
func (s *IQDetectService) UpdateTestResultReview(ctx context.Context, id int64, probe, verdict string) error {
	return s.doJSON(ctx, http.MethodPut, fmt.Sprintf("/v1/test-results/%d/review", id),
		iqdetectd.TestResultReviewInput{Probe: probe, Verdict: verdict}, nil)
}

// IQDetectPublicGroup is one detection group visible to regular users, with
// its recent result metadata (no probe output content).
type IQDetectPublicGroup struct {
	ID              int64                      `json:"id"`
	Name            string                     `json:"name"`
	IntervalMinutes int                        `json:"interval_minutes"`
	Results         []IQDetectPublicResultMeta `json:"results"`
}

// IQDetectPublicResultMeta is the metadata of one test result without the
// probe output content (candy answer / pelican HTML live in the detail API).
type IQDetectPublicResultMeta struct {
	ID               int64     `json:"id"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	CandyCorrect     *bool     `json:"candy_correct"`
	CandyLatencyMS   int64     `json:"candy_latency_ms,omitempty"`
	CandyError       string    `json:"candy_error,omitempty"`
	PelicanOK        bool      `json:"pelican_ok"`
	PelicanLatencyMS int64     `json:"pelican_latency_ms,omitempty"`
	PelicanError     string    `json:"pelican_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// IQDetectPublicResult is the full sanitized view of one test result for
// regular users: no account identifiers or review state. It carries the
// outcome of both probes — the candy final answer and the pelican HTML body —
// plus the per-probe latency.
type IQDetectPublicResult struct {
	ID               int64     `json:"id"`
	GroupID          int64     `json:"group_id"`
	GroupName        string    `json:"group_name"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	CandyAnswer      string    `json:"candy_answer"`
	CandyCorrect     *bool     `json:"candy_correct"`
	CandyLatencyMS   int64     `json:"candy_latency_ms,omitempty"`
	CandyError       string    `json:"candy_error,omitempty"`
	PelicanHTML      string    `json:"pelican_html"`
	PelicanOK        bool      `json:"pelican_ok"`
	PelicanLatencyMS int64     `json:"pelican_latency_ms,omitempty"`
	PelicanError     string    `json:"pelican_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// IQDetectPublicSettings is the detection call configuration shown on the
// widget page alongside the results.
type IQDetectPublicSettings struct {
	Models          []string `json:"models"`
	ReasoningEffort string   `json:"reasoning_effort"`
}

// IQDetectPublicOverview is the payload of the public overview endpoint:
// detection settings, the published groups and their last 48 result metadata.
type IQDetectPublicOverview struct {
	Settings IQDetectPublicSettings `json:"settings"`
	Groups   []IQDetectPublicGroup  `json:"groups"`
}

// publicOverviewResults is how many result metadata rows are returned per
// group in the overview.
const publicOverviewResults = 48

// GetPublicOverview serves the regular-user results page: only groups with
// scheduled detection enabled (interval > 0) are published, and account
// identifiers are stripped from the results.
func (s *IQDetectService) GetPublicOverview(ctx context.Context) (*IQDetectPublicOverview, error) {
	selection, err := s.GetDetectionSelection(ctx)
	if err != nil {
		return nil, err
	}
	out := &IQDetectPublicOverview{Groups: []IQDetectPublicGroup{}}
	// 检测设置随结果一起返回（模型列表 + 推理等级）；失败不阻塞结果输出
	if settings, err := s.GetDetectionModels(ctx); err == nil {
		out.Settings.Models = make([]string, 0, len(settings.Items))
		for _, dm := range settings.Items {
			out.Settings.Models = append(out.Settings.Models, dm.Name)
		}
		out.Settings.ReasoningEffort = settings.ReasoningEffort
	}
	for _, group := range selection.Groups {
		if group.IntervalMinutes <= 0 {
			continue
		}
		entry := IQDetectPublicGroup{
			ID: group.ID, Name: group.Name, IntervalMinutes: group.IntervalMinutes,
			Results: []IQDetectPublicResultMeta{},
		}
		results, _, err := s.ListTestResults(ctx, publicOverviewResults, 0, group.ID)
		if err == nil {
			for _, result := range results {
				if result.GroupID != group.ID {
					continue
				}
				entry.Results = append(entry.Results, IQDetectPublicResultMeta{
					ID:               result.ID,
					Model:            result.Model,
					Status:           result.Status,
					CandyCorrect:     result.CandyCorrect,
					CandyLatencyMS:   result.CandyLatencyMS,
					CandyError:       result.CandyError,
					PelicanOK:        result.PelicanError == "" && result.PelicanHTMLLen > 0,
					PelicanLatencyMS: result.PelicanLatencyMS,
					PelicanError:     result.PelicanError,
					CreatedAt:        result.CreatedAt,
				})
			}
		}
		out.Groups = append(out.Groups, entry)
	}
	return out, nil
}

// GetPublicResultDetail returns the full sanitized content of one result pair
// (candy final answer + pelican HTML). The pair is only served when its group
// is published.
func (s *IQDetectService) GetPublicResultDetail(ctx context.Context, id int64) (*IQDetectPublicResult, error) {
	result, err := s.GetTestResultByID(ctx, id)
	if err != nil {
		return nil, err
	}
	selection, err := s.GetDetectionSelection(ctx)
	if err != nil {
		return nil, err
	}
	published := false
	for _, group := range selection.Groups {
		if group.ID == result.GroupID && group.IntervalMinutes > 0 {
			published = true
			break
		}
	}
	if !published {
		return nil, fmt.Errorf("test result %d not found", id)
	}
	pelicanHTML := ""
	if result.PelicanHTMLLen > 0 && result.PelicanError == "" {
		if html, err := s.GetTestResultPelican(ctx, result.ID); err == nil {
			pelicanHTML = html
		}
	}
	return &IQDetectPublicResult{
		ID:               result.ID,
		GroupID:          result.GroupID,
		GroupName:        result.GroupName,
		Model:            result.Model,
		Status:           result.Status,
		CandyAnswer:      iqdetectd.ExtractFinalAnswer(result.CandyAnswer),
		CandyCorrect:     result.CandyCorrect,
		CandyLatencyMS:   result.CandyLatencyMS,
		CandyError:       result.CandyError,
		PelicanHTML:      pelicanHTML,
		PelicanOK:        result.PelicanError == "" && result.PelicanHTMLLen > 0,
		PelicanLatencyMS: result.PelicanLatencyMS,
		PelicanError:     result.PelicanError,
		CreatedAt:        result.CreatedAt,
	}, nil
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
