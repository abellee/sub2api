//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupSyncIsolationLifecycle(t *testing.T) {
	for _, simple := range []bool{false, true} {
		repo := &groupRepoStubForAdmin{createID: 1}
		svc := &adminServiceImpl{groupRepo: repo}
		if simple {
			svc.cfg = &config.Config{RunMode: config.RunModeSimple}
		}
		group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "sync", Platform: PlatformOpenAI, RateMultiplier: 1, SyncIsolationEnabled: true})
		require.NoError(t, err)
		require.True(t, repo.created.SyncIsolationEnabled)
		require.True(t, cloneGroupForDuplicate(group, "fixture").SyncIsolationEnabled)
		repo.getByID = group
		_, err = svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{Name: "renamed"})
		require.NoError(t, err)
		require.True(t, repo.updated.SyncIsolationEnabled, "omission preserves the setting")
		disabled := false
		_, err = svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{SyncIsolationEnabled: &disabled})
		require.NoError(t, err)
		require.False(t, repo.updated.SyncIsolationEnabled)
	}
}

func TestSyncIsolationSubscriptionBindingRequiresGrant(t *testing.T) {
	svc := &APIKeyService{}
	group := &Group{ID: 4, SyncIsolationSourceGroupID: 3, SubscriptionType: SubscriptionTypeSubscription, IsExclusive: true}
	user := &User{ID: 2}
	require.False(t, svc.canUserBindGroupInternal(user, group, map[int64]bool{3: true}))
	user.AllowedGroups = []int64{4}
	require.True(t, svc.canUserBindGroupInternal(user, group, map[int64]bool{3: true}))
	require.False(t, svc.canUserBindGroupInternal(user, group, nil), "a grant cannot replace an active subscription")
}
