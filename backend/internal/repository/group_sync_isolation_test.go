//go:build unit

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestSyncIsolationPostgresLifecycle(t *testing.T) {
	dsn := os.Getenv("SYNC_ISOLATION_TEST_DSN")
	if dsn == "" {
		t.Skip("requires an isolated SYNC_ISOLATION_TEST_DSN")
	}
	ctx := context.Background()
	base, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	database := fmt.Sprintf("sync_isolation_test_%d", time.Now().UnixNano())
	_, err = base.Exec(`CREATE DATABASE ` + database)
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn+" dbname="+database)
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Close()
		_, _ = base.Exec(`DROP DATABASE ` + database)
		_ = base.Close()
	})
	require.NoError(t, ApplyMigrations(ctx, db))
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	repo := newGroupRepositoryWithSQL(client, db)
	source := &service.Group{Name: "Isolation test", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, RateMultiplier: 0.15, SyncIsolationEnabled: true, ForceOpenAIFast: true}
	require.NoError(t, createGroupRecord(ctx, client, source))
	linked := &service.Group{Name: "Existing rate target", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard}
	require.NoError(t, createGroupRecord(ctx, client, linked))
	source.RateSyncGroupIDs = []int64{linked.ID}
	require.NoError(t, repo.Update(ctx, source))
	_, err = db.Exec(`INSERT INTO users (id,email,password_hash) VALUES (2,'sync-isolation@test.invalid','fixture')`)
	require.NoError(t, err)
	user, err := client.User.Get(ctx, 2)
	require.NoError(t, err)
	keyRepo := newAPIKeyRepositoryWithSQL(client, db)
	keys := []*service.APIKey{
		{UserID: user.ID, GroupID: &source.ID, Key: "sk-sync-fixture-a", Name: "A", Status: service.StatusActive, Quota: 123},
		{UserID: user.ID, GroupID: &source.ID, Key: "sk-sync-fixture-b", Name: "B", Status: service.StatusActive, Quota: 456},
	}
	for _, key := range keys {
		require.NoError(t, keyRepo.Create(ctx, key))
	}
	_, err = db.Exec(`INSERT INTO user_group_rate_multipliers (user_id,group_id,rate_multiplier,rpm_override) VALUES (2,$1,0.12,42)`, source.ID)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO channels(name) VALUES ('sync test')`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO channel_groups(channel_id,group_id) SELECT id,$1 FROM channels WHERE name='sync test'`, source.ID)
	require.NoError(t, err)
	for range 10 {
		id, changed, err := repo.ObserveSyncIsolation(ctx, keys[0].ID, user.ID, source.ID, false, time.Now())
		require.NoError(t, err)
		require.False(t, changed)
		require.Equal(t, source.ID, id)
	}
	childID, changed, err := repo.ObserveSyncIsolation(ctx, keys[1].ID, user.ID, source.ID, false, time.Now())
	require.NoError(t, err)
	require.True(t, changed)
	require.NotEqual(t, source.ID, childID)
	child, err := repo.GetByIDLite(ctx, childID)
	require.NoError(t, err)
	require.True(t, child.IsExclusive)
	require.True(t, child.IsActive())
	require.True(t, child.ForceOpenAIFast)
	require.False(t, child.SyncIsolationEnabled, "children cannot recursively isolate")
	require.Equal(t, source.ID, child.SyncIsolationSourceGroupID)
	require.Equal(t, source.RateMultiplier, child.RateMultiplier)
	require.Equal(t, "Isolation test [同步]", child.Name)
	require.Empty(t, child.RateSyncGroupIDs, "children must not inherit the source's outgoing rate links")
	storedSource, err := repo.GetByIDLite(ctx, source.ID)
	require.NoError(t, err)
	require.Equal(t, []int64{linked.ID, childID}, storedSource.RateSyncGroupIDs)
	var candidate string
	require.NoError(t, db.QueryRow(`SELECT sync_isolation_group_name($1)`, source.Name).Scan(&candidate))
	require.Equal(t, "Isolation test [同步 2]", candidate, "collisions use an ordinal, not a group ID")
	require.NoError(t, db.QueryRow(`SELECT sync_isolation_group_name($1)`, strings.Repeat("长", 100)).Scan(&candidate))
	require.Len(t, []rune(candidate), 100)
	require.True(t, strings.HasSuffix(candidate, " [同步]"))
	for _, key := range keys {
		stored, err := keyRepo.GetByKeyForAuth(ctx, key.Key)
		require.NoError(t, err)
		require.Equal(t, childID, *stored.GroupID)
		require.Equal(t, key.Quota, stored.Quota)
		require.Equal(t, source.ID, stored.Group.SyncIsolationSourceGroupID)
		require.Contains(t, stored.User.AllowedGroups, childID)
	}
	var rate float64
	var rpm int
	require.NoError(t, db.QueryRow(`SELECT rate_multiplier,rpm_override FROM user_group_rate_multipliers WHERE user_id=2 AND group_id=$1`, childID).Scan(&rate, &rpm))
	require.Equal(t, 0.12, rate)
	require.Equal(t, 42, rpm)
	_, err = db.Exec(`UPDATE groups SET rate_multiplier=0.18, force_openai_fast=false WHERE id=$1`, source.ID)
	require.NoError(t, err)
	child, err = repo.GetByIDLite(ctx, childID)
	require.NoError(t, err)
	require.Equal(t, 0.18, child.RateMultiplier)
	require.False(t, child.ForceOpenAIFast)
	require.Empty(t, child.RateSyncGroupIDs, "source updates must not introduce child self-links")
	_, err = db.Exec(`UPDATE user_group_rate_multipliers SET rate_multiplier=0.13 WHERE user_id=2 AND group_id=$1`, source.ID)
	require.NoError(t, err)
	require.NoError(t, db.QueryRow(`SELECT rate_multiplier FROM user_group_rate_multipliers WHERE user_id=2 AND group_id=$1`, childID).Scan(&rate))
	require.Equal(t, 0.13, rate)
	for range 9 {
		id, _, err := repo.ObserveSyncIsolation(ctx, keys[0].ID, user.ID, childID, true, time.Now())
		require.NoError(t, err)
		require.Equal(t, childID, id)
	}
	id, changed, err := repo.ObserveSyncIsolation(ctx, keys[1].ID, user.ID, childID, true, time.Now())
	require.NoError(t, err)
	require.True(t, changed)
	require.Equal(t, source.ID, id)

	// Concurrent requests from both keys aggregate without duplicate children.
	var wg sync.WaitGroup
	errors := make(chan error, 12)
	for i := range 12 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, _, err := repo.ObserveSyncIsolation(ctx, keys[i%2].ID, user.ID, source.ID, false, time.Now())
			errors <- err
		}(i)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	count, err := client.Group.Query().Where(group.SyncIsolationSourceGroupIDEQ(source.ID)).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
	_, err = db.Exec(`UPDATE groups SET sync_isolation_enabled=false WHERE id=$1`, source.ID)
	require.NoError(t, err)
	for _, key := range keys {
		stored, err := keyRepo.GetByKeyForAuth(ctx, key.Key)
		require.NoError(t, err)
		require.Equal(t, source.ID, *stored.GroupID, "disabling restores every affected key immediately")
	}

	_, err = db.Exec(`UPDATE groups SET sync_isolation_enabled=true WHERE id=$1`, source.ID)
	require.NoError(t, err)
	arrived := time.Now()
	for range 10 {
		_, _, err := repo.ObserveSyncIsolation(ctx, keys[0].ID, user.ID, source.ID, false, arrived.Add(-61*time.Second))
		require.NoError(t, err)
	}
	id, changed, err = repo.ObserveSyncIsolation(ctx, keys[0].ID, user.ID, source.ID, false, arrived)
	require.NoError(t, err)
	require.False(t, changed, "delayed processing must not turn old requests into a fresh burst")
	require.Equal(t, source.ID, id)
	_, err = db.Exec(`DELETE FROM sync_isolation_windows WHERE group_id=$1`, source.ID)
	require.NoError(t, err)

	// Hold creation in the database while requests continue using their source group.
	lock, err := db.Conn(ctx)
	require.NoError(t, err)
	_, err = lock.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, -source.ID)
	require.NoError(t, err)
	svc := service.NewAPIKeyService(keyRepo, nil, repo, nil, nil, nil, nil)
	t.Cleanup(func() {
		_, _ = lock.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, -source.ID)
		_ = lock.Close()
		svc.StopSyncIsolation()
	})
	// Remove the existing child so this covers creation as well as key switching.
	_, err = db.Exec(`UPDATE groups SET deleted_at=NOW() WHERE id=$1`, childID)
	require.NoError(t, err)
	_, err = db.Exec(`UPDATE groups SET name=name||' old' WHERE id=$1`, childID)
	require.NoError(t, err)
	authKey, err := keyRepo.GetByKeyForAuth(ctx, keys[0].Key)
	require.NoError(t, err)
	for range 11 {
		svc.QueueSyncIsolation(authKey, false)
	}
	require.Eventually(t, func() bool {
		var waiting bool
		err := db.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_stat_activity WHERE datname=$1 AND wait_event='advisory')`, database).Scan(&waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "background creation must reach the held lock")
	queued := make(chan struct{})
	go func() {
		for range 20 {
			svc.QueueSyncIsolation(authKey, false)
		}
		close(queued)
	}()
	select {
	case <-queued:
	case <-time.After(time.Second):
		t.Fatal("requests waited for group creation")
	}
	stored, err := keyRepo.GetByKeyForAuth(ctx, keys[0].Key)
	require.NoError(t, err)
	require.Equal(t, source.ID, *stored.GroupID, "requests continue on the source group during creation")
	_, err = lock.ExecContext(ctx, `SELECT pg_advisory_unlock($1)`, -source.ID)
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		stored, err := keyRepo.GetByKeyForAuth(ctx, keys[0].Key)
		return err == nil && stored.Group.SyncIsolationSourceGroupID == source.ID
	}, 5*time.Second, 10*time.Millisecond)
	stored, err = keyRepo.GetByKeyForAuth(ctx, keys[0].Key)
	require.NoError(t, err)
	for range 210 {
		svc.QueueSyncIsolation(stored, true)
	}
	require.Eventually(t, func() bool {
		stored, err := keyRepo.GetByKeyForAuth(ctx, keys[1].Key)
		return err == nil && *stored.GroupID == source.ID
	}, 5*time.Second, 10*time.Millisecond, "streaming recovery switches all keys in the background")
	_, err = db.Exec(`UPDATE groups SET is_exclusive=true WHERE id=$1`, source.ID)
	require.NoError(t, err)
	for range 11 {
		id, changed, err := repo.ObserveSyncIsolation(ctx, keys[0].ID, user.ID, source.ID, false, time.Now())
		require.NoError(t, err)
		require.False(t, changed, "queued work must not restore revoked source access")
		require.Equal(t, source.ID, id)
	}

	t.Run("repair existing isolation groups", func(t *testing.T) {
		legacySource := &service.Group{Name: "Legacy source", Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard}
		require.NoError(t, createGroupRecord(ctx, client, legacySource))
		legacy := &service.Group{Name: fmt.Sprintf("Legacy source [同步-%d]", legacySource.ID), Platform: service.PlatformOpenAI, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, SyncIsolationSourceGroupID: legacySource.ID}
		require.NoError(t, createGroupRecord(ctx, client, legacy))
		legacy.RateSyncGroupIDs = []int64{legacy.ID, linked.ID}
		require.NoError(t, repo.Update(ctx, legacy))
		legacySource.RateSyncGroupIDs = []int64{linked.ID}
		require.NoError(t, repo.Update(ctx, legacySource))
		sql, err := migrations.FS.ReadFile("273_sync_isolation_rate_links.sql")
		require.NoError(t, err)
		backfill := string(sql)[strings.Index(string(sql), "DO $$"):]
		for range 2 {
			_, err = db.Exec(backfill)
			require.NoError(t, err)
		}
		got, err := repo.GetByIDLite(ctx, legacy.ID)
		require.NoError(t, err)
		require.Equal(t, "Legacy source [同步]", got.Name)
		require.Equal(t, []int64{linked.ID}, got.RateSyncGroupIDs)
		got, err = repo.GetByIDLite(ctx, legacySource.ID)
		require.NoError(t, err)
		require.Equal(t, []int64{linked.ID, legacy.ID}, got.RateSyncGroupIDs)
	})
}
