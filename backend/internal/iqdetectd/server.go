package iqdetectd

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Server is the HTTP layer of iqdetectd.
type Server struct {
	Store       *Store
	Runner      *Runner
	TestRunner  *TestRunner
	StartedAt   time.Time
	Version     string
	publicLimit publicLimiter
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/probes", s.handleListProbes)
	mux.HandleFunc("GET /v1/models", s.handleListModels)
	mux.HandleFunc("POST /v1/models", s.handleCreateModel)
	mux.HandleFunc("PUT /v1/models/{id}", s.handleUpdateModel)
	mux.HandleFunc("DELETE /v1/models/{id}", s.handleDeleteModel)
	mux.HandleFunc("POST /v1/runs", s.handleTriggerRun)
	mux.HandleFunc("GET /v1/runs", s.handleListRuns)
	mux.HandleFunc("GET /v1/runs/{id}", s.handleGetRun)
	mux.HandleFunc("GET /v1/scores", s.handleListScores)
	mux.HandleFunc("GET /v1/account-credentials", s.handleListAccountCredentials)
	mux.HandleFunc("PUT /v1/account-credentials", s.handleUpsertAccountCredential)
	mux.HandleFunc("GET /v1/selection", s.handleGetSelection)
	mux.HandleFunc("PUT /v1/selection", s.handleSaveSelection)
	mux.HandleFunc("GET /v1/detection-models", s.handleListDetectionModels)
	mux.HandleFunc("PUT /v1/detection-models", s.handleReplaceDetectionModels)
	mux.HandleFunc("POST /v1/test-runs", s.handleTriggerTestRun)
	mux.HandleFunc("POST /v1/test-runs/stop", s.handleStopTestRuns)
	mux.HandleFunc("GET /v1/test-results", s.handleListTestResults)
	mux.HandleFunc("GET /v1/test-results/{id}", s.handleGetTestResult)
	mux.HandleFunc("GET /v1/test-results/{id}/pelican", s.handleGetTestResultPelican)
	mux.HandleFunc("PUT /v1/test-results/{id}/review", s.handleUpdateTestResultReview)
	mux.HandleFunc("/iq-detection/overview", s.handlePublicOverview)
	mux.HandleFunc("/iq-detection/results/{id}", s.handlePublicResult)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"version":        s.Version,
		"uptime_seconds": int64(time.Since(s.StartedAt).Seconds()),
	})
}

func (s *Server) handleListProbes(w http.ResponseWriter, r *http.Request) {
	infos := ProbeInfos()
	writeJSON(w, http.StatusOK, map[string]any{"items": infos, "total": len(infos)})
}

func (s *Server) handleListModels(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListModels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleCreateModel(w http.ResponseWriter, r *http.Request) {
	var input CreateModelInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	model, err := s.Store.CreateModel(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, model)
}

func (s *Server) handleUpdateModel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var input CreateModelInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	model, err := s.Store.UpdateModel(r.Context(), id, input)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, model)
}

