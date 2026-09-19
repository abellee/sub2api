package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorVisibilityMigration(t *testing.T) {
	content, err := FS.ReadFile("239_channel_monitor_visibility.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "VALUES ('channel_monitor_visibility', 'selected')")
	require.Contains(t, sql, "VALUES ('channel_monitor_visible_user_ids', '[]')")
	require.Contains(t, sql, "ON CONFLICT (key) DO NOTHING")
}
