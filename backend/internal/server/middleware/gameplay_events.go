package middleware

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"lotteryd/api/events"
)

// GameplayEvents observes completed requests only. It neither reads nor wraps bodies/writers.
func GameplayEvents() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !events.Enabled() {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()
		if !events.Enabled() {
			return
		}
		defer func() { _ = recover() }()
		path := c.FullPath()
		if path == "" {
			return
		}
		status := c.Writer.Status()
		method := c.Request.Method
		uid := int64(0)
		if subject, ok := GetAuthSubjectFromContext(c); ok {
			uid = subject.UserID
		}
		if actor, ok := c.Get(auditCtxKeyActorID); ok {
			if id, ok := actor.(int64); ok && id > 0 {
				uid = id
			}
		}
		data := map[string]string{"endpoint": path, "method": method, "http_status": strconv.Itoa(status), "duration_ms": events.S(time.Since(start).Milliseconds()), "actor_id": events.S(uid), "action": strings.ToLower(method)}
		if key, ok := GetAPIKeyFromContext(c); ok && key != nil {
			uid = key.UserID
			data["api_key_id"] = events.S(key.ID)
			if key.GroupID != nil {
				data["group_id"] = events.S(*key.GroupID)
			}
		}
		for _, p := range c.Params {
			if p.Key == "id" || p.Key == "provider" || p.Key == "slug" || p.Key == "resource" {
				data["resource"] = p.Value
			}
		}
		if strings.HasPrefix(path, "/api/v1/admin/users/:id") {
			if target, err := strconv.ParseInt(c.Param("id"), 10, 64); err == nil && target > 0 {
				uid = target
			}
		}
		requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
		emit := func(kind string) { events.Publish(kind, uid, requestID, data) }
		if c.Request.Context().Err() == context.Canceled || c.GetBool("gameplay_client_disconnected") {
			if uid > 0 && c.GetBool("gameplay_request") {
				emit("request.cancelled")
				data["status"] = "cancelled"
				emit("request.completed")
			}
			return
		}
		if c.GetBool("gameplay_request") && uid > 0 {
			data["model"] = c.GetString("ops_model")
			outcome := "unknown"
			if _, observed := c.Get("gameplay_forward_failed"); observed {
				outcome = "success"
			}
			if status >= 400 || c.GetBool("gameplay_forward_failed") {
				outcome = "failed"
			}
			if status == 202 {
				outcome = "accepted"
			}
			if result := service.RequestAccountFailoverResult(c); result != "" {
				outcome = result
			}
			data["status"] = outcome
			emit("request.completed")
		}
		if status < 200 || status >= 300 {
			return
		}
		if method == "GET" && uid > 0 {
			switch {
			case strings.Contains(path, "/documentation/") && strings.HasSuffix(path, "/skill"):
				emit("content.skill_downloaded")
			case strings.Contains(path, "/documentation"):
				emit("content.documentation")
			case strings.Contains(path, "/pages/"):
				emit("content.page")
			case strings.Contains(path, "/model-plaza"):
				emit("content.model_plaza")
			case strings.Contains(path, "/apps"):
				emit("content.apps")
			case strings.HasPrefix(path, "/api/v1/user/") || strings.HasPrefix(path, "/api/v1/usage") || strings.HasPrefix(path, "/api/v1/keys") || strings.HasPrefix(path, "/api/v1/subscriptions") || strings.HasPrefix(path, "/api/v1/channel-monitor") || strings.HasPrefix(path, "/api/v1/groups") || strings.HasPrefix(path, "/api/v1/channels"):
				emit("content.user_view")
			}
			return
		}
		if strings.HasPrefix(path, "/api/v1/admin/") && method != "GET" {
			if path == "/api/v1/admin/users/batch-concurrency" || path == "/api/v1/admin/users/batch-limits" || path == "/api/v1/admin/users/batch-disable" {
				return
			}
			if !strings.HasPrefix(path, "/api/v1/admin/users/:id") {
				uid = 0
			}
			switch {
			case strings.Contains(path, "/users/") && strings.HasSuffix(path, "/attributes"):
				emit("admin.user_attributes_changed")
			case strings.Contains(path, "/users") && (strings.Contains(path, "limits") || strings.Contains(path, "concurrency") || strings.Contains(path, "platform-quota") || strings.Contains(path, "replace-group")):
				emit("admin.user_limits_changed")
			case strings.Contains(path, "/accounts"):
				emit("admin.account_changed")
			case strings.Contains(path, "/announcements") || strings.Contains(path, "/documentation") || strings.Contains(path, "/apps"):
				emit("admin.content_changed")
			case strings.Contains(path, "/users"):
				emit("admin.user_changed")
			default:
				emit("admin.settings_changed")
			}
		}
		// These routes represent a successful operation; substantive financial and task events use service hooks.
		key := method + " " + path
		routes := map[string]string{
			"POST /api/v1/auth/logout":                   "user.logged_out",
			"POST /api/v1/user/notify-email/verify":      "user.notify_email_verified",
			"PUT /api/v1/user/notify-email/toggle":       "user.notify_email_changed",
			"DELETE /api/v1/user/notify-email":           "user.notify_email_changed",
			"POST /api/v1/user/totp/enable":              "user.totp_changed",
			"POST /api/v1/user/totp/disable":             "user.totp_changed",
			"POST /api/v1/user/totp/step-up":             "user.step_up_verified",
			"POST /api/v1/user/passkeys/register/finish": "user.passkey_changed",
			"PATCH /api/v1/user/passkeys/:id":            "user.passkey_changed",
			"DELETE /api/v1/user/passkeys/:id":           "user.passkey_changed",
			"POST /api/v1/user/mcp/authorize":            "user.mcp_decided",
			"DELETE /api/v1/user/mcp/grants/:id":         "user.mcp_revoked",
		}
		if kind := routes[key]; kind != "" && uid > 0 {
			emit(kind)
		}
	}
}

func markGameplayAdmission(c *gin.Context, key *service.APIKey) {
	if !events.Enabled() || c.Request.Method != "POST" {
		return
	}
	c.Set("gameplay_request", true)
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	data := map[string]string{"api_key_id": events.S(key.ID), "endpoint": c.FullPath()}
	if key.GroupID != nil {
		data["group_id"] = events.S(*key.GroupID)
	}
	events.Publish("request.admitted", key.UserID, requestID, data)
}
func recordGameplayRejection(c *gin.Context, status int, code string) {
	if !events.Enabled() || c == nil || c.GetBool("gameplay_rejected") {
		return
	}
	key, ok := GetAPIKeyFromContext(c)
	if !ok {
		key, ok = GetOpsFallbackAPIKey(c)
	}
	if !ok || key == nil {
		return
	}
	upper := strings.ToUpper(code)
	kind := ""
	if status == 429 {
		kind = "request.rate_limited"
	} else if strings.Contains(upper, "BALANCE") || strings.Contains(upper, "QUOTA") || strings.Contains(upper, "SUBSCRIPTION") {
		kind = "request.insufficient_quota"
	}
	if kind == "" {
		return
	}
	c.Set("gameplay_rejected", true)
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	data := map[string]string{"api_key_id": events.S(key.ID), "http_status": events.S(status), "reason": code, "endpoint": c.FullPath()}
	if key.GroupID != nil {
		data["group_id"] = events.S(*key.GroupID)
	}
	events.Publish(kind, key.UserID, requestID, data)
	if strings.Contains(upper, "RATE_LIMIT") || strings.Contains(upper, "QUOTA") {
		events.Publish("billing.window_limit", key.UserID, requestID, data)
	}
}
