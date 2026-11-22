package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupHandlerBlockedClientsRoundTrip(t *testing.T) {
	svc := newStubAdminService()
	r := newSimpleModeGroupRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(`{"name":"clients","platform":"openai","blocked_user_agents":"claude,cursor"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "claude,cursor", svc.createdGroups[0].BlockedUserAgents)
	for _, value := range []string{"Claude", ""} {
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/groups/2", strings.NewReader(`{"blocked_user_agents":"`+value+`"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		input := svc.updatedGroups[len(svc.updatedGroups)-1]
		require.NotNil(t, input.BlockedUserAgents)
		require.Equal(t, value, *input.BlockedUserAgents)
	}
	group := &service.Group{BlockedUserAgents: "claude,cursor"}
	require.Equal(t, group.BlockedUserAgents, dto.GroupFromServiceAdmin(group).BlockedUserAgents)
	require.Equal(t, group.BlockedUserAgents, groupForSimpleMode(group).BlockedUserAgents)
}
