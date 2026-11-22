//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type rateSyncRepoStub struct {
	groupRepoStubForAdmin
	targets []int64
	syncErr error
}

func (r *rateSyncRepoStub) UpdateWithRateSync(_ context.Context, g *Group, ids []int64) error {
	r.targets = append([]int64{}, ids...)
	if r.syncErr != nil {
		return r.syncErr
	}
	r.updated = g
	return nil
}

func rateSyncTestService() (*adminServiceImpl, *rateSyncRepoStub, *authCacheInvalidatorStub) {
	repo := &rateSyncRepoStub{groupRepoStubForAdmin: groupRepoStubForAdmin{getByIDByID: map[int64]*Group{
		1: {ID: 1, Name: "source", Platform: PlatformOpenAI, SubscriptionType: SubscriptionTypeStandard, RateMultiplier: 1, RateSyncGroupIDs: []int64{2}},
		2: {ID: 2, Name: "target", Platform: PlatformOpenAI, RateMultiplier: 2, RateSyncGroupIDs: []int64{3}},
		3: {ID: 3, Name: "other", Platform: PlatformGemini, RateMultiplier: 3},
	}}}
	cache := &authCacheInvalidatorStub{}
	return &adminServiceImpl{groupRepo: repo, groupRateSyncRepo: repo, authCacheInvalidator: cache}, repo, cache
}

func TestGroupRateSyncOnlyWhenRateChanges(t *testing.T) {
	for _, tc := range []struct {
		name    string
		rate    *float64
		links   *[]int64
		targets []int64
	}{
		{name: "rate omitted"},
		{name: "same rate", rate: rateSyncFloat(1)},
		{name: "existing links", rate: rateSyncFloat(0.5), targets: []int64{2}},
		{name: "new links and rate", rate: rateSyncFloat(0.5), links: &[]int64{3, 2, 3}, targets: []int64{3, 2}},
		{name: "clear links", rate: rateSyncFloat(0.5), links: &[]int64{}},
		{name: "selection only", links: &[]int64{3}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, repo, cache := rateSyncTestService()
			result, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{RateMultiplier: tc.rate, RateSyncGroupIDs: tc.links})
			require.NoError(t, err)
			require.Equal(t, tc.targets, repo.targets)
			require.Equal(t, append([]int64{1}, tc.targets...), cache.groupIDs)
			if tc.links != nil && len(*tc.links) == 0 {
				require.Empty(t, result.RateSyncGroupIDs)
			}
			require.Equal(t, 2.0, repo.getByIDByID[2].RateMultiplier, "service must not rewrite entire target objects")
		})
	}
}

func TestGroupRateSyncRejectsBadTargetsBeforeWriting(t *testing.T) {
	for _, ids := range [][]int64{{1}, {0}, {-1}, {999}} {
		svc, repo, cache := rateSyncTestService()
		_, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{RateMultiplier: rateSyncFloat(0.5), RateSyncGroupIDs: &ids})
		require.Error(t, err)
		require.Nil(t, repo.updated)
		require.Empty(t, repo.targets)
		require.Empty(t, cache.groupIDs)
	}
	svc, repo, cache := rateSyncTestService()
	repo.syncErr = errors.New("write failed")
	_, err := svc.UpdateGroup(context.Background(), 1, &UpdateGroupInput{RateMultiplier: rateSyncFloat(0.5)})
	require.ErrorContains(t, err, "write failed")
	require.Nil(t, repo.updated)
	require.Empty(t, cache.groupIDs)
}

func TestGroupRateSyncCreateStoresLinksWithoutChangingRates(t *testing.T) {
	svc, repo, _ := rateSyncTestService()
	g, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "new", Platform: PlatformOpenAI, RateMultiplier: 0.5, RateSyncGroupIDs: []int64{2, 3, 2}})
	require.NoError(t, err)
	require.Equal(t, []int64{2, 3}, g.RateSyncGroupIDs)
	require.Empty(t, repo.targets)
}

func rateSyncFloat(v float64) *float64 { return &v }
