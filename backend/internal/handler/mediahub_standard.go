package handler

import (
	"context"
	"encoding/json"
	"io"
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
	if key, _ := middleware.GetAPIKeyFromContext(c); key != nil && key.Group != nil && key.Group.Platform == "grok" {
		protocol = "grok_image"
	}
	if async || c.Query("async") == "true" {
		protocol += "_async"
	}
	c.Set(mediaProtocolKey, protocol)
	h.relay.Relay(c)
	return true
}

func (h *MediaHubController) TryStandardVideo(c *gin.Context) bool {
	if !h.standardEnabled(c) {
		return false
	}
	key, _ := middleware.GetAPIKeyFromContext(c)
	b, _ := h.bindings.Get(*key.GroupID)
	if b.VideoSchemaID == "" {
		return false
	}
	c.Set(mediaProtocolKey, "grok_video")
	h.relay.Relay(c)
	return true
}

// Old upstream task IDs keep their native handler during migration.
func (h *MediaHubController) TryStandardVideoTask(c *gin.Context, content bool) bool {
	id := c.Param("request_id")
	if h == nil || h.relay == nil || !strings.HasPrefix(id, "mtask_") {
		return false
	}
	key, ok := middleware.GetAPIKeyFromContext(c)
	subject, ownerOK := middleware.GetAuthSubjectFromContext(c)
	if !ok || key == nil || !ownerOK {
		c.Status(401)
		return true
	}
	gid := int64(0)
	if key.GroupID != nil {
		gid = *key.GroupID
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
	defer cancel()
	in := mediahub.ForwardInput{MediadBaseURL: h.relay.cfg.MediadBaseURL, RelaySecret: h.relay.cfg.RelaySecret, UserID: subject.UserID, KeyID: key.ID, GroupID: gid}
	result, err := h.relay.standardFetch(ctx, in, id, "grok_video")
	if err != nil {
		mediaStandardError(c, 502, "media task query unavailable")
		return true
	}
	if result.StatusCode == http.StatusNotFound && h.relay.deletedTaskResponse(c, subject.UserID, id) {
		return true
	}
	if content && result.StatusCode == 200 {
		if url := gjson.GetBytes(result.Body, "video.url").String(); url != "" {
			c.Header("Cache-Control", "private, no-store")
			c.Redirect(http.StatusTemporaryRedirect, url)
			return true
		}
	}
	c.Data(result.StatusCode, "application/json", result.Body)
	return true
}

func (h *MediaHubController) TryStandardGemini(c *gin.Context) bool {
	model, action, ok := strings.Cut(strings.TrimPrefix(c.Param("modelAction"), "/"), ":")
	if !ok || action != "generateContent" || !h.standardEnabled(c) {
		return false
	}
	protocol := "gemini"
	if c.Query("async") == "true" {
		protocol += "_async"
	}
	c.Set(mediaProtocolKey, protocol)
	if model == "" || strings.ContainsAny(model, "/\\?#") {
		mediaStandardError(c, 400, "invalid model")
		return true
	}
	c.Set(mediaModelKey, model)
	h.relay.Relay(c)
	return true
}

func mediaStandardError(c *gin.Context, status int, message string) {
	if strings.HasPrefix(c.GetString(mediaProtocolKey), "gemini") {
		code := "INTERNAL"
		switch status {
		case 400:
			code = "INVALID_ARGUMENT"
		case 504:
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
	if strings.HasSuffix(c.GetString(mediaProtocolKey), "_async") {
		c.Header("Location", "/v1/images/tasks/"+id)
		c.Header("Retry-After", "3")
		pollURL := "/v1/images/tasks/" + id
		c.JSON(202, gin.H{"id": id, "task_id": id, "object": "image.generation.task", "status": "processing", "poll_url": pollURL})
		return
	}
	if c.GetString(mediaProtocolKey) == "grok_video" {
		c.JSON(200, gin.H{"id": id, "request_id": id})
		return
	}
	format := "openai_url"
	if gjson.GetBytes(body, "response_format").String() == "b64_json" {
		format = "openai_b64_json"
	}
	if strings.HasPrefix(c.GetString(mediaProtocolKey), "gemini") {
		format = "gemini"
	}
	stream := c.GetString(mediaProtocolKey) == "openai" && gjson.GetBytes(body, "stream").Bool()
	if stream {
		format = "openai_sse"
		if strings.HasSuffix(c.Request.URL.Path, "/images/edits") {
			format = "openai_edit_sse"
		}
	}

	in := mediahub.ForwardInput{MediadBaseURL: h.cfg.MediadBaseURL, RelaySecret: h.cfg.RelaySecret, UserID: userID, KeyID: keyID, GroupID: groupID}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.cfg.ForwardTimeout)
	defer cancel()

	in.ResultFormat = format
	in.Wait = true
	response, err := mediahub.OpenTaskResult(ctx, h.client, in, id)
	if err != nil {
		mediaStandardError(c, 502, "media result unavailable; task continues in background")
		return
	}
	defer func() { _ = response.Body.Close() }()
	c.Header("X-Task-Id", id)
	for _, name := range []string{"Content-Type", "Cache-Control", "X-Accel-Buffering"} {
		if value := response.Header.Get(name); value != "" {
			c.Header(name, value)
		}
	}
	c.Status(response.StatusCode)
	if stream {
		c.Writer.Flush()
	}
	writer := io.Writer(c.Writer)
	if stream {
		writer = mediaFlushWriter{c}
	}
	if _, err := io.Copy(writer, response.Body); err != nil {
		_ = c.Error(err)
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
		if state.StatusCode == http.StatusNotFound && h.relay.deletedTaskResponse(c, subject.UserID, id) {
			return true
		}
		c.Data(state.StatusCode, "application/json", state.Body)
		return true
	}
	out := gin.H{"id": id, "task_id": id, "status": "processing"}
	status := gjson.GetBytes(state.Body, "status").String()
	if mediaTaskFailed(status) {
		h.relay.writeTaskFailure(c, ctx, in, id, state.Body)
		return true
	}
	if status == "succeeded" && gjson.GetBytes(state.Body, "files_acknowledged").Bool() {
		c.JSON(http.StatusGone, gin.H{"id": id, "task_id": id, "status": "deleted", "error": gin.H{"message": "media files have expired"}})
		return true
	}
	if status == "succeeded" && gjson.GetBytes(state.Body, "callback_state").String() == "confirmed" {
		result, err := h.relay.standardFetch(ctx, in, id, "openai_url")
		if err != nil {
			mediaStandardError(c, 502, "media result unavailable")
			return true
		}
		out["http_status"] = result.StatusCode
		out["status"] = "completed"
		out["result"] = json.RawMessage(result.Body)
		out["image_url"] = gjson.GetBytes(result.Body, "data.0.url").String()
	}
	c.JSON(200, out)
	return true
}

type mediaFlushWriter struct{ c *gin.Context }

func (w mediaFlushWriter) Write(p []byte) (int, error) {
	n, err := w.c.Writer.Write(p)
	w.c.Writer.Flush()
	return n, err
}
