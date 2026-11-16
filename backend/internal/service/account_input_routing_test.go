package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type inputRoutingSettingsRepo struct {
	SettingRepository
	raw string
	err error
}

func (r *inputRoutingSettingsRepo) GetValue(context.Context, string) (string, error) {
	return r.raw, r.err
}
func (r *inputRoutingSettingsRepo) Set(_ context.Context, _ string, raw string) error {
	if r.err != nil {
		return r.err
	}
	r.raw = raw
	return nil
}

type inputRoutingStoreStub struct {
	restricted              bool
	err                     error
	reads, writes           int
	userID, groupID, keyID  int64
	input, threshold        int
	output, outputThreshold int
}

func (s *inputRoutingStoreStub) Restricted(context.Context, string, int64, int64) (bool, error) {
	s.reads++
	return s.restricted, s.err
}
func (s *inputRoutingStoreStub) Observe(_ context.Context, _ string, userID, groupID, keyID int64, _ string, input, output, threshold, outputThreshold int) error {
	s.writes++
	s.userID, s.groupID, s.keyID = userID, groupID, keyID
	s.input, s.threshold = input, threshold
	s.output, s.outputThreshold = output, outputThreshold
	return s.err
}
func newInputRoutingTestService(t *testing.T) (*AccountInputRoutingService, *inputRoutingStoreStub, *inputRoutingSettingsRepo) {
	t.Helper()
	cfg := AccountInputRoutingSettings{Enabled: true, Threshold: 100000, OutputThreshold: 10000, GroupIDs: []int64{10}, AccountIDs: []int64{20, 21}, Revision: "v1"}
	raw, err := json.Marshal(cfg)
	require.NoError(t, err)
	repo := &inputRoutingSettingsRepo{raw: string(raw)}
	store := &inputRoutingStoreStub{restricted: true}
	return NewAccountInputRoutingService(repo, store, nil, nil), store, repo
}

func TestAccountInputRoutingPriorityRefreshAndFailures(t *testing.T) {
	s, store, _ := newInputRoutingTestService(t)
	manual := &matchingLookupStub{}
	api := &APIKeyService{accountMatching: manual, inputRouting: s}
	groupID := int64(10)
	ctx, err := api.WithAccountMatching(context.Background(), 2, &groupID)
	require.NoError(t, err)
	require.True(t, AccountMatchingAllows(ctx, 20))
	require.True(t, AccountMatchingAllows(ctx, 21))
	require.False(t, AccountMatchingAllows(ctx, 22))
	manual.ids = []int64{22}
	store.err = errors.New("redis unavailable")
	ctx, err = api.WithAccountMatching(ctx, 2, &groupID)
	require.NoError(t, err)
	require.True(t, AccountMatchingAllows(ctx, 22))
	require.False(t, AccountMatchingAllows(ctx, 20))
	require.Equal(t, 1, store.reads, "manual matching must bypass automatic restrictions, even when Redis is down")
	manual.ids = nil
	_, err = api.WithAccountMatching(ctx, 2, &groupID)
	require.Error(t, err, "do not escape a restriction when Redis is down")
	store.err = nil
	store.restricted = false
	ctx, err = api.WithAccountMatching(ctx, 2, &groupID)
	require.NoError(t, err)
	require.True(t, AccountMatchingAllows(ctx, 22), "refresh must clear stale restrictions")
	other := int64(11)
	_, err = api.WithAccountMatching(ctx, 2, &other)
	require.NoError(t, err)
	require.Equal(t, 3, store.reads, "unselected groups do not read state")
}

func TestAccountInputRoutingObservesOrdinaryInputAndOriginalGroup(t *testing.T) {
	s, store, _ := newInputRoutingTestService(t)
	ctx, err := s.apply(context.Background(), 2, 10, nil)
	require.NoError(t, err)
	ctx = CopyAccountInputRoutingContext(ctx, context.Background())
	mode := string(BillingModeToken)
	fallbackGroup := int64(99)
	usage := &UsageLog{UserID: 2, APIKeyID: 7, GroupID: &fallbackGroup, RequestID: "req", InputTokens: 12, OutputTokens: 5, CacheReadTokens: 900000, CacheCreationTokens: 800000, BillingMode: &mode}
	observeAccountInputRouting(ctx, usage)
	require.Equal(t, 1, store.writes)
	require.Equal(t, 12, store.input)
	require.Equal(t, 5, store.output)
	require.Equal(t, int64(10), store.groupID)
	require.Equal(t, 100000, store.threshold)
	require.Equal(t, 10000, store.outputThreshold)
	usage.InputTokens = 0
	usage.OutputTokens = 0
	observeAccountInputRouting(ctx, usage)
	require.Equal(t, 2, store.writes, "reported cache-only input counts as ordinary input zero")
	usage.CacheReadTokens = 0
	usage.CacheCreationTokens = 0
	observeAccountInputRouting(ctx, usage)
	require.Equal(t, 2, store.writes, "missing usage must not unlock")
	usage.InputTokens = 200000
	usage.ImageCount = 1
	observeAccountInputRouting(ctx, usage)
	require.Equal(t, 2, store.writes)
	usage.ImageCount = 0
	usage.OutputTokens = -1
	observeAccountInputRouting(ctx, usage)
	require.Equal(t, 2, store.writes, "negative output is invalid usage")
	usage.OutputTokens = 5
	usage.UserID = 3
	observeAccountInputRouting(ctx, usage)
	require.Equal(t, 2, store.writes)
}

