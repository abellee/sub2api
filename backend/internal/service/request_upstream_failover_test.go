package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func upstreamFailoverAccounts(upstreams ...int) []Account {
	accounts := make([]Account, len(upstreams))
	for i, upstream := range upstreams {
		accounts[i] = Account{
			ID: int64(i + 1), Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
			Status: StatusActive, Schedulable: true, Concurrency: 1, Priority: i,
			GroupIDs:    []int64{1},
			Credentials: map[string]any{"base_url": fmt.Sprintf("https://upstream-%d.example/v1", upstream)},
		}
	}
	return accounts
}

func upstreamFailoverFirstAvailable(accounts []Account) func(context.Context, map[int64]struct{}) (*AccountSelectionResult, error) {
	return func(ctx context.Context, excluded map[int64]struct{}) (*AccountSelectionResult, error) {
		for i := range accounts {
			if _, skip := excluded[accounts[i].ID]; !skip {
				return &AccountSelectionResult{Account: &accounts[i]}, nil
			}
		}
		return nil, ErrNoAvailableAccounts
	}
}

func TestUpstreamFailoverRounds(t *testing.T) {
	for _, tt := range []struct {
		name string
		pool []int
		want []int
	}{
		{"user example", []int{1, 1, 2, 2, 2, 2, 3, 3}, []int{1, 2, 3, 1, 2, 3, 2}},
		{"one upstream", []int{1, 1, 1, 1}, []int{1, 1}},
		{"distinct upstreams", []int{1, 2, 3, 4}, []int{1, 2, 3, 4}},
		{"equal pools", []int{1, 1, 2, 2, 3, 3}, []int{1, 2, 3, 1, 2, 3}},
		{"one remaining after first round", []int{1, 2, 2, 2}, []int{1, 2, 2}},
		{"several rounds", []int{1, 1, 1, 2, 2, 2, 2, 2, 3, 3, 3}, []int{1, 2, 3, 1, 2, 3, 1, 2, 3, 2}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			accounts := upstreamFailoverAccounts(tt.pool...)
			ctx := withRequestAccountFailoverAccounts(context.Background(), accounts)
			excluded := map[int64]struct{}{}
			var got []int
			for i := 0; i <= len(accounts); i++ {
				selection, err := selectAccountByFailoverUpstream(ctx, excluded, upstreamFailoverFirstAvailable(accounts))
				if err != nil {
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					break
				}
				id := selection.Account.ID
				require.NotContains(t, excluded, id)
				got = append(got, tt.pool[id-1])
				attemptCtx := context.WithValue(ctx, ctxkey.AccountID, id)
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(attemptCtx)
				original := &UpstreamFailoverError{StatusCode: 502, RetryableOnSameAccount: true}
				_, err = ForwardWithAccountRetries(attemptCtx, c, func() (*ForwardResult, error) { return nil, original })
				var failure *UpstreamFailoverError
				require.ErrorAs(t, err, &failure)
				require.Equal(t, !requestAccountFailoverFinalAttempt(ctx, id), failure.ShouldRetryNextAccount())
				require.True(t, original.ShouldRetryNextAccount(), "do not mutate a shared upstream error")
				excluded[id] = struct{}{}
			}
			require.Equal(t, tt.want, got)
			require.Len(t, GetRequestAccountFailoverAttempts(ctx), len(tt.want))
		})
	}
}

func TestUpstreamFailoverUnselectableRound(t *testing.T) {
	accounts := upstreamFailoverAccounts(1, 1, 1, 2, 3)
	ctx := withRequestAccountFailoverAccounts(context.Background(), accounts)
	MarkRequestAccountFailoverAction(ctx, "switch_account", 1)
	excluded := map[int64]struct{}{1: {}}
	selection, err := selectAccountByFailoverUpstream(ctx, excluded, upstreamFailoverFirstAvailable(accounts[:3]))
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID)
	require.True(t, requestAccountFailoverFinalAttempt(ctx, 2), "unavailable upstreams must not prolong retries")
	require.Equal(t, map[int64]struct{}{1: {}}, excluded, "selection preferences must not modify the handler exclusions")
	MarkRequestAccountFailoverAction(ctx, "switch_account", 2)
	_, err = selectAccountByFailoverUpstream(ctx, excluded, func(context.Context, map[int64]struct{}) (*AccountSelectionResult, error) {
		t.Fatal("final failure must stop before selecting another account")
		return nil, nil
	})
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
}