func (s *Server) handleDeleteModel(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.DeleteModel(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func (s *Server) handleTriggerRun(w http.ResponseWriter, r *http.Request) {
	var input TriggerRunInput
	if r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json")
			return
		}
	}
	run, err := s.Runner.TriggerRun(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, run)
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	runs, err := s.Store.ListRuns(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": runs, "total": len(runs)})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	run, err := s.Store.GetRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	results, err := s.Store.ListResultsByRun(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, RunDetail{Run: *run, Results: results})
}

func (s *Server) handleListScores(w http.ResponseWriter, r *http.Request) {
	history, _ := strconv.Atoi(r.URL.Query().Get("history"))
	scores, err := s.Store.ListScores(r.Context(), history)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": scores, "total": len(scores)})
}

func (s *Server) handleListAccountCredentials(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListAccountCredentials(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleUpsertAccountCredential(w http.ResponseWriter, r *http.Request) {
	var input UpsertAccountCredentialInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	credential, err := s.Store.UpsertAccountCredential(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, credential)
}

func (s *Server) handleGetSelection(w http.ResponseWriter, r *http.Request) {
	selection, err := s.Store.GetSelection(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, selection)
}

func (s *Server) handleSaveSelection(w http.ResponseWriter, r *http.Request) {
	var input DetectionSelection
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	selection, err := s.Store.SaveSelection(r.Context(), input.Groups)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	// 选择变化后重新尝试归属历史无分组的结果（失败不影响保存）；
	// 调度器每分钟从数据库重读选择，无需在此重排定时器
	_, _ = s.Store.BackfillTestResultGroups(r.Context())
	writeJSON(w, http.StatusOK, selection)
}

func (s *Server) handleListDetectionModels(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListDetectionModels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	prompts, err := s.Store.GetDetectionPrompts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, DetectionModelSettings{
		Items:           items,
		Total:           len(items),
		PelicanPrompt:   prompts.PelicanPrompt,
		CandyPrompt:     prompts.CandyPrompt,
		CandyAnswer:     prompts.CandyAnswer,
		ReasoningEffort: prompts.ReasoningEffort,
		Temperature:     prompts.Temperature,
		QuietWindows:    prompts.QuietWindows,
	})
}

func (s *Server) handleReplaceDetectionModels(w http.ResponseWriter, r *http.Request) {
	var input PutDetectionModelsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	quietWindows, quietErr := normalizeQuietWindows(input.QuietWindows)
	if quietErr != "" {
		writeError(w, http.StatusBadRequest, quietErr)
		return
	}
	items, err := s.Store.ReplaceDetectionModels(r.Context(), input.Names)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.Store.UpsertDetectionPrompts(r.Context(), DetectionPrompts{
		PelicanPrompt:   input.PelicanPrompt,
		CandyPrompt:     input.CandyPrompt,
		CandyAnswer:     input.CandyAnswer,
		ReasoningEffort: input.ReasoningEffort,
		Temperature:     input.Temperature,
		QuietWindows:    quietWindows,
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, DetectionModelSettings{
		Items:           items,
		Total:           len(items),
		PelicanPrompt:   input.PelicanPrompt,
		CandyPrompt:     input.CandyPrompt,
		CandyAnswer:     input.CandyAnswer,
		ReasoningEffort: input.ReasoningEffort,
		Temperature:     input.Temperature,
		QuietWindows:    quietWindows,
	})
}

func (s *Server) handleTriggerTestRun(w http.ResponseWriter, r *http.Request) {
	if s.TestRunner == nil {
		writeError(w, http.StatusInternalServerError, "test runner unavailable")
		return
	}
	var input TriggerTestRunInput
	if r.Body != nil {
		_ = json.NewDecoder(io.LimitReader(r.Body, 4<<20)).Decode(&input) // 空 body 合法
	}
	plan, err := s.TestRunner.TriggerTestRun(r.Context(), input.Credentials, input.GroupID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, plan)
}

// handleStopTestRuns cancels every in-flight group run (scheduled or manual)
// and reports how many were stopped.
func (s *Server) handleStopTestRuns(w http.ResponseWriter, r *http.Request) {
	if s.TestRunner == nil {
		writeError(w, http.StatusInternalServerError, "test runner unavailable")
		return
	}
	stopped := s.TestRunner.StopTestRuns()
	writeJSON(w, http.StatusOK, map[string]any{"stopped": stopped})
}

// handleListTestResults serves the newest result pairs without the pelican
// HTML body; clients fetch the body per pair via /v1/test-results/{id}/pelican.
func (s *Server) handleListTestResults(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 100 {
		limit = 100
	}
	offset := 0
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}
	// group_id 显式传 0 表示只看未分组的历史结果；不传则返回全部
	groupID := int64(0)
	hasGroup := r.URL.Query().Has("group_id")
	if hasGroup {
		if n, err := strconv.ParseInt(r.URL.Query().Get("group_id"), 10, 64); err == nil && n >= 0 {
			groupID = n
		}
	}
	items, total, err := s.Store.ListTestResults(r.Context(), int64(limit), int64(offset), hasGroup, groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (s *Server) handleGetTestResult(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	result, err := s.Store.GetTestResultByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGetTestResultPelican(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	html, err := s.Store.GetTestResultPelicanHTML(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func (s *Server) handleUpdateTestResultReview(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var input TestResultReviewInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if err := s.Store.UpdateTestResultReview(r.Context(), id, input.Probe, input.Verdict); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": strings.TrimSpace(message)})
}
