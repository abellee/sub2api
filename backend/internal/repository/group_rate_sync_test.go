package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGroupRateSyncAtomicPersistence(t *testing.T) {
	keys, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	_, err := keys.sql.ExecContext(ctx, `CREATE TABLE scheduler_outbox(event_type TEXT, account_id INTEGER, group_id INTEGER, payload TEXT, dedup_key TEXT);
        CREATE UNIQUE INDEX rate_sync_outbox_dedup ON scheduler_outbox(dedup_key) WHERE dedup_key IS NOT NULL`)
	require.NoError(t, err)
	repo := newGroupRepositoryWithSQL(client, keys.sql)
	source := &service.Group{Name: "source", Platform: service.PlatformOpenAI, RateMultiplier: 1}
	target := &service.Group{Name: "target", Platform: service.PlatformOpenAI, RateMultiplier: 2, Status: "inactive", ImageRateMultiplier: 7}
	other := &service.Group{Name: "other", Platform: service.PlatformGemini, RateMultiplier: 3}
	for _, g := range []*service.Group{source, target, other} {
		require.NoError(t, repo.Create(ctx, g))
	}
	target.RateSyncGroupIDs = []int64{other.ID, source.ID}
	require.NoError(t, repo.Update(ctx, target))
	source.RateSyncGroupIDs = []int64{target.ID}
	source.RateMultiplier = 0.5
	_, err = keys.sql.ExecContext(ctx, "DELETE FROM scheduler_outbox")
	require.NoError(t, err)
	require.NoError(t, repo.UpdateWithRateSync(ctx, source, source.RateSyncGroupIDs))
	got, err := repo.GetByIDLite(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, source.RateSyncGroupIDs, got.RateSyncGroupIDs)
	got, err = repo.GetByIDLite(ctx, target.ID)
	require.NoError(t, err)
	require.Equal(t, 0.5, got.RateMultiplier)
	require.Equal(t, 7.0, got.ImageRateMultiplier)
	require.Equal(t, "inactive", got.Status)
	require.Equal(t, target.RateSyncGroupIDs, got.RateSyncGroupIDs)
	got, err = repo.GetByIDLite(ctx, other.ID)
	require.NoError(t, err)
	require.Equal(t, 3.0, got.RateMultiplier, "target links must not cascade")
	var events int
	rows, err := keys.sql.QueryContext(ctx, "SELECT COUNT(*) FROM scheduler_outbox")
	require.NoError(t, err)
	require.True(t, rows.Next())
	require.NoError(t, rows.Scan(&events))
	require.NoError(t, rows.Close())
	require.Equal(t, 2, events)

	// A disappeared target rolls back the source and existing targets together.
	source.RateMultiplier = 0.7
	require.ErrorIs(t, repo.UpdateWithRateSync(ctx, source, []int64{target.ID, 99999}), service.ErrGroupNotFound)
	for _, id := range []int64{source.ID, target.ID} {
		got, err = repo.GetByIDLite(ctx, id)
		require.NoError(t, err)
		require.Equal(t, 0.5, got.RateMultiplier)
	}

	// Notification failure must also roll back all rate changes.
	_, err = keys.sql.ExecContext(ctx, "DROP TABLE scheduler_outbox")
	require.NoError(t, err)
	require.Error(t, repo.UpdateWithRateSync(ctx, source, source.RateSyncGroupIDs))
	for _, id := range []int64{source.ID, target.ID} {
		got, err = repo.GetByIDLite(ctx, id)
		require.NoError(t, err)
		require.Equal(t, 0.5, got.RateMultiplier)
	}
}
