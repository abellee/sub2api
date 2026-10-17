package iqdetectd

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// 公开只读接口给 widget.llmfree.work 上的结果页。响应形状放在 iqdetectd 里，
// 后续改字段只发这个服务，不发主站。无登录，只出检测间隔大于 0 的分组，
// 不带账号标识、提示词和正确答案配置。

const (
	publicOverviewResults = 48
	publicRatePerMinute   = 300
)

type publicEnvelope struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// publicGroup is one detection group visible on the results page.
type publicGroup struct {
	ID              int64              `json:"id"`
	Name            string             `json:"name"`
	IntervalMinutes int                `json:"interval_minutes"`
	Results         []publicResultMeta `json:"results"`
}

// publicResultMeta is one timeline row. Probe bodies stay on the detail route.
// Reviews stay pending until a person approves or rejects. CandyCorrect is
// only the automatic grade.
type publicResultMeta struct {
	ID               int64     `json:"id"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	CandyCorrect     *bool     `json:"candy_correct"`
	CandyReview      string    `json:"candy_review"`
	CandyLatencyMS   int64     `json:"candy_latency_ms,omitempty"`
	CandyError       string    `json:"candy_error,omitempty"`
	PelicanOK        bool      `json:"pelican_ok"`
	PelicanReview    string    `json:"pelican_review"`
	PelicanLatencyMS int64     `json:"pelican_latency_ms,omitempty"`
	PelicanError     string    `json:"pelican_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

// publicResult is one sanitized result pair: candy final answer, pelican HTML,
// latencies and the manual review verdicts.
type publicResult struct {
	ID               int64     `json:"id"`
	GroupID          int64     `json:"group_id"`
	GroupName        string    `json:"group_name"`
	Model            string    `json:"model"`
	Status           string    `json:"status"`
	CandyAnswer      string    `json:"candy_answer"`
	CandyCorrect     *bool     `json:"candy_correct"`
	CandyReview      string    `json:"candy_review"`
	CandyLatencyMS   int64     `json:"candy_latency_ms,omitempty"`
	CandyError       string    `json:"candy_error,omitempty"`
	PelicanHTML      string    `json:"pelican_html"`
	PelicanOK        bool      `json:"pelican_ok"`
	PelicanReview    string    `json:"pelican_review"`
	PelicanLatencyMS int64     `json:"pelican_latency_ms,omitempty"`
	PelicanError     string    `json:"pelican_error,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type publicSettings struct {
	Models          []string      `json:"models"`
	ReasoningEffort string        `json:"reasoning_effort"`
	QuietWindows    []QuietWindow `json:"quiet_windows"`
}

type publicOverview struct {
	Settings publicSettings `json:"settings"`
	Groups   []publicGroup  `json:"groups"`
}

type publicLimiter struct {
	mu   sync.Mutex
	hits map[string]*publicWindow
}

type publicWindow struct {
	start time.Time
	count int
}

func publicReview(verdict string) string {
	switch verdict {
	case "approved", "rejected":
		return verdict
	default:
		return "pending"
	}
}

func (s *Server) handlePublicOverview(w http.ResponseWriter, r *http.Request) {
	if !s.beginPublic(w, r) {
		return
	}
	overview, err := s.loadPublicOverview(r.Context())
	if err != nil {
		writePublicError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writePublicOK(w, overview)
}

func (s *Server) handlePublicResult(w http.ResponseWriter, r *http.Request) {
	if !s.beginPublic(w, r) {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		writePublicError(w, http.StatusBadRequest, "invalid id")
		return
	}
	detail, err := s.loadPublicResult(r.Context(), id)
	if err != nil {
		if err == errNotPublished {
			writePublicError(w, http.StatusNotFound, "not found")
			return
		}
		writePublicError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writePublicOK(w, detail)
}

func (s *Server) loadPublicOverview(ctx context.Context) (*publicOverview, error) {
	selection, err := s.Store.GetSelection(ctx)
	if err != nil {
		return nil, err
	}
	out := &publicOverview{
		Settings: publicSettings{QuietWindows: []QuietWindow{}},
		Groups:   []publicGroup{},
	}
	if models, err := s.Store.ListDetectionModels(ctx); err == nil {
		out.Settings.Models = make([]string, 0, len(models))
		for _, model := range models {
			out.Settings.Models = append(out.Settings.Models, model.Name)
		}
	}
	if prompts, err := s.Store.GetDetectionPrompts(ctx); err == nil && prompts != nil {
		out.Settings.ReasoningEffort = prompts.ReasoningEffort
		out.Settings.QuietWindows = append(out.Settings.QuietWindows, prompts.QuietWindows...)
	}
	for _, group := range selection.Groups {
		if group.IntervalMinutes <= 0 || !group.IsPublicVisible() {
			continue
		}
		entry := publicGroup{
			ID:              group.ID,
			Name:            group.Name,
			IntervalMinutes: group.IntervalMinutes,
			Results:         []publicResultMeta{},
		}
		results, _, err := s.Store.ListTestResults(ctx, publicOverviewResults, 0, true, group.ID)
		if err == nil {
			for _, result := range results {
				if result.GroupID != group.ID {
					continue
				}
				entry.Results = append(entry.Results, publicMeta(result))
			}
		}
		out.Groups = append(out.Groups, entry)
	}
	return out, nil
}

func (s *Server) loadPublicResult(ctx context.Context, id int64) (*publicResult, error) {
	result, err := s.Store.GetTestResultByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil, errNotPublished
		}
		return nil, err
	}
	selection, err := s.Store.GetSelection(ctx)
	if err != nil {
		return nil, err
	}
	published := false
	for _, group := range selection.Groups {
		if group.ID == result.GroupID && group.IntervalMinutes > 0 && group.IsPublicVisible() {
			published = true
			break
		}
	}
	if !published {
		return nil, errNotPublished
	}
	html := result.PelicanHTML
	detail := &publicResult{
		ID:               result.ID,
		GroupID:          result.GroupID,
		GroupName:        result.GroupName,
		Model:            result.Model,
		Status:           result.Status,
		CandyAnswer:      extractFinalAnswer(result.CandyAnswer),
		CandyCorrect:     result.CandyCorrect,
		CandyReview:      publicReview(result.CandyReview),
		CandyLatencyMS:   result.CandyLatencyMS,
		CandyError:       result.CandyError,
		PelicanHTML:      html,
		PelicanOK:        result.PelicanError == "" && html != "",
		PelicanReview:    publicReview(result.PelicanReview),
		PelicanLatencyMS: result.PelicanLatencyMS,
		PelicanError:     result.PelicanError,
		CreatedAt:        result.CreatedAt,
	}
	return detail, nil
}

func publicMeta(result TestResult) publicResultMeta {
	return publicResultMeta{
		ID:               result.ID,
		Model:            result.Model,
		Status:           result.Status,
		CandyCorrect:     result.CandyCorrect,
		CandyReview:      publicReview(result.CandyReview),
		CandyLatencyMS:   result.CandyLatencyMS,
		CandyError:       result.CandyError,
		PelicanOK:        result.PelicanError == "" && result.PelicanHTMLLen > 0,
		PelicanReview:    publicReview(result.PelicanReview),
		PelicanLatencyMS: result.PelicanLatencyMS,
		PelicanError:     result.PelicanError,
		CreatedAt:        result.CreatedAt,
	}
}

var errNotPublished = errString("not found")

type errString string

func (e errString) Error() string { return string(e) }

func (s *Server) beginPublic(w http.ResponseWriter, r *http.Request) bool {
	setPublicCORS(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return false
	}
	if r.Method != http.MethodGet {
		writePublicError(w, http.StatusMethodNotAllowed, "method not allowed")
		return false
	}
	if !s.publicLimit.allow(publicClientIP(r), time.Now()) {
		w.Header().Set("Retry-After", "60")
		writePublicError(w, http.StatusTooManyRequests, "too many requests")
		return false
	}
	return true
}

func setPublicCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "*")
	w.Header().Set("Access-Control-Max-Age", "86400")
}

func writePublicOK(w http.ResponseWriter, data any) {
	writePublicEnvelope(w, http.StatusOK, publicEnvelope{Code: 0, Message: "success", Data: data})
}

func writePublicError(w http.ResponseWriter, status int, message string) {
	writePublicEnvelope(w, status, publicEnvelope{Code: status, Message: message})
}

func writePublicEnvelope(w http.ResponseWriter, status int, payload publicEnvelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func (l *publicLimiter) allow(ip string, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.hits == nil {
		l.hits = map[string]*publicWindow{}
	}
	window := l.hits[ip]
	if window == nil || now.Sub(window.start) >= time.Minute {
		l.hits[ip] = &publicWindow{start: now, count: 1}
		if len(l.hits) > 4096 {
			for key, item := range l.hits {
				if now.Sub(item.start) >= time.Minute {
					delete(l.hits, key)
				}
			}
		}
		return true
	}
	if window.count >= publicRatePerMinute {
		return false
	}
	window.count++
	return true
}

// publicClientIP uses the first X-Forwarded-For hop only when the direct
// peer is loopback (the widget nginx on this machine). Other peers keep
// their own address so a spoofed header cannot pick a fresh rate-limit bucket.
func publicClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip != nil && ip.IsLoopback() {
		if fwd := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-For"), ",")[0]); fwd != "" {
			if net.ParseIP(fwd) != nil {
				return fwd
			}
		}
	}
	return host
}
