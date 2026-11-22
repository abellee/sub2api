//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdminGroupThinkingCompatLifecycle(t *testing.T) {
	ctx := context.Background()
	repo := &groupRepoStubForAdmin{createID: 51}
	svc := &adminServiceImpl{groupRepo: repo}
	cfg := GroupAnthropicThinkingCompat{Enabled: true, Models: []string{" claude-sonnet-5-5 ", "CLAUDE-SONNET-5-5", " "}}
	created, err := svc.CreateGroup(ctx, &CreateGroupInput{Name: "thinking", Platform: PlatformAnthropic, RateMultiplier: 1, AnthropicThinkingCompat: cfg})
	require.NoError(t, err)
	require.Equal(t, []string{"claude-sonnet-5-5"}, created.AnthropicThinkingCompat.Models)
	repo.getByID = created
	updated, err := svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{})
	require.NoError(t, err)
	require.Equal(t, created.AnthropicThinkingCompat, updated.AnthropicThinkingCompat, "omitted setting must be preserved")
	all := GroupAnthropicThinkingCompat{Enabled: true, AllModels: true}
	updated, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{AnthropicThinkingCompat: &all})
	require.NoError(t, err)
	require.Equal(t, all, updated.AnthropicThinkingCompat)
	off := GroupAnthropicThinkingCompat{}
	updated, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{AnthropicThinkingCompat: &off})
	require.NoError(t, err)
	require.False(t, updated.AnthropicThinkingCompat.Enabled)
	platform := PlatformOpenAI
	updated, err = svc.UpdateGroup(ctx, created.ID, &UpdateGroupInput{Platform: platform, AnthropicThinkingCompat: &all})
	require.NoError(t, err)
	require.Equal(t, GroupAnthropicThinkingCompat{}, updated.AnthropicThinkingCompat)
}

func TestAdminGroupThinkingCompatRejectsEmptySelection(t *testing.T) {
	cfg := GroupAnthropicThinkingCompat{Enabled: true, Models: []string{" "}}
	repo := &groupRepoStubForAdmin{createID: 51, getByID: &Group{ID: 1, Platform: PlatformAnthropic, Status: StatusActive}}
	svc := &adminServiceImpl{groupRepo: repo}
	_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "thinking", Platform: PlatformAnthropic, RateMultiplier: 1, AnthropicThinkingCompat: cfg})
	require.ErrorContains(t, err, "select at least one model")
	require.Nil(t, repo.created)
	_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{AnthropicThinkingCompat: &cfg})
	require.ErrorContains(t, err, "select at least one model")
	require.Nil(t, repo.updated)
}

func TestCloneGroupThinkingCompat(t *testing.T) {
	source := &Group{Platform: PlatformAnthropic, AnthropicThinkingCompat: GroupAnthropicThinkingCompat{Enabled: true, Models: []string{"claude-sonnet-*"}}}
	cloned := cloneGroupForDuplicate(source, "operation")
	require.Equal(t, source.AnthropicThinkingCompat, cloned.AnthropicThinkingCompat)
	cloned.AnthropicThinkingCompat.Models[0] = "changed"
	require.Equal(t, "claude-sonnet-*", source.AnthropicThinkingCompat.Models[0])
}
