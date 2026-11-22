package admin

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupAccountFailoverRequestsAndResponses(t *testing.T) {
	var create CreateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"name":"retry","account_error_failover_enabled":true}`), &create))
	require.True(t, create.AccountErrorFailoverEnabled)
	sanitizeCreateGroupRequestForSimpleMode(&create)
	require.True(t, create.AccountErrorFailoverEnabled)
	var update UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{"account_error_failover_enabled":false}`), &update))
	sanitizeUpdateGroupRequestForSimpleMode(&update)
	require.NotNil(t, update.AccountErrorFailoverEnabled)
	require.False(t, *update.AccountErrorFailoverEnabled)
	require.NoError(t, json.Unmarshal([]byte(`{}`), &create))
	var omitted UpdateGroupRequest
	require.NoError(t, json.Unmarshal([]byte(`{}`), &omitted))
	require.Nil(t, omitted.AccountErrorFailoverEnabled)
	group := &service.Group{ID: 1, AccountErrorFailoverEnabled: true}
	require.True(t, dto.GroupFromServiceAdmin(group).AccountErrorFailoverEnabled)
	require.True(t, groupForSimpleMode(group).AccountErrorFailoverEnabled)
}
