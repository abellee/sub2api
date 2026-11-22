package admin

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/latencytrace"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (h *OpsHandler) latencyCollector(c *gin.Context) *latencytrace.Collector {
	collector := h.opsService.LatencyCollector()
	if collector == nil {
		response.Error(c, http.StatusServiceUnavailable, "Latency collection unavailable")
	}
	return collector
}
func latencyError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, latencytrace.ErrInvalidScope):
		response.BadRequest(c, err.Error())
	case errors.Is(err, latencytrace.ErrActiveLimit):
		response.Error(c, http.StatusConflict, err.Error())
	case errors.Is(err, latencytrace.ErrNotFound):
		response.Error(c, http.StatusNotFound, err.Error())
	default:
		response.Error(c, http.StatusInternalServerError, "Unable to access latency collection")
	}
}
func latencyID(c *gin.Context, id string, optional bool) (string, bool) {
	id = strings.TrimSpace(id)
	if id == "" && optional {
		return "", true
	}
	parsed, err := uuid.Parse(id)
	if err != nil {
		response.BadRequest(c, "Invalid collection ID")
		return "", false
	}
	return parsed.String(), true
}
func latencyCursor(c *gin.Context) (int64, bool) {
	raw := c.DefaultQuery("before", "0")
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 0 {
		response.BadRequest(c, "Invalid cursor")
		return 0, false
	}
	return n, true
}
func (h *OpsHandler) CreateLatencyCollection(c *gin.Context) {
	collector := h.latencyCollector(c)
	if collector == nil {
		return
	}
	var scope latencytrace.Scope
	if err := c.ShouldBindJSON(&scope); err != nil {
		response.BadRequest(c, "Invalid collection scope")
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, "Authentication required")
		return
	}
	session, err := collector.Create(c.Request.Context(), scope, subject.UserID)
	if err != nil {
		latencyError(c, err)
		return
	}
	response.Success(c, session)
}
func (h *OpsHandler) StopLatencyCollection(c *gin.Context) {
	collector := h.latencyCollector(c)
	if collector == nil {
		return
	}
	id, ok := latencyID(c, c.Param("id"), false)
	if !ok {
		return
	}
	if err := collector.Stop(c.Request.Context(), id); err != nil {
		latencyError(c, err)
		return
	}
	response.Success(c, gin.H{"stopped": true})
}
func (h *OpsHandler) ListLatencyCollections(c *gin.Context) {
	collector := h.latencyCollector(c)
	if collector == nil {
		return
	}
	id, ok := latencyID(c, c.Query("collection_id"), true)
	if !ok {
		return
	}
	before, ok := latencyCursor(c)
	if !ok {
		return
	}
	items, err := collector.Sessions(c.Request.Context(), id, before)
	if err != nil {
		latencyError(c, err)
		return
	}
	var next int64
	if len(items) == 50 {
		next = items[len(items)-1].Sequence
	}
	response.Success(c, gin.H{"items": items, "next_before": next, "health": collector.Health()})
}
func (h *OpsHandler) ListLatencyRecords(c *gin.Context) {
	collector := h.latencyCollector(c)
	if collector == nil {
		return
	}
	id, ok := latencyID(c, c.Param("id"), false)
	if !ok {
		return
	}
	before, ok := latencyCursor(c)
	if !ok {
		return
	}
	requestID := strings.TrimSpace(c.Query("request_id"))
	if len(requestID) > 256 {
		response.BadRequest(c, "Invalid request ID")
		return
	}
	items, err := collector.Records(c.Request.Context(), id, requestID, before)
	if err != nil {
		latencyError(c, err)
		return
	}
	var next int64
	if len(items) == 50 {
		next = items[len(items)-1].ID
	}
	response.Success(c, gin.H{"items": items, "next_before": next})
}
