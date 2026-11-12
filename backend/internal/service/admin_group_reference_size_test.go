//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminGroupReferenceSizeSettingLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := &groupRepoStubForAdmin{createID: 1}
	svc := &adminServiceImpl{groupRepo: repo}
	created, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "reference-size", Platform: PlatformOpenAI, RateMultiplier: 1, AllowImageGeneration: true, ImageAutoSizeFromReference: true})
	require.NoError(t, err)
	require.True(t, repo.created.ImageAutoSizeFromReference)
	repo.getByID = created

	name := "renamed"
	_, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{Name: name})
	require.NoError(t, err)
	require.True(t, repo.updated.ImageAutoSizeFromReference, "an unrelated edit must preserve the setting")

	enabled := false
	_, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{ImageAutoSizeFromReference: &enabled})
	require.NoError(t, err)
	require.False(t, repo.updated.ImageAutoSizeFromReference)

	_, err = svc.CreateGroup(ctx, &CreateGroupInput{Name: "default-off", Platform: PlatformOpenAI, RateMultiplier: 1})
	require.NoError(t, err)
	require.False(t, repo.created.ImageAutoSizeFromReference)
}

func TestAPIKeyReferenceSizeSettingSurvivesSnapshot(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, User: &User{ID: 2}, Group: &Group{ID: 3, Platform: PlatformOpenAI, ImageAutoSizeFromReference: true}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	require.True(t, snapshot.Group.ImageAutoSizeFromReference)
	restored := svc.snapshotToAPIKey("fixture", snapshot)
	require.True(t, restored.Group.ImageAutoSizeFromReference)
}
