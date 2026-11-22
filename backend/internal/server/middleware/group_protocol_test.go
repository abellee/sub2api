package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRequireGroupProtocol(t *testing.T) {
	gin.SetMode(gin.TestMode)
	routes := []struct{ method, route, url, protocol string }{
		{"POST", "/v1/messages", "/v1/messages", "anthropic"},
		{"POST", "/v1/messages/count_tokens", "/v1/messages/count_tokens", "anthropic"},
		{"POST", "/messages/count_tokens", "/messages/count_tokens", "anthropic"},
		{"POST", "/antigravity/v1/messages", "/antigravity/v1/messages", "anthropic"},
		{"POST", "/v1/chat/completions", "/v1/chat/completions", "openai"},
		{"POST", "/chat/completions", "/chat/completions", "openai"},
		{"POST", "/v1/responses", "/v1/responses", "openai"},
		{"POST", "/responses/*subpath", "/responses/compact", "openai"},
		{"POST", "/v1/responses/*subpath", "/v1/responses/messages", "openai"},
		{"GET", "/v1/responses", "/v1/responses", "openai"},
		{"GET", "/responses", "/responses", "openai"},
		{"POST", "/backend-api/codex/responses", "/backend-api/codex/responses", "openai"},
		{"GET", "/backend-api/codex/responses", "/backend-api/codex/responses", "openai"},
		{"POST", "/backend-api/codex/realtime/calls", "/backend-api/codex/realtime/calls", "openai"},
		{"GET", "/backend-api/codex/:call_id", "/backend-api/codex/call-1", "openai"},
		{"POST", "/v1/images/generations", "/v1/images/generations", "openai"},
		{"POST", "/v1beta/models/*modelAction", "/v1beta/models/gemini:generateContent", "google"},
		{"POST", "/v1beta/models/*modelAction", "/v1beta/models/messages:streamGenerateContent", "google"},
		{"POST", "/antigravity/v1beta/models/*modelAction", "/antigravity/v1beta/models/gemini:generateContent", "google"},
		{"POST", "/v1beta/cachedContents", "/v1beta/cachedContents", "google"},
		{"GET", "/v1beta/cachedContents/:cacheID", "/v1beta/cachedContents/1", "google"},
		{"POST", "/v1/systemone", "/v1/systemone", "systemone"},
		{"GET", "/v1/models", "/v1/models", ""},
		{"GET", "/v1/models/:model", "/v1/models/claude", ""},
		{"GET", "/antigravity/models", "/antigravity/models", ""},
		{"GET", "/backend-api/codex/models", "/backend-api/codex/models", ""},
		{"GET", "/v1beta/models", "/v1beta/models", ""},
		{"GET", "/v1/usage", "/v1/usage", ""},
		{"GET", "/v1/sub2api/billing", "/v1/sub2api/billing", ""},
	}
	for _, required := range []string{"", "anthropic", "openai", "google"} {
		for _, route := range routes {
			t.Run(required+"/"+route.method+route.url, func(t *testing.T) {
				r := gin.New()
				called := false
				var reason IngressRejectReason
				r.Use(func(c *gin.Context) {
					c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{RequiredProtocol: required}})
					c.Next()
					reason, _ = GetIngressRejectReason(c)
				}, RequireGroupProtocol())
				r.Handle(route.method, route.route, func(c *gin.Context) { called = true; c.Status(http.StatusNoContent) })
				// Headers cannot disguise an incompatible route; no request-body parsing is needed.
				req := httptest.NewRequest(route.method, route.url, strings.NewReader("not json"))
				req.Header.Set("Anthropic-Version", "2023-06-01")
				req.Header.Set("Upgrade", "websocket")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				allowed := required == "" || route.protocol == "" || required == route.protocol
				require.Equal(t, allowed, called)
				if allowed {
					require.Equal(t, http.StatusNoContent, w.Code)
					return
				}
				require.Equal(t, http.StatusBadRequest, w.Code)
				require.Equal(t, IngressRejectProtocolMismatch, reason)
				var response struct {
					Type  string `json:"type"`
					Error struct {
						Message string `json:"message"`
						Type    string `json:"type"`
						Status  string `json:"status"`
						Code    any    `json:"code"`
					} `json:"error"`
				}
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				require.Equal(t, requiredProtocolMessage(required), response.Error.Message)
				require.Contains(t, response.Error.Message, "缓存丢失")
				switch route.protocol {
				case "google":
					require.Equal(t, "INVALID_ARGUMENT", response.Error.Status)
				case "anthropic":
					require.Equal(t, "error", response.Type)
					require.Equal(t, "invalid_request_error", response.Error.Type)
				default:
					require.Equal(t, "protocol_mismatch", response.Error.Code)
				}
			})
		}
	}
}
