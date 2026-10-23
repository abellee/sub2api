package service

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type safetyBillingFake struct {
	mu       sync.Mutex
	commands map[string]UsageBillingCommand
	attempts int
	fail     bool
	loseAck  bool
	balance  float64
}

func (b *safetyBillingFake) Apply(_ context.Context, cmd *UsageBillingCommand) (*UsageBillingApplyResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.attempts++
	if b.fail {
		return nil, errors.New("database offline")
	}
	cmd.Normalize()
	if _, exists := b.commands[cmd.RequestID]; exists {
		return &UsageBillingApplyResult{Applied: false}, nil
	}
	if b.commands == nil {
		b.commands = map[string]UsageBillingCommand{}
	}
	b.commands[cmd.RequestID] = *cmd
	b.balance -= cmd.BalanceCost
	if b.loseAck {
		b.loseAck = false
		return nil, errors.New("commit acknowledgement lost")
	}
	return &UsageBillingApplyResult{Applied: true}, nil
}

func safetyTestConfig() ContentSafetyBillingConfig {
	return ContentSafetyBillingConfig{Enabled: true, Amount: 0.125, MatchDescription: "Failed check: SAFETY_CHECK_TYPE", ClientMessage: "请求未通过内容安全检查"}
}

func newSafetyTestService(t *testing.T, billing *safetyBillingFake) *ContentSafetyBillingService {
	t.Helper()
	s, err := OpenContentSafetyBilling(filepath.Join(t.TempDir(), "safety.sqlite"), billing, nil)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, s.Close()) })
	require.NoError(t, s.UpdateConfig(context.Background(), safetyTestConfig()))
	return s
}

