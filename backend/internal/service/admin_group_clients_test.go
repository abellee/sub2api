//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupBlockedClientsLifecycle(t *testing.T) {
	for _, simple := range []bool{false, true} {
		repo := &groupRepoStubForAdmin{createID: 1}
		svc := &adminServiceImpl{groupRepo: repo}
		if simple {
			svc.cfg = &config.Config{RunMode: config.RunModeSimple}
		}
		group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "clients", Platform: PlatformOpenAI, RateMultiplier: 1, BlockedUserAgents: "  Claude,cursor  "})
		require.NoError(t, err)
		require.Equal(t, "Claude,cursor", repo.created.BlockedUserAgents)
		require.Equal(t, group.BlockedUserAgents, cloneGroupForDuplicate(group, "fixture").BlockedUserAgents)
		keySvc := &APIKeyService{}
		key := &APIKey{ID: 1, User: &User{ID: 2}, Group: group}
		data, err := json.Marshal(keySvc.snapshotFromAPIKey(context.Background(), key))
		require.NoError(t, err)
		var snapshot APIKeyAuthSnapshot
		require.NoError(t, json.Unmarshal(data, &snapshot))
		require.Equal(t, group.BlockedUserAgents, keySvc.snapshotToAPIKey("fixture", &snapshot).Group.BlockedUserAgents)
		repo.getByID = group
		_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{Name: "renamed"})
		require.NoError(t, err)
		require.Equal(t, "Claude,cursor", repo.updated.BlockedUserAgents)
		for _, next := range []string{"Claude", ""} {
			_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{BlockedUserAgents: &next})
			require.NoError(t, err)
			require.Equal(t, next, repo.updated.BlockedUserAgents)
		}
		_, err = svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "default", RateMultiplier: 1})
		require.NoError(t, err)
		require.Empty(t, repo.created.BlockedUserAgents)
	}
}
