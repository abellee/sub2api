package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
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
	in.ClientQuery = "status"
	if content {
		in.ClientQuery = "content"
	}
	result, err := h.relay.standardFetch(ctx, in, id, "grok_video")
	if err != nil {
		h.relay.transportFailure(c, "querying media task", id, err)
		return true
	}
	copyMediaTaskQueryHeaders(c, result.Header)
	if result.StatusCode == http.StatusTooManyRequests || result.StatusCode == http.StatusGone {
		h.relay.queryMediaError(c, ctx, in, result.StatusCode, result.Body, id)
		return true
	}
	if result.StatusCode == http.StatusNotFound && h.relay.deletedTaskResponse(c, subject.UserID, id) {
		return true
	}
	if result.StatusCode >= http.StatusBadRequest {
		c.Header("X-Task-Id", id)
		upstreamStatus, matchBody := h.relay.taskFailureContext(c, ctx, in, id, nil)
		h.relay.finishMediaFailure(c, result.StatusCode, result.Body, upstreamStatus, matchBody, id)
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

func mediaStandardError(c *gin.Context, status int, message string, taskID ...string) {
	response := gin.H{}
	if strings.HasPrefix(c.GetString(mediaProtocolKey), "gemini") {
		code := "INTERNAL"
		switch status {
		case 400:
			code = "INVALID_ARGUMENT"
		case 504:
			code = "DEADLINE_EXCEEDED"
		}
		response["error"] = gin.H{"code": status, "message": message, "status": code}
	} else {
		response["error"] = gin.H{"type": "api_error", "message": message}
	}
	if len(taskID) > 0 && taskID[0] != "" {
		response["task_id"] = taskID[0]
		response["poll_url"] = "/v1/images/tasks/" + taskID[0]
		c.Header("X-Task-Id", taskID[0])
	}
	c.JSON(status, response)
}

func (h *MediaHubRelayHandler) standardFetch(ctx context.Context, in mediahub.ForwardInput, id, format string) (*mediahub.ForwardResponse, error) {
	in.ResultFormat = format
	return mediahub.ForwardTaskStatus(ctx, h.client, in, id)
}

func copyMediaTaskQueryHeaders(c *gin.Context, headers http.Header) {
	for _, name := range []string{"Retry-After", "X-Task-Query-Expires-At", "Cache-Control"} {
		if value := headers.Get(name); value != "" {
			c.Header(name, value)
		}
	}
}

func (h *MediaHubRelayHandler) finishStandard(c *gin.Context, accepted *mediahub.ForwardResponse, body []byte, userID, keyID, groupID int64) {
	if accepted.StatusCode != http.StatusAccepted {
		c.Data(accepted.StatusCode, "application/json", accepted.Body)
		return
	}
	id := gjson.GetBytes(accepted.Body, "task_id").String()
	if id == "" {
		const msg = "media service accepted the request without a task ID"
		recordMediaDiagnostic(c, 0, []byte(msg), accepted.Body)
		status, message, _ := h.applyTaskFailureRule(c, 0, []byte(msg), 502, msg)
		mediaStandardError(c, status, message)
		return
	}
	queryExpiresAt := gjson.GetBytes(accepted.Body, "query_expires_at").String()
	if strings.HasSuffix(c.GetString(mediaProtocolKey), "_async") {
		c.Header("Location", "/v1/images/tasks/"+id)
		c.Header("Retry-After", "3")
		pollURL := "/v1/images/tasks/" + id
		c.JSON(202, gin.H{"id": id, "task_id": id, "object": "image.generation.task", "status": "processing", "poll_url": pollURL, "poll_interval_seconds": 3, "query_expires_at": queryExpiresAt})
		return
	}
	if c.GetString(mediaProtocolKey) == "grok_video" {
		c.Header("Retry-After", "3")
		c.JSON(200, gin.H{"id": id, "request_id": id, "poll_interval_seconds": 3, "query_expires_at": queryExpiresAt})
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
		logger.L().Warn("mediahub: result wait failed", zap.String("task_id", id),
			zap.Int64("user_id", userID), zap.Int64("group_id", groupID), zap.Error(err))
		h.transportFailure(c, "waiting for image result", id, err)
		return
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode >= http.StatusBadRequest {
		upstreamStatus, matchBody := h.taskFailureContext(c, ctx, in, id, nil)
		raw, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
		if err != nil {
			h.transportFailure(c, "reading media error response", id, err)
			return
		}
		h.finishMediaFailure(c, response.StatusCode, raw, upstreamStatus, matchBody, id)
		return
	}
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
		if err := h.copyMediaEvents(c, ctx, in, id, writer, response.Body); err != nil {
			status, msg := h.transportErrorRule(c, "reading image event stream", err)
			_ = writeMediaStreamError(c, writer, id, status, msg, "")
		}
		return
	}
	if _, err := io.Copy(writer, response.Body); err != nil {
		status, msg := h.transportErrorRule(c, "reading image result", err)
		if !c.Writer.Written() {
			mediaStandardError(c, status, msg, id)
		} else {
			service.MarkOpsStreamFailure(c, "api_error", "", msg, status)
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
	in.ClientQuery = "status"
	state, err := h.relay.standardFetch(ctx, in, id, "state")
	if err != nil {
		h.relay.transportFailure(c, "querying media task", id, err)
		return true
	}
	copyMediaTaskQueryHeaders(c, state.Header)
	if state.StatusCode != 200 {
		if state.StatusCode == http.StatusNotFound && h.relay.deletedTaskResponse(c, subject.UserID, id) {
			return true
		}
		h.relay.queryMediaError(c, ctx, in, state.StatusCode, state.Body, id)
		return true
	}
	out := gin.H{"id": id, "task_id": id, "status": "processing", "poll_interval_seconds": 3, "query_expires_at": gjson.GetBytes(state.Body, "query_expires_at").String()}
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
		in.ClientQuery = ""
		result, err := h.relay.standardFetch(ctx, in, id, "openai_url")
		if err != nil {
			h.relay.transportFailure(c, "reading completed image result", id, err)
			return true
		}
		if result.StatusCode >= 400 {
			h.relay.directMediaError(c, result.StatusCode, result.Body, id)
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

// mediad emits heartbeat comments, then either completed images or one error event.
// Stream large image lines untouched. Only inspect the bounded error frame.
func (h *MediaHubRelayHandler) copyMediaEvents(c *gin.Context, ctx context.Context, in mediahub.ForwardInput, id string, w io.Writer, src io.Reader) error {
	r := bufio.NewReader(src)
	for {
		line, err := r.ReadSlice('\n')
		if err == io.EOF {
			return io.ErrUnexpectedEOF
		}
		if err != nil {
			return err
		}
		if string(bytes.TrimSpace(line)) == "event: error" {
			const limit = 1 << 20
			data, err := bufio.NewReader(io.LimitReader(r, limit+1)).ReadBytes('\n')
			if len(data) > limit {
				return fmt.Errorf("media error frame exceeds %d bytes", limit)
			}
			if err != nil {
				return err
			}
			body := bytes.TrimSpace(bytes.TrimPrefix(data, []byte("data:")))
			code, raw := h.taskFailureContext(c, ctx, in, id, nil)
			if len(raw) == 0 {
				raw = body
				code = 502
				recordMediaDiagnostic(c, 0, raw, raw)
			}
			msg := publicMediaFailureMessage(body, raw, code)
			status, msg, _ := h.applyTaskFailureRule(c, code, raw, 502, msg)
			return writeMediaStreamError(c, w, id, status, msg, gjson.GetBytes(body, "error.code").String())
		}
		if _, err := w.Write(line); err != nil {
			return err
		}
		if bytes.HasPrefix(line, []byte("event: ")) {
			_, err := io.Copy(w, r)
			return err
		}
	}
}
func writeMediaStreamError(c *gin.Context, w io.Writer, id string, status int, msg, code string) error {
	service.MarkOpsStreamFailure(c, "api_error", code, msg, status)
	detail := gin.H{"message": msg}
	if code != "" {
		detail["code"] = code
	}
	b, _ := json.Marshal(gin.H{"type": "error", "error": detail, "http_status": status, "task_id": id})
	_, err := fmt.Fprintf(w, "event: error\ndata: %s\n\n", b)
	return err
}
