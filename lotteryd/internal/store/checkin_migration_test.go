package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCheckInRewardsMigrationPreservesExistingSchedule(t *testing.T) {
	dsn := os.Getenv("LOTTERYD_TEST_PG_DSN")
	if dsn == "" {
		t.Skip("set LOTTERYD_TEST_PG_DSN to run PostgreSQL-backed tests")
	}
	u, err := url.Parse(dsn)
	require.NoError(t, err)
	u.Path = "/postgres"
	admin, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	defer admin.Close()
	name := fmt.Sprintf("lottery_test_migration_%d", time.Now().UnixNano())
	_, err = admin.Exec(`CREATE DATABASE ` + name)
	require.NoError(t, err)
	defer func() {
		_, err := admin.Exec(`DROP DATABASE ` + name + ` WITH (FORCE)`)
		require.NoError(t, err)
	}()
	u.Path = "/" + name
	db, err := sql.Open("pgx", u.String())
	require.NoError(t, err)
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE points_settings (id BOOLEAN PRIMARY KEY DEFAULT TRUE, daily_checkin_points BIGINT NOT NULL DEFAULT 10);
		INSERT INTO points_settings (id,daily_checkin_points) VALUES (TRUE,1000000)`)
	require.NoError(t, err)
	_, err = db.Exec(pointsSchemaSQL)
	require.NoError(t, err)
	s := &Store{db: db}
	settings, err := s.GetPointsSettings()
	require.NoError(t, err)
	require.Equal(t, []int64{1000000, 1000000, 1000000, 1000000, 1000000, 2000000, 2000000}, settings.DailyCheckInRewards)
	custom := PointsSettings{DailyCheckInRewards: []int64{12, 40, 25, 18, 31, 7, 9}}
	require.NoError(t, s.SavePointsSettings(custom))
	_, err = db.Exec(pointsSchemaSQL)
	require.NoError(t, err)
	settings, err = s.GetPointsSettings()
	require.NoError(t, err)
	require.Equal(t, custom, settings, "restarts must preserve the seven configured rewards")
	_, err = db.Exec(`DELETE FROM points_settings; INSERT INTO points_settings (id) VALUES (TRUE)`)
	require.NoError(t, err)
	settings, err = s.GetPointsSettings()
	require.NoError(t, err)
	require.Equal(t, []int64{10, 10, 10, 10, 10, 10, 10}, settings.DailyCheckInRewards, "new defaults do not double any day")
}
