package store_test

import (
	"github.com/stretchr/testify/require"
	"lotteryd/internal/lottery"
	"lotteryd/internal/store/storetest"
	"testing"
)

func TestNotificationSettingsPreserveExplicitEmptyChannels(t *testing.T) {
	s := storetest.Open(t)
	for i, channels := range [][]string{nil, {}, {lottery.NotifyGroup}, {lottery.NotifyBrowser}} {
		id := int64(i + 1)
		require.NoError(t, s.SetActivityNotify(id, []string{lottery.StageStarted, lottery.StageResults}, channels))
		settings, err := s.ActivityNotificationSettings(id)
		require.NoError(t, err)
		want, err := lottery.ResolveNotifyChannels(channels)
		require.NoError(t, err)
		require.Equal(t, want, settings.Channels)
		require.Equal(t, []string{lottery.StageStarted, lottery.StageResults}, settings.Stages)
		all, err := s.ListActivityNotificationSettings()
		require.NoError(t, err)
		require.Equal(t, settings, all[id])
	}
	settings, err := s.ActivityNotificationSettings(999)
	require.NoError(t, err)
	require.Empty(t, settings.Stages)
	require.Equal(t, []string{lottery.NotifyBrowser, lottery.NotifyGroup}, settings.Channels)

	tx, err := s.Begin()
	require.NoError(t, err)
	_, err = tx.Exec(`INSERT INTO activity_notify(activity_id, enabled, stages) VALUES(1000, TRUE, '')`)
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	legacy, err := s.ActivityNotificationSettings(1000)
	require.NoError(t, err)
	require.Equal(t, lottery.AllNotifyStages(), legacy.Stages)
	require.Equal(t, []string{lottery.NotifyBrowser, lottery.NotifyGroup}, legacy.Channels)
}
