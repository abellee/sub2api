//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminGroupAccountFailoverLifecycle(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 1}
	svc := &adminServiceImpl{groupRepo: repo}
	created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "retry", Platform: PlatformOpenAI, RateMultiplier: 1, AccountErrorFailoverEnabled: true,
	})
	require.NoError(t, err)
	require.True(t, created.AccountErrorFailoverEnabled)
	repo.getByID = created
	_, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{Name: "renamed"})
	require.NoError(t, err)
	require.True(t, repo.updated.AccountErrorFailoverEnabled, "omission must preserve the saved switch")
	disabled := false
	_, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{AccountErrorFailoverEnabled: &disabled})
	require.NoError(t, err)
	require.False(t, repo.updated.AccountErrorFailoverEnabled)
	_, err = svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "default", Platform: PlatformOpenAI, RateMultiplier: 1})
	require.NoError(t, err)
	require.False(t, repo.created.AccountErrorFailoverEnabled)
}

func TestAPIKeyGroupAccountFailoverSurvivesSnapshot(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, User: &User{ID: 2}, Group: &Group{ID: 3, Platform: PlatformOpenAI, AccountErrorFailoverEnabled: true}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	require.True(t, snapshot.Group.AccountErrorFailoverEnabled)
	require.True(t, svc.snapshotToAPIKey("fixture", snapshot).Group.AccountErrorFailoverEnabled)
}
