package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupBlockedClientsPersistenceAndAuthProjection(t *testing.T) {
	ctx := context.Background()
	keyRepo, client := newAPIKeyRepoSQLite(t)
	groupRepo := newGroupRepositoryWithSQL(client, keyRepo.sql)
	group := &service.Group{Name: "clients", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, BlockedUserAgents: "claude,cursor", RateMultiplier: 1}
	require.NoError(t, createGroupRecord(ctx, client, group))
	stored, err := groupRepo.GetByIDLite(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, group.BlockedUserAgents, stored.BlockedUserAgents)
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "clients@test.invalid")
	key := &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-clients-fixture", Name: "clients", Status: service.StatusActive}
	require.NoError(t, keyRepo.Create(ctx, key))
	for _, keywords := range []string{"claude,cursor", "Claude", ""} {
		group.BlockedUserAgents = keywords
		require.NoError(t, groupRepo.Update(ctx, group))
		auth, err := keyRepo.GetByKeyForAuth(ctx, key.Key)
		require.NoError(t, err)
		require.NotNil(t, auth.Group)
		require.Equal(t, keywords, auth.Group.BlockedUserAgents)
	}
}
