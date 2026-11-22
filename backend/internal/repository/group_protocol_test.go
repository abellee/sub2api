package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupRequiredProtocolPersistenceAndAuthProjection(t *testing.T) {
	ctx := context.Background()
	keyRepo, client := newAPIKeyRepoSQLite(t)
	groupRepo := newGroupRepositoryWithSQL(client, keyRepo.sql)
	group := &service.Group{Name: "protocol", Platform: service.PlatformAnthropic, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RequiredProtocol: service.GroupProtocolAnthropic, RateMultiplier: 1}
	require.NoError(t, createGroupRecord(ctx, client, group))
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "protocol@test.invalid")
	key := &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-protocol-fixture", Name: "protocol", Status: service.StatusActive}
	require.NoError(t, keyRepo.Create(ctx, key))
	for _, protocol := range []string{"anthropic", "openai", "google", ""} {
		group.RequiredProtocol = protocol
		require.NoError(t, groupRepo.Update(ctx, group))
		stored, err := groupRepo.GetByIDLite(ctx, group.ID)
		require.NoError(t, err)
		require.Equal(t, protocol, stored.RequiredProtocol)
		auth, err := keyRepo.GetByKeyForAuth(ctx, key.Key)
		require.NoError(t, err)
		require.NotNil(t, auth.Group)
		require.Equal(t, protocol, auth.Group.RequiredProtocol)
	}
}
