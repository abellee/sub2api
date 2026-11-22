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

func TestGroupHandlerRequiredProtocolRoundTrip(t *testing.T) {
	svc := newStubAdminService()
	r := newSimpleModeGroupRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(`{"name":"protocol","platform":"anthropic","required_protocol":"anthropic"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, service.GroupProtocolAnthropic, svc.createdGroups[0].RequiredProtocol)

	for _, value := range []string{"openai", "google", ""} {
		w = httptest.NewRecorder()
		req = httptest.NewRequest(http.MethodPut, "/groups/2", strings.NewReader(`{"required_protocol":"`+value+`"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		require.NotNil(t, svc.updatedGroups[len(svc.updatedGroups)-1].RequiredProtocol)
		require.Equal(t, value, *svc.updatedGroups[len(svc.updatedGroups)-1].RequiredProtocol)
	}
	group := &service.Group{RequiredProtocol: service.GroupProtocolAnthropic}
	require.Equal(t, group.RequiredProtocol, dto.GroupFromServiceAdmin(group).RequiredProtocol)
	require.Equal(t, group.RequiredProtocol, dto.GroupFromService(group).RequiredProtocol)
	require.Equal(t, group.RequiredProtocol, groupForSimpleMode(group).RequiredProtocol)
}
