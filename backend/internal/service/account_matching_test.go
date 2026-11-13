package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type matchingLookupStub struct {
	AccountMatchingRepository
	ids             []int64
	err             error
	userID, groupID int64
}

func (r *matchingLookupStub) AllowedAccounts(_ context.Context, userID, groupID int64) ([]int64, error) {
	r.userID, r.groupID = userID, groupID
	return r.ids, r.err
}

func TestAccountMatchingRequestScopeAndLookupFailure(t *testing.T) {
	repo := &matchingLookupStub{ids: []int64{22, 23}}
	svc := &APIKeyService{accountMatching: repo}
	groupID := int64(9)
	ctx, err := svc.WithAccountMatching(context.Background(), 2, &groupID)
	require.NoError(t, err)
	require.Equal(t, int64(2), repo.userID)
	require.Equal(t, groupID, repo.groupID)
	require.True(t, AccountMatchingAllows(ctx, 22))
	require.True(t, AccountMatchingAllows(ctx, 23))
	require.False(t, AccountMatchingAllows(ctx, 24))
	require.True(t, AccountMatchingAllows(context.Background(), 24), "must not affect other requests")
	repo.ids = nil
	ctx, err = svc.WithAccountMatching(context.Background(), 3, &groupID)
	require.NoError(t, err)
	require.True(t, AccountMatchingAllows(ctx, 24), "unmatched user/group keeps normal scheduling")
	repo.err = errors.New("database unavailable")
	_, err = svc.WithAccountMatching(context.Background(), 2, &groupID)
	require.Error(t, err, "auth must abort, not silently allow all accounts")
}

func TestAccountMatchingEmptyRuleAndSharedCandidates(t *testing.T) {
	ctx := withAccountMatching(context.Background(), []int64{})
	require.False(t, AccountMatchingAllows(ctx, 1))
	accounts := []Account{{ID: 1}, {ID: 2}, {ID: 3}}
	filtered := filterAccountMatching(withAccountMatching(context.Background(), []int64{2}), accounts)
	require.Equal(t, []Account{{ID: 2}}, filtered)
	require.Equal(t, int64(1), accounts[0].ID, "must not mutate shared scheduler snapshots")
}

func TestAccountMatchingOpenAIStickyAndExhaustion(t *testing.T) {
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 1},
		{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: 2},
	}}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: &stubGatewayCache{sessionBindings: map[string]int64{"openai:sticky": 1}}}
	ctx := withAccountMatching(context.Background(), []int64{2})
	account, err := svc.SelectAccountForModelWithExclusions(ctx, nil, "sticky", "gpt-4", nil)
	require.NoError(t, err)
	require.Equal(t, int64(2), account.ID, "disallowed sticky must not override the rule")
	account, err = svc.SelectAccountForModelWithExclusions(ctx, nil, "sticky", "gpt-4", map[int64]struct{}{2: {}})
	require.Error(t, err)
	require.Nil(t, account, "failover must not escape to account 1")
}

func TestAccountMatchingSelectionReleasesDeniedSlot(t *testing.T) {
	ctx := withAccountMatching(context.Background(), []int64{2})
	for _, selectResult := range []func(context.Context, *Account, bool, func(), *AccountWaitPlan) (*AccountSelectionResult, error){
		(&GatewayService{}).newSelectionResult, (&OpenAIGatewayService{}).newSelectionResult,
	} {
		released := false
		result, err := selectResult(ctx, &Account{ID: 1}, true, func() { released = true }, nil)
		require.ErrorIs(t, err, ErrNoAvailableAccounts)
		require.Nil(t, result)
		require.True(t, released)
	}
}
