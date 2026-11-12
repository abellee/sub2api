package handler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func mediaTaskFailed(status string) bool {
	return status == "failed" || status == "recovery_required" || status == "interrupted"
}

// Provider details feed rule matching and private Ops diagnostics.
func (h *MediaHubRelayHandler) taskFailureContext(c *gin.Context, ctx context.Context, in mediahub.ForwardInput, id string, state []byte) (int, []byte) {
	// Private diagnostics are an immediate lookup, even after a synchronous wait.
	in.Wait = false
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	taskStatus := gjson.GetBytes(state, "status").String()
	upstreamStatus := int(gjson.GetBytes(state, "response_status").Int())
	matchBody := []byte(gjson.GetBytes(state, "error_message").String())
	if upstreamStatus == 0 {
		_, _ = fmt.Sscanf(string(matchBody), "upstream HTTP %d", &upstreamStatus)
	}
	// Older media services lack this internal format; their saved summary is still safe to match.
	if details, err := h.standardFetch(ctx, in, id, "error_context"); err == nil && details.StatusCode == http.StatusOK {
		if code := int(gjson.GetBytes(details.Body, "status_code").Int()); code >= 400 && code <= 599 {
			upstreamStatus = code
		}
		if raw := gjson.GetBytes(details.Body, "body").String(); raw != "" {
			matchBody = []byte(raw)
		} else if summary := gjson.GetBytes(details.Body, "message").String(); summary != "" {
			matchBody = []byte(summary)
		}
	} else if taskStatus == "failed" {
		if result, err := h.standardFetch(ctx, in, id, "openai_url"); err == nil && result.StatusCode >= 400 && result.StatusCode <= 599 {
			upstreamStatus = result.StatusCode
			matchBody = []byte(fmt.Sprintf("upstream HTTP %d: %s", upstreamStatus, string(matchBody)))
		}
	}
	if upstreamStatus > 0 || len(matchBody) > 0 {
		detail, _ := service.SanitizeOpsErrorBodyForQueue(string(matchBody))
		service.SetOpsUpstreamError(c, upstreamStatus, parseOpsErrorResponse([]byte(detail)).Message, detail)
	}
	return upstreamStatus, matchBody
}

func (h *MediaHubRelayHandler) writeTaskFailure(c *gin.Context, ctx context.Context, in mediahub.ForwardInput, id string, state []byte) {
	taskStatus := gjson.GetBytes(state, "status").String()
	upstreamStatus, matchBody := h.taskFailureContext(c, ctx, in, id, state)
	status, errorType, message := http.StatusBadGateway, "generation_failed", "Generation failed. Please contact support with the task ID."
	if taskStatus == "recovery_required" || taskStatus == "interrupted" {
		errorType, message = "generation_unavailable", "Generation result is unavailable. Please contact support with the task ID before retrying."
	} else {
		switch upstreamStatus {
		case 400, 413, 415, 422:
			status, message = upstreamStatus, "Generation failed. Please check the request and try again."
		case 429:
			status, errorType, message = 429, "generation_busy", "Generation service is busy. Please try again later."
		}
	}
	status, message, _ = h.applyTaskFailureRule(c, upstreamStatus, matchBody, status, message)
	c.Header("X-Task-Id", id)
	c.JSON(status, gin.H{"id": id, "task_id": id, "status": taskStatus, "http_status": status,
		"error": gin.H{"type": errorType, "message": message}})
}

func (h *MediaHubRelayHandler) applyTaskFailureRule(c *gin.Context, upstreamStatus int, matchBody []byte, status int, message string) (int, string, bool) {
	changed := false
	switch upstreamStatus {
	case 400, 413, 415, 422:
		original := service.ExtractUpstreamErrorMessage(matchBody)
		if original == "" && !gjson.ValidBytes(matchBody) {
			original = string(matchBody)
		}
		if strings.TrimSpace(original) != "" {
			message = service.SanitizeUpstreamErrorMessage(original)
			changed = true
		}
	}
	platform := service.PlatformOpenAI
	if key, ok := middleware.GetAPIKeyFromContext(c); ok && key != nil && key.Group != nil && key.Group.Platform != "" {
		platform = key.Group.Platform
	}
	if h.oai != nil && h.oai.errorPassthroughService != nil {
		if rule := h.oai.errorPassthroughService.MatchRule(platform, upstreamStatus, matchBody); rule != nil {
			if rule.PassthroughCode && upstreamStatus >= 400 && upstreamStatus <= 599 {
				status = upstreamStatus
			} else if rule.ResponseCode != nil && *rule.ResponseCode >= 400 && *rule.ResponseCode <= 599 {
				status = *rule.ResponseCode
			}
			if !rule.PassthroughBody && rule.CustomMessage != nil && strings.TrimSpace(*rule.CustomMessage) != "" {
				message = *rule.CustomMessage
			}
			if rule.SkipMonitoring {
				c.Set(service.OpsSkipPassthroughKey, true)
			}
			return status, message, true
		}
	}
	return status, message, changed
}
