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

func TestGroupHandlerSyncIsolationRoundTrip(t *testing.T) {
	svc := newStubAdminService()
	r := newSimpleModeGroupRouter(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(`{"name":"sync","platform":"openai","sync_isolation_enabled":true}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, svc.createdGroups[0].SyncIsolationEnabled)
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/groups/2", strings.NewReader(`{"sync_isolation_enabled":false}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, svc.updatedGroups[len(svc.updatedGroups)-1].SyncIsolationEnabled)
	require.False(t, *svc.updatedGroups[len(svc.updatedGroups)-1].SyncIsolationEnabled)
	g := &service.Group{SyncIsolationEnabled: true, SyncIsolationSourceGroupID: 9}
	require.True(t, dto.GroupFromServiceAdmin(g).SyncIsolationEnabled)
	require.Equal(t, int64(9), dto.GroupFromService(g).SyncIsolationSourceGroupID)
	require.True(t, groupForSimpleMode(g).SyncIsolationEnabled)
}