func TestAccountInputRoutingUsageWriteSurvivesCancellation(t *testing.T) {
	s, store, _ := newInputRoutingTestService(t)
	ctx, err := s.apply(context.Background(), 2, 10, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(ctx)
	cancel()
	repo := &openAIRecordUsageLogRepoStub{}
	writeUsageLogBestEffort(ctx, repo, &UsageLog{UserID: 2, APIKeyID: 7, RequestID: "completed", InputTokens: 100001}, "test")
	require.Equal(t, 1, store.writes)
	require.Equal(t, 100001, store.input)
	require.Equal(t, 1, repo.calls)
	require.NoError(t, repo.lastCtxErr)
}

type inputRoutingGroups struct{ GroupRepository }

func (*inputRoutingGroups) GetByID(_ context.Context, id int64) (*Group, error) {
	return &Group{ID: id}, nil
}

type inputRoutingAccounts struct{ AccountRepository }

func (*inputRoutingAccounts) GetByIDs(_ context.Context, ids []int64) ([]*Account, error) {
	out := make([]*Account, 0, len(ids))
	for _, id := range ids {
		out = append(out, &Account{ID: id})
	}
	return out, nil
}

func TestAccountInputRoutingSettingsValidationAndDisable(t *testing.T) {
	repo := &inputRoutingSettingsRepo{err: ErrSettingNotFound}
	s := NewAccountInputRoutingService(repo, nil, &inputRoutingGroups{}, &inputRoutingAccounts{})
	cfg, err := s.Settings(context.Background())
	require.NoError(t, err)
	require.False(t, cfg.Enabled)
	require.Equal(t, 100000, cfg.Threshold)
	require.Equal(t, 10000, cfg.OutputThreshold)
	repo.err = nil
	cfg.Enabled = true
	cfg.GroupIDs = []int64{10}
	cfg.AccountIDs = []int64{20, 21}
	for _, ids := range [][]int64{{20}, {20, 20}, {0, 21}, {20, 21, 22}} {
		bad := cfg
		bad.AccountIDs = ids
		_, err = s.SaveSettings(context.Background(), bad)
		require.ErrorIs(t, err, ErrAccountInputRoutingInvalid)
	}
	bad := cfg
	bad.Threshold = 0
	_, err = s.SaveSettings(context.Background(), bad)
	require.ErrorIs(t, err, ErrAccountInputRoutingInvalid)
	for _, threshold := range []int{-1, 0, 100000001} {
		bad := cfg
		bad.OutputThreshold = threshold
		_, err = s.SaveSettings(context.Background(), bad)
		require.ErrorIs(t, err, ErrAccountInputRoutingInvalid)
	}
	cfg.OutputThreshold = 25000
	saved, err := s.SaveSettings(context.Background(), cfg)
	require.NoError(t, err)
	require.NotEmpty(t, saved.Revision)
	saved.AccountIDs[0] = 999
	fresh, err := s.Settings(context.Background())
	require.NoError(t, err)
	require.Equal(t, int64(20), fresh.AccountIDs[0], "callers must not mutate cached settings")
	reloaded := NewAccountInputRoutingService(repo, nil, nil, nil)
	persisted, err := reloaded.Settings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 25000, persisted.OutputThreshold)
	store := &inputRoutingStoreStub{restricted: true}
	s.store = store
	ctx, err := s.apply(context.Background(), 2, 10, nil)
	require.NoError(t, err)
	fresh.Enabled = false
	s.groups = nil
	s.accounts = nil
	_, err = s.SaveSettings(context.Background(), fresh)
	require.NoError(t, err, "turning off must not depend on old resources")
	observeAccountInputRouting(ctx, &UsageLog{UserID: 2, APIKeyID: 7, RequestID: "late", InputTokens: 200000})
	require.Zero(t, store.writes, "in-flight requests from a previous configuration must not recreate counters")
	ctx, err = s.apply(context.Background(), 2, 10, nil)
	require.NoError(t, err)
	require.True(t, AccountMatchingAllows(ctx, 99))
	require.Equal(t, 1, store.reads)
}

func TestAccountInputRoutingLoadsLegacySettingsWithOutputDefault(t *testing.T) {
	repo := &inputRoutingSettingsRepo{raw: `{"enabled":true,"threshold":120000,"group_ids":[10],"account_ids":[20,21],"revision":"old"}`}
	s := NewAccountInputRoutingService(repo, nil, nil, nil)
	cfg, err := s.Settings(context.Background())
	require.NoError(t, err)
	require.Equal(t, 120000, cfg.Threshold)
	require.Equal(t, 10000, cfg.OutputThreshold)
	for _, value := range []int{0, -1, 100000001} {
		cfg.OutputThreshold = value
		raw, err := json.Marshal(cfg)
		require.NoError(t, err)
		repo.raw = string(raw)
		s = NewAccountInputRoutingService(repo, nil, nil, nil)
		_, err = s.Settings(context.Background())
		require.Error(t, err)
	}
}
