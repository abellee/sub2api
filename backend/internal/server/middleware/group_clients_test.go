package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGroupBlocksUserAgent(t *testing.T) {
	for _, tt := range []struct {
		name, blocked string
		uas           []string
		want          bool
	}{
		{"prefix and case", "claude", []string{"Claude-Code/2.0 (external, cli)"}, true},
		{"middle keyword allowed", "claude", []string{"OtherClient/1.0 (Claude-Code/2.0)"}, false},
		{"suffix keyword allowed", "claude", []string{"myclaude"}, false},
		{"second keyword", "claude,CuRsOr", []string{"cursor/1.2"}, true},
		{"spaces and empty items", " , CLAUDE , , cursor, ", []string{"claude-cli/2.0"}, true},
		{"empty items do not match everything", " , , ", []string{"codex/1.0"}, false},
		{"empty config", "", []string{"Claude-Code/2.0"}, false},
		{"other client", "claude,cursor", []string{"codex/1.0"}, false},
		{"missing UA", "claude", nil, false},
		{"empty UA", "claude", []string{""}, false},
		{"literal not regexp", "claude.*", []string{"claude-code"}, false},
		{"multiple headers", "claude", []string{"codex/1.0", "CLAUDE/2.0"}, true},
	} {
		t.Run(tt.name, func(t *testing.T) { require.Equal(t, tt.want, groupBlocksUserAgent(tt.blocked, tt.uas)) })
	}
}

func TestRejectGroupClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, route := range []struct{ method, route, url, errorShape string }{
		{"POST", "/v1/messages", "/v1/messages", "permission_error"},
		{"POST", "/antigravity/v1/messages", "/antigravity/v1/messages", "permission_error"},
		{"POST", "/v1/chat/completions", "/v1/chat/completions", "client_blocked"},
		{"POST", "/v1/responses", "/v1/responses", "client_blocked"},
		{"GET", "/backend-api/codex/responses", "/backend-api/codex/responses", "client_blocked"},
		{"POST", "/v1beta/models/*modelAction", "/v1beta/models/gemini:generateContent", "PERMISSION_DENIED"},
		{"POST", "/antigravity/v1beta/models/*modelAction", "/antigravity/v1beta/models/gemini:generateContent", "PERMISSION_DENIED"},
		{"GET", "/v1beta/models", "/v1beta/models", "PERMISSION_DENIED"},
		{"GET", "/v1/models", "/v1/models", "client_blocked"},
	} {
		for _, ua := range []string{"CLAUDE-Code/2.0", "codex/1.0", "OtherClient/1.0 (Claude-Code/2.0)", ""} {
			t.Run(route.method+route.url+ua, func(t *testing.T) {
				r := gin.New()
				called := false
				var reason IngressRejectReason
				r.Use(func(c *gin.Context) {
					c.Set(string(ContextKeyAPIKey), &service.APIKey{Group: &service.Group{BlockedUserAgents: "claude,cursor"}})
					c.Next()
					reason, _ = GetIngressRejectReason(c)
				}, RejectGroupClients())
				r.Handle(route.method, route.route, func(c *gin.Context) { called = true; c.Status(http.StatusNoContent) })
				w := httptest.NewRecorder()
				req := httptest.NewRequest(route.method, route.url, strings.NewReader("not json"))
				req.Header.Set("User-Agent", ua)
				req.Header.Set("Upgrade", "websocket")
				r.ServeHTTP(w, req)
				if ua != "CLAUDE-Code/2.0" {
					require.True(t, called)
					require.Equal(t, http.StatusNoContent, w.Code)
					return
				}
				require.False(t, called)
				require.Equal(t, http.StatusForbidden, w.Code)
				require.Equal(t, IngressRejectClientBlocked, reason)
				require.Contains(t, w.Body.String(), route.errorShape)
				require.Contains(t, w.Body.String(), "可能会导致缓存丢失，请更换其它客户端")
			})
		}
	}
}
