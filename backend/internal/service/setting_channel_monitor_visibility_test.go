//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestChannelMonitorRuntimeVisibleToUser(t *testing.T) {
	t.Parallel()

	enabledAll := ChannelMonitorRuntime{Enabled: true, Visibility: ChannelMonitorVisibilityAll}
	enabledSelected := ChannelMonitorRuntime{
		Enabled:        true,
		Visibility:     ChannelMonitorVisibilitySelected,
		VisibleUserIDs: []int64{7, 11},
	}
	disabled := ChannelMonitorRuntime{Enabled: false, Visibility: ChannelMonitorVisibilityAll}

	require.False(t, disabled.VisibleToUser(7, true), "disabled hides even admins from the user-facing surface")
	require.True(t, enabledAll.VisibleToUser(0, false))
	require.True(t, enabledAll.VisibleToUser(7, false))
	require.True(t, enabledSelected.VisibleToUser(99, true), "admins bypass the allow-list")
	require.True(t, enabledSelected.VisibleToUser(7, false))
	require.False(t, enabledSelected.VisibleToUser(8, false))
	require.False(t, enabledSelected.VisibleToUser(0, false), "anonymous is hidden in selected mode")

	empty := ChannelMonitorRuntime{Enabled: true}
	require.False(t, empty.VisibleToUser(7, false), "empty visibility fail-closes for non-admins")
	require.True(t, empty.VisibleToUser(7, true), "admins still pass when visibility is empty")
}

func TestGetPublicSettingsChannelMonitorVisibilityDefaultsSelected(t *testing.T) {
	t.Parallel()

	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorEnabled: "true",
	}}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.Equal(t, ChannelMonitorVisibilitySelected, settings.ChannelMonitorVisibility)

	svc.ApplyChannelMonitorPublicVisibility(context.Background(), settings, 7, false)
	require.False(t, settings.ChannelMonitorVisible)

	settings, err = svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	svc.ApplyChannelMonitorPublicVisibility(context.Background(), settings, 7, true)
	require.True(t, settings.ChannelMonitorVisible)
}

func TestGetChannelMonitorRuntimeMissingVisibilityIsSelected(t *testing.T) {
	t.Parallel()

	missing := NewSettingService(&settingPublicRepoStub{values: map[string]string{}}, &config.Config{}).
		GetChannelMonitorRuntime(context.Background())
	require.Equal(t, ChannelMonitorVisibilitySelected, missing.Visibility)
	require.False(t, missing.VisibleToUser(1, false))
	require.True(t, missing.VisibleToUser(1, true))
}

func TestApplyChannelMonitorPublicVisibilityHidesSelectedCallers(t *testing.T) {
	t.Parallel()

	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorEnabled:        "true",
		SettingKeyChannelMonitorVisibility:     ChannelMonitorVisibilitySelected,
		SettingKeyChannelMonitorVisibleUserIDs: "[7]",
	}}, &config.Config{})

	settings, err := svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	require.True(t, settings.ChannelMonitorEnabled)
	require.Equal(t, ChannelMonitorVisibilitySelected, settings.ChannelMonitorVisibility)

	svc.ApplyChannelMonitorPublicVisibility(context.Background(), settings, 8, false)
	require.True(t, settings.ChannelMonitorEnabled, "enabled stays the global switch")
	require.False(t, settings.ChannelMonitorVisible)

	settings, err = svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	svc.ApplyChannelMonitorPublicVisibility(context.Background(), settings, 7, false)
	require.True(t, settings.ChannelMonitorEnabled)
	require.True(t, settings.ChannelMonitorVisible)

	settings, err = svc.GetPublicSettings(context.Background())
	require.NoError(t, err)
	svc.ApplyChannelMonitorPublicVisibility(context.Background(), settings, 8, true)
	require.True(t, settings.ChannelMonitorEnabled)
	require.True(t, settings.ChannelMonitorVisible)
}

func TestGetPublicSettingsForInjectionHidesSelectedModeAnonymously(t *testing.T) {
	t.Parallel()

	svc := NewSettingService(&settingPublicRepoStub{values: map[string]string{
		SettingKeyChannelMonitorEnabled:        "true",
		SettingKeyChannelMonitorVisibility:     ChannelMonitorVisibilitySelected,
		SettingKeyChannelMonitorVisibleUserIDs: "[7]",
	}}, &config.Config{})

	raw, err := svc.GetPublicSettingsForInjection(context.Background())
	require.NoError(t, err)
	payload, ok := raw.(*PublicSettingsInjectionPayload)
	require.True(t, ok)
	require.True(t, payload.ChannelMonitorEnabled, "SSR keeps the global enabled switch")
	require.Equal(t, ChannelMonitorVisibilitySelected, payload.ChannelMonitorVisibility)
	require.False(t, payload.ChannelMonitorVisible, "anonymous selected-mode is not visible")
}

func TestParseChannelMonitorVisibleUserIDsNormalizes(t *testing.T) {
	t.Parallel()

	require.Equal(t, []int64{}, parseChannelMonitorVisibleUserIDs(""))
	require.Equal(t, []int64{3, 9}, parseChannelMonitorVisibleUserIDs("[9,0,3,3,-1]"))
	require.Equal(t, []int64{}, parseChannelMonitorVisibleUserIDs("not-json"))
	require.Equal(t, "[3,9]", marshalChannelMonitorVisibleUserIDs([]int64{9, 3, 3}))
}
