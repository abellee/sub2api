package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const mediaProtocolKey = "mediahub.standard_protocol"
const mediaModelKey = "mediahub.standard_model"

func (h *MediaHubController) standardEnabled(c *gin.Context) bool {
	if h == nil || h.relay == nil {
		return false
	}
	key, ok := middleware.GetAPIKeyFromContext(c)
	if !ok || key == nil || key.GroupID == nil {
		return false
	}
	b, ok := h.bindings.Get(*key.GroupID)
	return ok && b.Enabled
}

func (h *MediaHubController) TryStandardImage(c *gin.Context, async bool) bool {
	if !h.standardEnabled(c) {
		return false
	}
	protocol := "openai"
	if async {
		protocol = "openai_async"
	}
	c.Set(mediaProtocolKey, protocol)
	h.relay.Relay(c)
	return true
}

func (h *MediaHubController) TryStandardGemini(c *gin.Context) bool {
	model, action, ok := strings.Cut(strings.TrimPrefix(c.Param("modelAction"), "/"), ":")
	if !ok || action != "generateContent" || !h.standardEnabled(c) {
		return false
	}
	c.Set(mediaProtocolKey, "gemini")
	if model == "" || strings.ContainsAny(model, "/\\?#") {
		mediaStandardError(c, 400, "invalid model")
		return true
	}
	c.Set(mediaModelKey, model)
	h.relay.Relay(c)
	return true
}

func mediaStandardError(c *gin.Context, status int, message string) {
	if c.GetString(mediaProtocolKey) == "gemini" {
		code := "INTERNAL"
		if status == 400 {
			code = "INVALID_ARGUMENT"
		} else if status == 504 {
			code = "DEADLINE_EXCEEDED"
		}
		c.JSON(status, gin.H{"error": gin.H{"code": status, "message": message, "status": code}})
	} else {
		c.JSON(status, gin.H{"error": gin.H{"type": "api_error", "message": message}})
	}
}

func (h *MediaHubRelayHandler) standardFetch(ctx context.Context, in mediahub.ForwardInput, id, format string) (*mediahub.ForwardResponse, error) {
	in.ResultFormat = format
	return mediahub.ForwardTaskStatus(ctx, h.client, in, id)
}

func (h *MediaHubRelayHandler) finishStandard(c *gin.Context, accepted *mediahub.ForwardResponse, body []byte, userID, keyID, groupID int64) {
	if accepted.StatusCode != http.StatusAccepted {
		c.Data(accepted.StatusCode, "application/json", accepted.Body)
		return
	}
	id := gjson.GetBytes(accepted.Body, "task_id").String()
	if id == "" {
		mediaStandardError(c, 502, "media service did not return a task ID")
		return
	}
	if c.GetString(mediaProtocolKey) == "openai_async" {
		c.Header("Location", "/v1/images/tasks/"+id)
		c.Header("Retry-After", "3")
		c.JSON(202, gin.H{"id": id, "task_id": id, "object": "image.generation.task", "status": "processing", "poll_url": "/v1/images/tasks/" + id})
		return
	}
	format := "openai_b64_json"
	if gjson.GetBytes(body, "response_format").String() == "url" {
		format = "openai_url"
	}
	if c.GetString(mediaProtocolKey) == "gemini" {
		format = "gemini"
	}
	in := mediahub.ForwardInput{MediadBaseURL: h.cfg.MediadBaseURL, RelaySecret: h.cfg.RelaySecret, UserID: userID, KeyID: keyID, GroupID: groupID}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.cfg.ForwardTimeout)
	defer cancel()
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()
	for {
		state, err := h.standardFetch(ctx, in, id, "state")
		if err != nil {
			mediaStandardError(c, 502, "media task query unavailable; task continues in background")
			return
		}
		if state.StatusCode != 200 {
			c.Data(state.StatusCode, "application/json", state.Body)
			return
		}
		status := gjson.GetBytes(state.Body, "status").String()
		if status == "failed" || (status == "succeeded" && gjson.GetBytes(state.Body, "callback_state").String() == "confirmed") {
			result, err := h.standardFetch(ctx, in, id, format)
			if err != nil {
				mediaStandardError(c, 502, "media result unavailable")
				return
			}
			c.Header("X-Task-Id", id)
			c.Data(result.StatusCode, "application/json", result.Body)
			return
		}
		select {
		case <-ctx.Done():
			mediaStandardError(c, 504, "generation is still running; task ID: "+id)
			return
		case <-ticker.C:
		}
	}
}

// Existing image tasks retain their original handler; media task ownership is checked by mediad.
func (h *MediaHubController) TryStandardImageTask(c *gin.Context) bool {
	id := c.Param("task_id")
	if h == nil || h.relay == nil || !strings.HasPrefix(id, "mtask_") {
		return false
	}
	key, ok := middleware.GetAPIKeyFromContext(c)
	subject, subjectOK := middleware.GetAuthSubjectFromContext(c)
	if !ok || key == nil || !subjectOK {
		c.Status(401)
		return true
	}
	gid := int64(0)
	if key.GroupID != nil {
		gid = *key.GroupID
	}
	in := mediahub.ForwardInput{MediadBaseURL: h.relay.cfg.MediadBaseURL, RelaySecret: h.relay.cfg.RelaySecret, UserID: subject.UserID, KeyID: key.ID, GroupID: gid}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	state, err := h.relay.standardFetch(ctx, in, id, "state")
	if err != nil {
		mediaStandardError(c, 502, "media task query unavailable")
		return true
	}
	if state.StatusCode != 200 {
		c.Data(state.StatusCode, "application/json", state.Body)
		return true
	}
	out := gin.H{"id": id, "task_id": id, "status": "processing"}
	status := gjson.GetBytes(state.Body, "status").String()
	if status == "failed" || (status == "succeeded" && gjson.GetBytes(state.Body, "callback_state").String() == "confirmed") {
		result, err := h.relay.standardFetch(ctx, in, id, "openai_url")
		if err != nil {
			mediaStandardError(c, 502, "media result unavailable")
			return true
		}
		out["http_status"] = result.StatusCode
		if status == "failed" {
			out["status"] = "failed"
			out["error"] = gin.H{"message": gjson.GetBytes(state.Body, "error_message").String()}
		} else {
			out["status"] = "completed"
			out["result"] = json.RawMessage(result.Body)
			out["image_url"] = gjson.GetBytes(result.Body, "data.0.url").String()
		}
	}
	c.JSON(200, out)
	return true
}
