//go:build unit

package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/pkg/querycache"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestPublicTransitCachePreservesIDsAndChecksEnabled(t *testing.T) {
	repo := &settingRepoStub{values: map[string]string{SettingKeyPublicTransitEnabled: "true"}}
	s := &PublicTransitService{settingService: NewSettingService(repo, nil)}
	ctx := context.Background()
	key := s.snapshotCacheKey(ctx, "v1", "https://example.com", "")
	_, err := querycache.Load(ctx, &s.snapshotCache, key, time.Minute, func(context.Context) (cachedPublicTransitV1, error) {
		return cachedPublicTransitV1{Snapshot: &PublicTransitSnapshot{Groups: []PublicTransitGroup{{Name: "public"}}}, GroupIDs: []int64{7}}, nil
	})
	require.NoError(t, err)
	first, err := s.Snapshot(ctx, "https://example.com")
	require.NoError(t, err)
	require.EqualValues(t, 7, first.Groups[0].ID)
	first.Groups[0].Name = "changed"
	second, err := s.Snapshot(ctx, "https://example.com")
	require.NoError(t, err)
	require.Equal(t, "public", second.Groups[0].Name)
	require.NotEqual(t, key, s.snapshotCacheKey(ctx, "v1", "https://other.example", ""))
	require.NotEqual(t, s.snapshotCacheKey(ctx, "v2", "https://example.com", "24h"), s.snapshotCacheKey(ctx, "v2", "https://example.com", "7d"))
	repo.mu.Lock()
	repo.values[SettingKeyPublicTransitEnabled] = "false"
	repo.mu.Unlock()
	_, err = s.Snapshot(ctx, "https://example.com")
	require.Error(t, err)
	_, err = s.SnapshotV2Range(ctx, "https://example.com", "24h")
	require.Error(t, err)
}
