package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupMapperClientPolicyVisibleToKeyOwners(t *testing.T) {
	group := &service.Group{RequiredProtocol: service.GroupProtocolAnthropic, BlockedUserAgents: "claude,cursor"}
	for _, mapper := range []func(*service.Group) *Group{GroupFromService, GroupFromServiceShallow} {
		fields := marshalToMap(t, mapper(group))
		require.Equal(t, "anthropic", fields["required_protocol"])
		require.Equal(t, "claude,cursor", fields["blocked_user_agents"])
	}
}
