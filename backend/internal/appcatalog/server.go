package appcatalog

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Server struct {
	Store     *Store
	Fetcher   *Fetcher
	StartedAt time.Time
	Version   string
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.handleHealth)
	mux.HandleFunc("GET /v1/apps", s.handleListApps)
	mux.HandleFunc("POST /v1/apps", s.handleCreateApp)
	mux.HandleFunc("POST /v1/fetch", s.handleFetchPreview)
	mux.HandleFunc("GET /v1/apps/{id}", s.handleGetApp)
	mux.HandleFunc("PUT /v1/apps/{id}", s.handleUpdateApp)
	mux.HandleFunc("DELETE /v1/apps/{id}", s.handleDeleteApp)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"version":        s.Version,
		"uptime_seconds": int64(time.Since(s.StartedAt).Seconds()),
	})
}

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	items, err := s.Store.ListApps(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) decodeAppInput(r *http.Request) (CreateAppInput, error) {
	var input CreateAppInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		return input, fmt.Errorf("invalid json")
	}
	official, github, err := NormalizeSourceURLs(input.OfficialURL, input.GitHubURL)
	if err != nil {
		return input, err
	}
	input.OfficialURL = official
	input.GitHubURL = github
	input.Screenshots = uniqueNonEmpty(input.Screenshots)
	return input, nil
}

func (s *Server) handleCreateApp(w http.ResponseWriter, r *http.Request) {
	input, err := s.decodeAppInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app, err := s.Store.CreateApp(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

func (s *Server) handleFetchPreview(w http.ResponseWriter, r *http.Request) {
	input, err := s.decodeAppInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.Fetcher == nil {
		writeError(w, http.StatusInternalServerError, "fetcher unavailable")
		return
	}
	result, err := s.Fetcher.FetchApp(r.Context(), input.OfficialURL, input.GitHubURL)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if result.Screenshots == nil {
		result.Screenshots = []string{}
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	input, err := s.decodeAppInput(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	app, err := s.Store.UpdateApp(r.Context(), id, input)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (s *Server) handleGetApp(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	app, err := s.Store.GetApp(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, app)
}

func (s *Server) handleDeleteApp(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.Store.DeleteApp(r.Context(), id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"error": strings.TrimSpace(message)})
}
