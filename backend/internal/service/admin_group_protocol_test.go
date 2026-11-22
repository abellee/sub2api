//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestAdminGroupRequiredProtocolLifecycle(t *testing.T) {
	for _, simple := range []bool{false, true} {
		for _, protocol := range []string{"", "anthropic", "openai", "google"} {
			t.Run(protocol+map[bool]string{true: "/simple", false: "/standard"}[simple], func(t *testing.T) {
				repo := &groupRepoStubForAdmin{createID: 1}
				svc := &adminServiceImpl{groupRepo: repo}
				if simple {
					svc.cfg = &config.Config{RunMode: config.RunModeSimple}
				}
				group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "protocol", Platform: PlatformAnthropic, RateMultiplier: 1, RequiredProtocol: protocol})
				require.NoError(t, err)
				require.Equal(t, protocol, repo.created.RequiredProtocol)
				require.Equal(t, protocol, cloneGroupForDuplicate(group, "test").RequiredProtocol)
				repo.getByID = group
				_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{Name: "renamed"})
				require.NoError(t, err)
				require.Equal(t, protocol, repo.updated.RequiredProtocol, "omission preserves policy")
				next := GroupProtocolGoogle
				_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{RequiredProtocol: &next})
				require.NoError(t, err)
				require.Equal(t, next, repo.updated.RequiredProtocol)
				next = ""
				_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{RequiredProtocol: &next})
				require.NoError(t, err)
				require.Empty(t, repo.updated.RequiredProtocol, "empty clears policy")
			})
		}
	}
}

func TestAdminGroupRequiredProtocolRejectsInvalidValues(t *testing.T) {
	repo := &groupRepoStubForAdmin{getByID: &Group{ID: 1, Platform: PlatformAnthropic}}
	svc := &adminServiceImpl{groupRepo: repo}
	for _, protocol := range []string{"gemini", "Anthropic", "other", " "} {
		_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "invalid", RateMultiplier: 1, RequiredProtocol: protocol})
		require.Error(t, err)
		_, err = svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{RequiredProtocol: &protocol})
		require.Error(t, err)
	}
	require.Nil(t, repo.created)
	require.Nil(t, repo.updated)
}

func TestGroupRequiredProtocolSurvivesAuthSnapshot(t *testing.T) {
	svc := &APIKeyService{}
	key := &APIKey{ID: 1, User: &User{ID: 2}, Group: &Group{ID: 3, Platform: PlatformAnthropic, RequiredProtocol: GroupProtocolAnthropic}}
	snapshot := svc.snapshotFromAPIKey(context.Background(), key)
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	var restored APIKeyAuthSnapshot
	require.NoError(t, json.Unmarshal(data, &restored))
	require.Equal(t, GroupProtocolAnthropic, svc.snapshotToAPIKey("fixture", &restored).Group.RequiredProtocol)
}
