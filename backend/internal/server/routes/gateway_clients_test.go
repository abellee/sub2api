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

func TestGatewayBlockedClientsRejectsBeforeDispatch(t *testing.T) {
	// A GPT group must reject a Claude UA even on the correct OpenAI endpoint.
	router := newGatewayRoutesTestRouterWithGroup(&service.Group{Platform: service.PlatformOpenAI,
		RequiredProtocol: service.GroupProtocolOpenAI, BlockedUserAgents: "claude,cursor"})
	for _, route := range []struct{ method, path string }{
		{"POST", "/v1/chat/completions"}, {"POST", "/chat/completions"},
		{"POST", "/v1/responses"}, {"POST", "/responses"},
		{"POST", "/v1/responses/compact"}, {"POST", "/responses/compact"},
		{"POST", "/backend-api/codex/responses"}, {"POST", "/backend-api/codex/responses/compact"},
		{"GET", "/v1/responses"}, {"GET", "/responses"}, {"GET", "/backend-api/codex/responses"},
		{"POST", "/v1/messages"}, {"POST", "/antigravity/v1/messages"},
		{"POST", "/v1/images/generations"}, {"POST", "/v1/embeddings"},
		{"GET", "/v1/models"}, {"GET", "/antigravity/models"}, {"GET", "/v1/sub2api/billing"},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(route.method, route.path, strings.NewReader(`{"model":"gpt-5"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "ClAuDe-Code/2.0")
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusForbidden, w.Code, route.path)
		require.Contains(t, w.Body.String(), "可能会导致缓存丢失，请更换其它客户端")
	}
}

func TestGatewayBlockedClientsCoversEveryChain(t *testing.T) {
	data, err := os.ReadFile("gateway.go")
	require.NoError(t, err)
	source := string(data)
	for _, group := range []string{"gemini", "antigravityV1", "antigravityV1Beta"} {
		require.Regexp(t, regexp.QuoteMeta(group+".Use(groupClientFilter)")+`\s*`+regexp.QuoteMeta(group+".Use(groupProtocol)"), source)
	}
	require.Contains(t, source, "gin.HandlerFunc(apiKeyAuth), groupClientFilter, groupProtocol, groupModelAllowlist, compositeTarget")
}