func TestUpstreamFailoverSelectionErrorsAndCancellation(t *testing.T) {
	accounts := upstreamFailoverAccounts(1, 1, 2)
	ctx := withRequestAccountFailoverAccounts(context.Background(), accounts)
	errDatabase := errors.New("repository unavailable")
	calls := 0
	_, err := selectAccountByFailoverUpstream(ctx, nil, func(context.Context, map[int64]struct{}) (*AccountSelectionResult, error) {
		calls++
		return nil, errDatabase
	})
	require.ErrorIs(t, err, errDatabase)
	require.Equal(t, 1, calls)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = selectAccountByFailoverUpstream(canceled, nil, func(context.Context, map[int64]struct{}) (*AccountSelectionResult, error) {
		t.Fatal("canceled request must not select another account")
		return nil, nil
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestUpstreamFailoverLocalExclusionsAndDisabled(t *testing.T) {
	accounts := upstreamFailoverAccounts(1, 1, 1)
	ctx := withRequestAccountFailoverAccounts(context.Background(), accounts)
	excluded := map[int64]struct{}{1: {}}
	MarkRequestAccountFailoverAction(ctx, "skip_ineligible_account", 1)
	selection, err := selectAccountByFailoverUpstream(ctx, excluded, upstreamFailoverFirstAvailable(accounts))
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID)
	require.False(t, requestAccountFailoverFinalAttempt(ctx, 2), "profit/model exclusions are not upstream failures")
	MarkRequestAccountFailoverAction(ctx, "switch_account", 2)
	selection, err = selectAccountByFailoverUpstream(withRequestAccountFailoverCount(ctx, 0), excluded, upstreamFailoverFirstAvailable(accounts))
	require.NoError(t, err)
	require.Equal(t, int64(2), selection.Account.ID, "disabled policy uses the original scheduler")
}

func TestUpstreamFailoverFinalAttemptSuccess(t *testing.T) {
	accounts := upstreamFailoverAccounts(1, 1, 1)
	ctx := withRequestAccountFailoverAccounts(context.Background(), accounts)
	MarkRequestAccountFailoverAction(ctx, "switch_account", 1)
	selection, err := selectAccountByFailoverUpstream(ctx, map[int64]struct{}{1: {}}, upstreamFailoverFirstAvailable(accounts))
	require.NoError(t, err)
	ctx = context.WithValue(ctx, ctxkey.AccountID, selection.Account.ID)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	result, err := ForwardWithAccountRetries(ctx, c, func() (*OpenAIForwardResult, error) { return &OpenAIForwardResult{}, nil })
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "success", RequestAccountFailoverResult(c))
}

func TestUpstreamFailoverIdentity(t *testing.T) {
	key := func(id int64, base string) string {
		return accountFailoverUpstream(&Account{ID: id, Credentials: map[string]any{"base_url": base}})
	}
	require.Equal(t, key(1, "https://UPSTREAM.example:443/v1/"), key(2, "https://upstream.example/other?key=secret"))
	require.Equal(t, key(1, "http://upstream.example:80/"), key(2, "https://upstream.example"))
	require.NotEqual(t, key(1, "https://upstream.example:8443"), key(2, "https://upstream.example"))
	require.NotEqual(t, key(1, "://bad"), key(2, "://bad"))
	require.Equal(t, key(1, "https://api.anthropic.com"), accountFailoverUpstream(&Account{Platform: PlatformAnthropic, Type: AccountTypeAPIKey}))
	require.Equal(t, key(1, "https://chatgpt.com"), accountFailoverUpstream(&Account{Platform: PlatformOpenAI, Type: AccountTypeOAuth}))
	require.Equal(t, key(1, "https://relay.example"), accountFailoverUpstream(&Account{Platform: PlatformAnthropic, Type: AccountTypeOAuth, Extra: map[string]any{"custom_base_url_enabled": true, "custom_base_url": "https://relay.example"}}))
}

type upstreamFailoverSchedulerRepo struct{ schedulerTestOpenAIAccountRepo }

func (r upstreamFailoverSchedulerRepo) ListSchedulableByGroupIDAndPlatforms(_ context.Context, _ int64, _ []string) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

func TestUpstreamFailoverSchedulerEntrypoints(t *testing.T) {
	for _, mode := range []string{"anthropic_model", "anthropic_legacy", "anthropic_load", "openai_model", "openai_load", "openai_scheduler_legacy", "openai_scheduler_advanced"} {
		t.Run(mode, func(t *testing.T) {
			resetOpenAIAdvancedSchedulerSettingCacheForTest()
			t.Cleanup(resetOpenAIAdvancedSchedulerSettingCacheForTest)
			pool := []int{1, 1, 2, 2, 2, 2, 3, 3}
			accounts := upstreamFailoverAccounts(pool...)
			platform := PlatformOpenAI
			if mode == "anthropic_model" || mode == "anthropic_legacy" || mode == "anthropic_load" {
				platform = PlatformAnthropic
				for i := range accounts {
					accounts[i].Platform = platform
				}
			}
			settingsRepo := &gatewayTTLSettingRepo{data: map[string]string{
				SettingKeyAccountErrorFailoverEnabled: "true",
				openAIAdvancedSchedulerSettingKey:     fmt.Sprint(mode == "openai_scheduler_advanced"),
			}}
			settings := NewSettingService(settingsRepo, nil)
			group := &Group{ID: 1, Platform: platform, AccountErrorFailoverEnabled: true}
			repo := upstreamFailoverSchedulerRepo{schedulerTestOpenAIAccountRepo{accounts: accounts}}
			cfg := newSchedulerTestSubscriptionPriorityConfig()
			cfg.Gateway.Scheduling.LoadBatchEnabled = mode != "anthropic_legacy"
			cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Load = 0
			cfg.Gateway.OpenAIWS.SchedulerScoreWeights.Queue = 0
			var acquired, released []int64
			concurrency := NewConcurrencyService(schedulerTestConcurrencyCache{acquiredIDs: &acquired, releasedIDs: &released})
			gateway := &GatewayService{accountRepo: repo, groupRepo: requestAccountFailoverGroupRepo{groups: map[int64]*Group{1: group}}, cfg: cfg, settingService: settings, concurrencyService: concurrency}
			openai := &OpenAIGatewayService{accountRepo: repo, cfg: cfg, settingService: settings, concurrencyService: concurrency}
			ctx := context.Background()
			if platform == PlatformAnthropic {
				ctx = gateway.PrepareRequestAccountFailover(ctx, &group.ID, "", platform)
			} else {
				ctx = openai.PrepareRequestAccountFailover(ctx, &group.ID, group, "", platform)
			}
			require.True(t, RequestAccountFailoverEnabled(ctx))
			excluded := map[int64]struct{}{}
			var got []int
			for i := 0; i <= len(accounts); i++ {
				var selection *AccountSelectionResult
				var account *Account
				var err error
				switch mode {
				case "anthropic_model":
					account, err = gateway.SelectAccountForModelWithExclusions(ctx, &group.ID, "", "", excluded)
				case "anthropic_legacy", "anthropic_load":
					selection, err = gateway.SelectAccountWithLoadAwareness(ctx, &group.ID, "", "", excluded, "", 0)
				case "openai_model":
					account, err = openai.SelectAccountForModelWithExclusions(ctx, &group.ID, "", "", excluded)
				case "openai_load":
					selection, err = openai.SelectAccountWithLoadAwareness(ctx, &group.ID, "", "", excluded)
				default:
					selection, _, err = openai.SelectAccountWithSchedulerForCapability(ctx, &group.ID, "", "", "", excluded, OpenAIUpstreamTransportAny, OpenAIEndpointCapabilityChatCompletions, false, false, false)
				}
				if err != nil {
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					break
				}
				if selection != nil {
					account = selection.Account
					if selection.ReleaseFunc != nil {
						selection.ReleaseFunc()
					}
				}
				require.NotNil(t, account)
				require.NotContains(t, excluded, account.ID)
				got = append(got, pool[account.ID-1])
				excluded[account.ID] = struct{}{}
				MarkRequestAccountFailoverAction(ctx, "switch_account", account.ID)
			}
			require.Equal(t, []int{1, 2, 3, 1, 2, 3, 2}, got)
			require.ElementsMatch(t, acquired, released, "every acquired slot must be released")
		})
	}
}
