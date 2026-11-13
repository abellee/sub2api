package service

import (
	"context"
	"errors"
	"testing"
	"time"

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

// Reproduces user 2 / group 27 selecting account 233, whose normal group is 55.
func TestAccountMatchingCrossGroupPoolAndSticky(t *testing.T) {
	groupID := int64(27)
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{ID: 233, GroupIDs: []int64{55}, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6-astra": "gpt-6-astra"}}},
		{ID: 234, GroupIDs: []int64{27}, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, Concurrency: 1},
	}}
	ctx := withAccountMatching(context.Background(), []int64{233})
	cache := &stubGatewayCache{sessionBindings: map[string]int64{"openai:sticky": 233}}
	svc := &OpenAIGatewayService{accountRepo: repo, cache: cache}
	for _, sticky := range []string{"", "sticky"} {
		account, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, sticky, "gpt-6-astra", nil)
		require.NoError(t, err)
		require.Equal(t, int64(233), account.ID)
	}
	snapshot := &SchedulerSnapshotService{accountRepo: repo}
	svc.schedulerSnapshot = snapshot
	accounts, _, err := snapshot.ListSchedulableAccounts(ctx, &groupID, PlatformOpenAI, false)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, int64(233), accounts[0].ID)
	require.Equal(t, []int64{55}, accounts[0].GroupIDs, "rule must not rewrite global membership")
	require.False(t, svc.openAIAccountMatchesSchedulingGroup(context.Background(), &repo.accounts[0], &groupID))
	require.True(t, svc.openAIAccountMatchesSchedulingGroup(ctx, &repo.accounts[0], &groupID))
	require.False(t, svc.openAIAccountMatchesSchedulingGroup(ctx, &repo.accounts[1], &groupID))
	scheduler := &defaultOpenAIAccountScheduler{service: svc, stats: newOpenAIAccountRuntimeStats()}
	selected, _, err := scheduler.Select(ctx, OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra", RequiredTransport: OpenAIUpstreamTransportAny})
	require.NoError(t, err)
	require.Equal(t, int64(233), selected.Account.ID)
	if selected.ReleaseFunc != nil {
		selected.ReleaseFunc()
	}
	selected, _, err = scheduler.Select(ctx, OpenAIAccountScheduleRequest{GroupID: &groupID, Platform: PlatformOpenAI, RequestedModel: "gpt-6-astra", RequiredTransport: OpenAIUpstreamTransportAny, ExcludedIDs: map[int64]struct{}{233: {}}})
	require.Error(t, err)
	require.Nil(t, selected, "must never escape to the group's ordinary accounts")
	_, err = svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "unsupported-model", nil)
	require.Error(t, err)
}

func TestAccountMatchingPoolPreservesAvailabilityAndPlatform(t *testing.T) {
	until := time.Now().Add(time.Hour)
	repo := stubOpenAIAccountRepo{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true},
		{ID: 2, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: false},
		{ID: 3, Platform: PlatformGemini, Status: StatusActive, Schedulable: true},
		{ID: 4, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, TempUnschedulableUntil: &until},
	}}
	ctx := withAccountMatching(context.Background(), []int64{1, 2, 3, 4, 999})
	accounts, matched, err := listAccountMatchingAccounts(ctx, repo, PlatformOpenAI, false)
	require.NoError(t, err)
	require.True(t, matched)
	require.Len(t, accounts, 1)
	require.Equal(t, int64(1), accounts[0].ID)
	gemini := &GeminiMessagesCompatService{accountRepo: repo}
	accounts, err = gemini.listSchedulableAccountsOnce(ctx, nil, PlatformGemini, false)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, int64(3), accounts[0].ID)
	batch := &BatchImagePublicService{AccountRepo: repo}
	accounts, err = batch.listCandidateAccounts(ctx, nil, PlatformGemini)
	require.NoError(t, err)
	require.Len(t, accounts, 1)
	require.Equal(t, int64(3), accounts[0].ID)
	accounts, matched, err = listAccountMatchingAccounts(context.Background(), nil, PlatformOpenAI, false)
	require.NoError(t, err)
	require.False(t, matched)
	require.Nil(t, accounts)
	accounts, matched, err = listAccountMatchingAccounts(withAccountMatching(context.Background(), []int64{}), nil, PlatformOpenAI, false)
	require.NoError(t, err)
	require.True(t, matched)
	require.Empty(t, accounts)
}
