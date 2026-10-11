package iqdetectd

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Server is the HTTP layer of iqdetectd.
type Server struct {
	Store     *Store
	Runner    *Runner
	StartedAt time.Time
	Version   string
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
	writeJSON(w, http.StatusOK, selection)
}

func (s *Server) handleListDetectionModels(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListDetectionModels(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	pelican, candy, err := s.Store.GetDetectionPrompts(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, DetectionModelSettings{
		Items:         items,
		Total:         len(items),
		PelicanPrompt: pelican,
		CandyPrompt:   candy,
	})
}

func (s *Server) handleReplaceDetectionModels(w http.ResponseWriter, r *http.Request) {
	var input PutDetectionModelsInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}
	items, err := s.Store.ReplaceDetectionModels(r.Context(), input.Names)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.Store.UpsertDetectionPrompts(r.Context(), input.PelicanPrompt, input.CandyPrompt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, DetectionModelSettings{
		Items:         items,
		Total:         len(items),
		PelicanPrompt: input.PelicanPrompt,
		CandyPrompt:   input.CandyPrompt,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": strings.TrimSpace(message)})
}
