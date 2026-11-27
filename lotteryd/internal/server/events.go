package server

import (
	"database/sql"
	"errors"
	"lotteryd/api/events"
	"lotteryd/internal/store"
	"net/http"
	"strconv"
)

func (s *Server) registerEvents(mux *http.ServeMux) {
	mux.Handle("GET /v1/admin/events/catalog", s.requireAdmin(s.handleEventCatalog))
	mux.Handle("GET /v1/admin/events/rules", s.requireAdmin(s.handleEventRules))
	mux.Handle("POST /v1/admin/events/rules", s.requireAdmin(s.handleSaveEventRule))
	mux.Handle("PUT /v1/admin/events/rules/{id}", s.requireAdmin(s.handleSaveEventRule))
	mux.Handle("DELETE /v1/admin/events/rules/{id}", s.requireAdmin(s.handleDisableEventRule))
	mux.Handle("GET /v1/admin/events/logs", s.requireAdmin(s.handleEventLogs))
	mux.Handle("GET /v1/admin/events/executions", s.requireAdmin(s.handleEventLogs))
	mux.Handle("GET /v1/admin/events/settings", s.requireAdmin(s.handleEventSettings))
	mux.Handle("PUT /v1/admin/events/settings", s.requireAdmin(s.handleEventSettings))
}
func (s *Server) handleEventCatalog(w http.ResponseWriter, r *http.Request, _ *Claims) {
	items := make([]events.Definition, 0)
	for _, item := range events.Catalog() {
		if item.Status != "internal" {
			items = append(items, item)
		}
	}
	ok(w, map[string]any{"items": items, "actions": []string{"grant_points", "grant_item", "item_effect"}})
}
func (s *Server) handleEventRules(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	rules, err := s.App.Store.ListEventRules(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, map[string]any{"items": rules})
}
func eventError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrInvalidEventRule) {
		fail(w, 400, 400, err.Error())
	} else if errors.Is(err, sql.ErrNoRows) {
		fail(w, 404, 404, "规则不存在")
	} else {
		internalError(w, err)
	}
}
func (s *Server) handleSaveEventRule(w http.ResponseWriter, r *http.Request, _ *Claims) {
	var rule store.EventRule
	if !decodeShopBody(w, r, &rule) {
		return
	}
	rule.ID = 0
	if r.Method == http.MethodPut {
		id, valid := shopPathID(w, r)
		if !valid {
			return
		}
		rule.ID = id
	}
	saved, err := s.App.Store.SaveEventRule(r.Context(), rule)
	if err != nil {
		eventError(w, err)
		return
	}
	ok(w, saved)
}
func (s *Server) handleDisableEventRule(w http.ResponseWriter, r *http.Request, _ *Claims) {
	id, valid := shopPathID(w, r)
	if !valid {
		return
	}
	if err := s.App.Store.DisableEventRule(r.Context(), id); err != nil {
		eventError(w, err)
		return
	}
	ok(w, map[string]bool{"disabled": true})
}
func (s *Server) handleEventLogs(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	data, err := s.App.Store.EventLogs(r.Context(), r.URL.Query().Get("event_type"), r.URL.Query().Get("status"), page, size, r.URL.Path == "/v1/admin/events/executions")
	if err != nil {
		internalError(w, err)
		return
	}
	ok(w, data)
}
func (s *Server) handleEventSettings(w http.ResponseWriter, r *http.Request, _ *Claims) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method == http.MethodPut {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if !decodeShopBody(w, r, &body) {
			return
		}
		if err := s.App.Store.SetState("gameplay_rewards_enabled", strconv.FormatBool(body.Enabled)); err != nil {
			internalError(w, err)
			return
		}
	}
	enabled, err := s.App.Store.GetState("gameplay_rewards_enabled")
	if err != nil {
		internalError(w, err)
		return
	}
	stats, err := s.App.Store.EventStats(r.Context())
	if err != nil {
		internalError(w, err)
		return
	}
	stats["enabled"] = enabled == "true"
	stats["settlement_disabled"] = s.App.Cfg.DisableSettlement
	stats["sender_mode"] = "best_effort"
	ok(w, stats)
}
