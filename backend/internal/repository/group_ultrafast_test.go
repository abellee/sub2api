package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupUltrafastPersistenceAndAuthProjection(t *testing.T) {
	keys, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	groups := newGroupRepositoryWithSQL(client, keys.sql)
	group := &service.Group{Name: "ultrafast", Platform: service.PlatformOpenAI,
		Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		RateMultiplier: 1, ForceOpenAIUltrafast: true}
	require.NoError(t, groups.Create(ctx, group))
	created, err := groups.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.True(t, created.ForceOpenAIUltrafast)
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "ultrafast@test.invalid")
	key := &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-ultrafast-unit", Name: "test", Status: service.StatusActive}
	require.NoError(t, keys.Create(ctx, key))

	for _, enabled := range []bool{true, false, true} {
		group.ForceOpenAIUltrafast = enabled
		require.NoError(t, groups.Update(ctx, group))
		stored, err := groups.GetByID(ctx, group.ID)
		require.NoError(t, err)
		require.Equal(t, enabled, stored.ForceOpenAIUltrafast)
		authKey, err := keys.GetByKeyForAuth(ctx, key.Key)
		require.NoError(t, err)
		require.NotNil(t, authKey.Group)
		require.Equal(t, enabled, authKey.Group.ForceOpenAIUltrafast)
	}
}
