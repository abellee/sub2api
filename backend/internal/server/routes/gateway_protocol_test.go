package routes

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGatewayRequiredProtocolRejectsBeforeDispatch(t *testing.T) {
	for _, platform := range []string{service.PlatformAnthropic, service.PlatformComposite} {
		group := &service.Group{Platform: platform, RequiredProtocol: service.GroupProtocolAnthropic,
			ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"claude-*"}}}
		router := newGatewayRoutesTestRouterWithGroup(group)
		for _, route := range []struct{ method, path string }{
			{"POST", "/v1/chat/completions"}, {"POST", "/chat/completions"},
			{"POST", "/v1/responses"}, {"POST", "/responses"},
			{"POST", "/v1/responses/compact"}, {"POST", "/responses/compact"},
			{"POST", "/backend-api/codex/responses"}, {"POST", "/backend-api/codex/responses/compact"},
			{"GET", "/v1/responses"}, {"GET", "/responses"}, {"GET", "/backend-api/codex/responses"},
			{"POST", "/v1/images/generations"}, {"POST", "/v1/embeddings"},
		} {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{"model":"gpt-5"}`))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)
			require.Equal(t, http.StatusBadRequest, w.Code, "%s %s", platform, route.path)
			require.Contains(t, w.Body.String(), "protocol_mismatch")
			require.Contains(t, w.Body.String(), "/v1/messages")
		}
	}
	for _, path := range []string{"/v1/messages", "/v1/messages/count_tokens", "/messages/count_tokens", "/antigravity/v1/messages", "/antigravity/v1/messages/count_tokens"} {
		router := newGatewayRoutesTestRouterWithGroup(&service.Group{Platform: service.PlatformAnthropic, RequiredProtocol: service.GroupProtocolGoogle})
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("POST", path, strings.NewReader(`{}`)))
		require.Equal(t, http.StatusBadRequest, w.Code, path)
		require.Contains(t, w.Body.String(), "Google")
	}
}

func TestGatewayRequiredProtocolCoversEveryChainBeforeModelRouting(t *testing.T) {
	data, err := os.ReadFile("gateway.go")
	require.NoError(t, err)
	source := string(data)
	for _, group := range []string{"gateway", "gemini", "antigravityV1", "antigravityV1Beta"} {
		require.Regexp(t, regexp.QuoteMeta(group+".Use(groupProtocol)")+`\s*`+regexp.QuoteMeta(group+".Use(groupModelAllowlist)"), source)
	}
	require.Contains(t, source, "gin.HandlerFunc(apiKeyAuth), groupClientFilter, groupProtocol, groupModelAllowlist, compositeTarget")
}
