package handler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

func mediaTaskFailed(status string) bool {
	return status == "failed" || status == "recovery_required" || status == "interrupted"
}

// Provider details feed rule matching and private Ops diagnostics.
func (h *MediaHubRelayHandler) taskFailureContext(c *gin.Context, ctx context.Context, in mediahub.ForwardInput, id string, state []byte) (int, []byte) {
	// Private diagnostics are an immediate lookup, even after a synchronous wait.
	in.Wait = false
	in.ClientQuery = ""
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	taskStatus := gjson.GetBytes(state, "status").String()
	upstreamStatus := int(gjson.GetBytes(state, "response_status").Int())
	matchBody := []byte(gjson.GetBytes(state, "error_message").String())
	var diagnosticBody []byte
	if upstreamStatus == 0 {
		_, _ = fmt.Sscanf(string(matchBody), "upstream HTTP %d", &upstreamStatus)
	}
	// Older media services lack this internal format; their saved summary is still safe to match.
	if details, err := h.standardFetch(ctx, in, id, "error_context"); err == nil && details.StatusCode == http.StatusOK {
		if header := details.Header.Get("X-Media-Error-Status"); header != "" {
			code, _ := strconv.Atoi(header)
			if code >= 100 && code <= 599 {
				upstreamStatus = code
			}
			diagnosticBody = details.Body
			if len(details.Body) != 0 {
				matchBody = details.Body
			} else if len(matchBody) == 0 {
				matchBody = []byte(fmt.Sprintf("upstream HTTP %d (empty response body)", code))
			}
		} else {
			if code := int(gjson.GetBytes(details.Body, "status_code").Int()); code >= 400 && code <= 599 {
				upstreamStatus = code
			}
			if raw := gjson.GetBytes(details.Body, "body").String(); raw != "" {
				matchBody = []byte(raw)
				diagnosticBody = matchBody
				// Older async tasks saved the acceptance response instead of the failure.
				if mediaTaskErrorMessage(matchBody) == "" {
					if summary := strings.TrimSpace(gjson.GetBytes(details.Body, "message").String()); summary != "" {
						matchBody = []byte(summary)
					}
				}
			} else if summary := gjson.GetBytes(details.Body, "message").String(); summary != "" {
				matchBody = []byte(summary)
			}
		}
	} else if taskStatus == "failed" {
		if result, err := h.standardFetch(ctx, in, id, "openai_url"); err == nil && result.StatusCode >= 400 && result.StatusCode <= 599 {
			upstreamStatus = result.StatusCode
			matchBody = []byte(fmt.Sprintf("upstream HTTP %d: %s", upstreamStatus, string(matchBody)))
		}
	}
	if upstreamStatus > 0 || len(matchBody) > 0 {
		if len(diagnosticBody) == 0 {
			diagnosticBody = matchBody
		}
		recordMediaDiagnostic(c, upstreamStatus, matchBody, diagnosticBody)
	}
	return upstreamStatus, matchBody
}

func recordMediaDiagnostic(c *gin.Context, upstreamStatus int, matchBody, diagnosticBody []byte) {
	detail, _ := service.SanitizeOpsErrorBodyForQueue(string(diagnosticBody))
	message, _ := service.SanitizeOpsErrorBodyForQueue(string(matchBody))
	service.SetOpsUpstreamError(c, upstreamStatus, parseOpsErrorResponse([]byte(message)).Message, detail)
}