func TestContentSafetyBillingMatchAndValidation(t *testing.T) {
	s := newSafetyTestService(t, &safetyBillingFake{})
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"structured", 403, `{"error":{"message":"I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE"}}`, true},
		{"escaped", 403, `{"error":{"message":"Failed check: SAFETY_CHECK_\u0054YPE"}}`, true},
		{"plain text", 403, "Failed check: SAFETY_CHECK_TYPE", true},
		{"native xai", 403, `{"code":"PermissionDenied","error":"I'm sorry, I can't help with that request. Failed check: SAFETY_CHECK_TYPE"}`, true},
		{"json string", 403, `"Failed check: SAFETY_CHECK_TYPE"`, true},
		{"different status", 400, `{"error":{"message":"Failed check: SAFETY_CHECK_TYPE"}}`, false},
		{"successful", 200, `{"message":"Failed check: SAFETY_CHECK_TYPE"}`, false},
		{"access denied", 403, `{"error":{"message":"permission denied"}}`, false},
		{"unrelated metadata", 403, `{"error":{"message":"permission denied"},"metadata":"Failed check: SAFETY_CHECK_TYPE"}`, false},
		{"case sensitive", 403, `{"message":"failed check: safety_check_type"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config, err := s.Match(ctx, tc.status, []byte(tc.body))
			require.NoError(t, err)
			require.Equal(t, tc.want, config != nil)
		})
	}
	c := safetyTestConfig()
	c.Enabled = false
	require.NoError(t, s.UpdateConfig(ctx, c))
	config, err := s.Match(ctx, 403, []byte(c.MatchDescription))
	require.NoError(t, err)
	require.Nil(t, config)
	for _, amount := range []float64{-1, 0, math.NaN(), math.Inf(1), 1000001, 0.000000001} {
		c = safetyTestConfig()
		c.Amount = amount
		require.Error(t, c.Validate())
	}
	c = safetyTestConfig()
	c.MatchDescription = "  "
	require.Error(t, c.Validate())
	c = safetyTestConfig()
	c.ClientMessage = ""
	require.Error(t, c.Validate())
	c = safetyTestConfig()
	c.Amount = 0.00000001
	require.NoError(t, c.Validate())
}

func TestContentSafetyBillingConcurrentDebitOnceAndPagination(t *testing.T) {
	b := &safetyBillingFake{balance: 1}
	s := newSafetyTestService(t, b)
	ctx := context.Background()
	d := ContentSafetyDebit{ID: "one-request", UserID: 11, APIKeyID: 22, AccountID: 33, UpstreamRequestID: "xai-123"}
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); errs <- s.Debit(ctx, d, safetyTestConfig()) }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Len(t, b.commands, 1)
	require.Equal(t, 0.875, b.balance)
	page, err := s.Records(ctx, 1, 20)
	require.NoError(t, err)
	require.Equal(t, 1, page.Total)
	require.Equal(t, "charged", page.Items[0].Status)
	require.Equal(t, safetyTestConfig().MatchDescription, page.Items[0].Notes)
	require.Equal(t, "xai-123", page.Items[0].UpstreamRequestID)
	require.NotEmpty(t, page.Items[0].ChargedAt)
	d.ID = "second-request"
	require.NoError(t, s.Debit(ctx, d, safetyTestConfig()))
	page, err = s.Records(ctx, 2, 1)
	require.NoError(t, err)
	require.Equal(t, 2, page.Total)
	require.Len(t, page.Items, 1)
	for _, cmd := range b.commands {
		require.Zero(t, cmd.SubscriptionCost)
		require.Zero(t, cmd.AccountQuotaCost)
		require.Zero(t, cmd.APIKeyQuotaCost)
	}
}

func TestContentSafetyBillingRecoveryAfterCommitAndRestart(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "safety.sqlite")
	b := &safetyBillingFake{balance: 1, loseAck: true}
	s, err := OpenContentSafetyBilling(path, b, nil)
	require.NoError(t, err)
	c := safetyTestConfig()
	require.NoError(t, s.UpdateConfig(ctx, c))
	d := ContentSafetyDebit{ID: "stable-money-event", UserID: 11, APIKeyID: 22, AccountID: 33}
	require.Error(t, s.Debit(ctx, d, c))
	require.Equal(t, 0.875, b.balance)
	page, err := s.Records(ctx, 1, 20)
	require.NoError(t, err)
	require.Equal(t, "pending", page.Items[0].Status)
	require.NoError(t, s.Close())
	s, err = OpenContentSafetyBilling(path, b, nil)
	require.NoError(t, err)
	defer s.Close()
	persisted, err := s.Config(ctx)
	require.NoError(t, err)
	require.Equal(t, c, persisted)
	// Config edits/disable cannot change the already journaled money event.
	c.Enabled = false
	c.Amount = 2
	require.NoError(t, s.UpdateConfig(ctx, c))
	require.NoError(t, s.Recover(ctx))
	require.NoError(t, s.Recover(ctx))
	require.Len(t, b.commands, 1)
	require.Equal(t, 0.875, b.balance)
	require.Equal(t, 2, b.attempts)
	page, err = s.Records(ctx, 1, 20)
	require.NoError(t, err)
	require.Equal(t, "charged", page.Items[0].Status)
	require.Equal(t, 0.125, page.Items[0].Amount)
	require.Empty(t, page.Items[0].LastError)
}

func TestContentSafetyBillingRecoveryOnBalanceFailureAndCacheFailure(t *testing.T) {
	ctx := context.Background()
	b := &safetyBillingFake{balance: 0.01, fail: true}
	s := newSafetyTestService(t, b)
	d := ContentSafetyDebit{ID: "retry", UserID: 1, APIKeyID: 2, AccountID: 3}
	require.Error(t, s.Debit(ctx, d, safetyTestConfig()))
	require.Equal(t, 0.01, b.balance)
	b.fail = false
	s.invalidate = func(context.Context, int64) error { return errors.New("cache offline") }
	require.NoError(t, s.Recover(ctx))
	page, err := s.Records(ctx, 1, 20)
	require.NoError(t, err)
	require.Equal(t, "pending", page.Items[0].Status)
	require.InDelta(t, -0.115, b.balance, 1e-8)
	invalidated := int64(0)
	s.invalidate = func(_ context.Context, id int64) error { invalidated = id; return nil }
	require.NoError(t, s.Recover(ctx))
	require.Equal(t, int64(1), invalidated)
	require.Len(t, b.commands, 1)
	require.InDelta(t, -0.115, b.balance, 1e-8)
}

func TestContentSafetyBillingJournalFailureDoesNotDebit(t *testing.T) {
	b := &safetyBillingFake{balance: 1}
	s := newSafetyTestService(t, b)
	_, err := s.db.Exec(`DROP TABLE content_safety_debits`)
	require.NoError(t, err)
	require.Error(t, s.Debit(context.Background(), ContentSafetyDebit{ID: "no-journal", UserID: 1, APIKeyID: 2, AccountID: 3}, safetyTestConfig()))
	require.Zero(t, b.attempts)
	require.Equal(t, float64(1), b.balance)
}
