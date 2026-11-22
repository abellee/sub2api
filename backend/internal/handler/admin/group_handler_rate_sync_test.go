package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupRateSyncRequestAndAdminResponse(t *testing.T) {
	var create CreateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"rate_sync_group_ids":[2,3]}`), &create))
	require.Equal(t, []int64{2, 3}, create.RateSyncGroupIDs)
	var update UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"rate_sync_group_ids":[]}`), &update))
	require.NotNil(t, update.RateSyncGroupIDs)
	require.Empty(t, *update.RateSyncGroupIDs)
	var omitted UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.RateSyncGroupIDs)
	g := &service.Group{ID: 1, RateSyncGroupIDs: []int64{2, 3}}
	adminJSON, err := json.Marshal(dto.GroupFromServiceAdmin(g))
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"rate_sync_group_ids":[2,3]`)
	userJSON, err := json.Marshal(dto.GroupFromService(g))
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "rate_sync_group_ids")
	sanitizeCreateGroupRequestForSimpleMode(&create)
	require.Nil(t, create.RateSyncGroupIDs)
	sanitizeUpdateGroupRequestForSimpleMode(&update)
	require.Nil(t, update.RateSyncGroupIDs)
}