func (h *MediaHubRelayHandler) writeTaskFailure(c *gin.Context, ctx context.Context, in mediahub.ForwardInput, id string, state []byte) {
	taskStatus := gjson.GetBytes(state, "status").String()
	upstreamStatus, matchBody := h.taskFailureContext(c, ctx, in, id, state)
	status, errorType, message := http.StatusBadGateway, "generation_failed", "Generation failed. Please contact support with the task ID."
	if taskStatus == "recovery_required" || taskStatus == "interrupted" {
		errorType, message = "generation_unavailable", "The media task needs verification before it can be retried."
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
	original := mediaTaskErrorMessage(matchBody)
	changed := false
	switch upstreamStatus {
	case 400, 413, 415, 422:
		if strings.TrimSpace(original) != "" {
			status = upstreamStatus
			message = service.SanitizeUpstreamErrorMessage(original)
			changed = true
		}
	}
	platform := service.PlatformOpenAI
	if key, ok := middleware.GetAPIKeyFromContext(c); ok && key != nil && key.Group != nil && key.Group.Platform != "" {
		platform = key.Group.Platform
	}
	if h.oai != nil && h.oai.errorPassthroughService != nil {
		matchStatus := upstreamStatus
		if matchStatus == 0 {
			matchStatus = status
		} // No HTTP response: match the gateway failure without attributing its code to the provider.
		if rule := h.oai.errorPassthroughService.MatchRule(platform, matchStatus, []byte(original)); rule != nil {
			service.MarkUpstreamClientErrorHandled(c)
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

func mediaTaskErrorMessage(body []byte) string {
	if !gjson.ValidBytes(body) {
		return string(body) // Older media services return the saved message as plain text.
	}
	for _, path := range []string{"error.message", "message", "data.error.message", "error_description", "error.code"} {
		if message := gjson.GetBytes(body, path); message.Type == gjson.String && strings.TrimSpace(message.Str) != "" {
			return message.Str
		}
	}
	return ""
}

// Transport errors may carry a private URL. Keep the cause for Ops and match rules on it,
// while using a stage-specific message without infrastructure details in the public response.
func (h *MediaHubRelayHandler) transportFailure(c *gin.Context, stage, id string, err error) {
	if mediaClientGone(c, stage, id) {
		return
	}
	status, msg := h.transportErrorRule(c, stage, err)
	mediaStandardError(c, status, msg, id)
}

func mediaClientGone(c *gin.Context, stage, id string) bool {
	if !failoverClientGone(c) {
		return false
	}
	logger.L().Info("mediahub: client disconnected", zap.String("stage", stage), zap.String("task_id", id), zap.Error(c.Request.Context().Err()))
	return true
}
func (h *MediaHubRelayHandler) transportErrorRule(c *gin.Context, stage string, err error) (int, string) {
	_ = c.Error(err)
	cause := err
	var ue *url.Error
	if errors.As(err, &ue) {
		cause = ue.Err
	}
	status := 502
	reason := "connection to media service failed"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		status = 504
		reason = "media service timed out"
	case errors.Is(err, context.Canceled):
		reason = "request was canceled"
	case errors.Is(err, io.EOF), errors.Is(err, io.ErrUnexpectedEOF):
		reason = "media service closed the connection before the response was complete"
	case errors.Is(err, syscall.ECONNRESET):
		reason = "media service connection was reset"
	case errors.Is(err, syscall.ECONNREFUSED):
		reason = "media service refused the connection"
	}
	body := []byte(stage + ": " + cause.Error())
	recordMediaDiagnostic(c, 0, body, body)
	status, msg, _ := h.applyTaskFailureRule(c, status, body, status, stage+": "+reason)
	return status, msg
}

func (h *MediaHubRelayHandler) directMediaError(c *gin.Context, status int, body []byte, id string) {
	recordMediaDiagnostic(c, status, body, body)
	msg := http.StatusText(status)
	// Query-policy responses are generated by mediad and include polling instructions.
	policy := (status == 410 || status == 429) && gjson.GetBytes(body, "task_id").String() == id && id != ""
	if policy {
		msg = mediaTaskErrorMessage(body)
	}
	code, msg, changed := h.applyTaskFailureRule(c, status, body, status, msg)
	if changed || !policy {
		mediaStandardError(c, code, msg, id)
		return
	}
	if id != "" {
		c.Header("X-Task-Id", id)
	}
	c.Data(status, "application/json", body)
}

// Staging errors also use 410, but unlike policy rejections they have a task diagnostic.
func (h *MediaHubRelayHandler) queryMediaError(c *gin.Context, ctx context.Context, in mediahub.ForwardInput, status int, body []byte, id string) {
	if status == 410 && gjson.GetBytes(body, "task_status").String() == "failed" {
		upstream, raw := h.taskFailureContext(c, ctx, in, id, nil)
		if len(raw) > 0 {
			code, msg, changed := h.applyTaskFailureRule(c, upstream, raw, status, mediaTaskErrorMessage(body))
			if changed {
				mediaStandardError(c, code, msg, id)
				return
			}
			c.Data(status, "application/json", body)
			return
		}
	}
	h.directMediaError(c, status, body, id)
}
func publicMediaFailureMessage(public, private []byte, code int) string {
	msg := gjson.GetBytes(public, "error.message").String()
	if msg != "" && msg != mediaTaskErrorMessage(private) {
		return msg
	}
	if code == 429 {
		return "Generation service is busy. Please try again later."
	}
	return "Generation failed. Please contact support with the task ID."
}
func (h *MediaHubRelayHandler) finishMediaFailure(c *gin.Context, status int, body []byte, upstream int, raw []byte, id string) {
	if len(raw) == 0 {
		upstream = status
		raw = body
		recordMediaDiagnostic(c, 0, raw, raw)
	}
	code, msg, _ := h.applyTaskFailureRule(c, upstream, raw, status, publicMediaFailureMessage(body, raw, upstream))
	mediaStandardError(c, code, msg, id)
}
