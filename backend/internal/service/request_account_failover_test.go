package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestForwardWithAccountRetries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name      string
		accounts  int
		succeedAt int
		partial   bool
		write     bool
		stop      bool
		wantCalls int
	}{
		{name: "single account unchanged", accounts: 1, wantCalls: 1},
		{name: "two retries then switch", accounts: 2, wantCalls: 3},
		{name: "success stops retries", accounts: 2, succeedAt: 3, wantCalls: 3},
		{name: "partial result never replayed", accounts: 2, partial: true, wantCalls: 1},
		{name: "written response never replayed", accounts: 2, write: true, wantCalls: 1},
		{name: "request or provider stop preserved", accounts: 2, stop: true, wantCalls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx := withRequestAccountFailoverCount(context.Background(), tt.accounts)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
			calls := 0
			original := &UpstreamFailoverError{StatusCode: 502, RetryableOnSameAccount: true}
			if tt.stop {
				original.NextAccountAction = NextAccountStop
			}
			result, err := ForwardWithAccountRetries(ctx, c, func() (*OpenAIForwardResult, error) {
				calls++
				if calls == tt.succeedAt {
					return &OpenAIForwardResult{}, nil
				}
				if tt.write {
					_, _ = c.Writer.WriteString("data: partial output\n\n")
				}
				if tt.partial {
					return &OpenAIForwardResult{ImageCount: 1}, original
				}
				return nil, original
			})
			require.Equal(t, tt.wantCalls, calls)
			if tt.succeedAt > 0 {
				require.NoError(t, err)
				require.NotNil(t, result)
			} else {
				var failure *UpstreamFailoverError
				require.ErrorAs(t, err, &failure)
				if calls == 3 {
					require.False(t, failure.RetryableOnSameAccount, "outer loop must only switch accounts")
					require.True(t, failure.SkipAccountCooldown, "exhaustion must not cool the account")
				}
			}
			require.True(t, original.RetryableOnSameAccount, "shared errors must not be mutated")
		})
	}
}

func TestForwardWithAccountRetriesCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(withRequestAccountFailoverCount(context.Background(), 2))
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	calls := 0
	_, err := ForwardWithAccountRetries(ctx, c, func() (*OpenAIForwardResult, error) {
		calls++
		cancel()
		return nil, &UpstreamFailoverError{StatusCode: 502}
	})
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 1, calls)
}

func TestRequestAccountFailoverPreservesAccountState(t *testing.T) {
	ctx := withRequestAccountFailoverCount(context.Background(), 2)
	account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true}
	limiter := &RateLimitService{}
	for _, status := range []int{401, 403, 429, 502, 503} {
		require.False(t, limiter.HandleUpstreamError(ctx, account, status, nil, []byte(`{"error":{"message":"unavailable"}}`)))
		require.False(t, limiter.HandleTempUnschedulable(ctx, account, status, nil))
		require.Equal(t, ErrorPolicyNone, limiter.CheckErrorPolicy(ctx, account, status, nil))
	}
	require.True(t, account.IsSchedulable())
	require.False(t, limiter.HandleStreamTimeout(ctx, account, "test-model"))
	require.False(t, limiter.ObserveOpenAIAPIKeyHealthFailure(ctx, account, errors.New("upstream unavailable")))
	require.Equal(t, 11, RequestAccountFailoverLimit(withRequestAccountFailoverCount(ctx, 12), 1))
	single := withRequestAccountFailoverCount(ctx, 1)
	require.False(t, RequestAccountFailoverEnabled(single))
	require.Equal(t, 10, RequestAccountFailoverLimit(single, 10))
}

func TestAccountCredentialFailoverPreservesSingleAccount(t *testing.T) {
	account := &Account{Type: AccountTypeAPIKey, Platform: PlatformOpenAI}
	for _, getToken := range []func(context.Context, *Account) (string, string, error){
		(&GatewayService{}).GetAccessToken,
		(&OpenAIGatewayService{}).GetAccessToken,
	} {
		_, _, singleErr := getToken(context.Background(), account)
		require.EqualError(t, singleErr, "api_key not found in credentials")
		_, _, multiErr := getToken(withRequestAccountFailoverCount(context.Background(), 2), account)
		var failure *UpstreamFailoverError
		require.ErrorAs(t, multiErr, &failure)
		require.True(t, failure.IsCredentialFailure())
		require.True(t, failure.ShouldRetryNextAccount())
		require.NotContains(t, failure.ClientMessage, "api_key")
	}
}

func TestPrepareRequestAccountFailoverCanceledSkipsRepository(t *testing.T) {
	ctx, cancel := context.WithCancel(withRequestAccountFailoverCount(context.Background(), 2))
	cancel()
	require.False(t, RequestAccountFailoverEnabled((&GatewayService{}).PrepareRequestAccountFailover(ctx, nil, "model", PlatformAnthropic)))
	require.False(t, RequestAccountFailoverEnabled((&OpenAIGatewayService{}).PrepareRequestAccountFailover(ctx, nil, nil, "model", PlatformOpenAI)))
}

func TestRequestAccountFailoverSwitchDisabledSkipsRepository(t *testing.T) {
	for _, settings := range []*SettingService{nil, NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{}}, nil)} {
		ctx := withRequestAccountFailoverCount(context.Background(), 2)
		gateway := &GatewayService{settingService: settings}
		openai := &OpenAIGatewayService{settingService: settings}
		require.False(t, RequestAccountFailoverEnabled(gateway.PrepareRequestAccountFailover(ctx, nil, "model", PlatformAnthropic)))
		require.False(t, RequestAccountFailoverEnabled(openai.PrepareRequestAccountFailover(ctx, nil, nil, "model", PlatformOpenAI)))
	}
}

func TestRequestAccountFailoverSwitchSavedAndRefreshed(t *testing.T) {
	settings := NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{SettingKeyAccountErrorFailoverEnabled: "true"}}, nil)
	require.True(t, settings.IsAccountErrorFailoverEnabled(context.Background()), "load persisted switch on startup")
	settings.refreshCachedSettings(&SystemSettings{AccountErrorFailoverEnabled: false})
	require.False(t, settings.IsAccountErrorFailoverEnabled(context.Background()))
	settings.refreshCachedSettings(&SystemSettings{AccountErrorFailoverEnabled: true})
	require.True(t, settings.IsAccountErrorFailoverEnabled(context.Background()))
}

type requestAccountFailoverAccountRepo struct {
	AccountRepository
	accounts []Account
}

func (r requestAccountFailoverAccountRepo) ListSchedulableUngroupedByPlatform(context.Context, string) ([]Account, error) {
	return r.accounts, nil
}

func (r requestAccountFailoverAccountRepo) ListSchedulableByGroupIDAndPlatform(context.Context, int64, string) ([]Account, error) {
	return r.accounts, nil
}

type requestAccountFailoverGroupRepo struct {
	GroupRepository
	groups map[int64]*Group
}

func (r requestAccountFailoverGroupRepo) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	if group := r.groups[id]; group != nil {
		return group, nil
	}
	return nil, ErrGroupNotFound
}

func TestRequestAccountFailoverRequiresBothSwitches(t *testing.T) {
	for _, systemEnabled := range []bool{false, true} {
		for _, groupEnabled := range []bool{false, true} {
			value := "false"
			if systemEnabled {
				value = "true"
			}
			settings := NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{SettingKeyAccountErrorFailoverEnabled: value}}, nil)
			group := &Group{ID: 1, Platform: PlatformOpenAI, AccountErrorFailoverEnabled: groupEnabled}
			groups := requestAccountFailoverGroupRepo{groups: map[int64]*Group{1: group}}
			want := systemEnabled && groupEnabled
			var accounts AccountRepository
			if want {
				accounts = requestAccountFailoverAccountRepo{accounts: []Account{
					{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
					{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true},
				}}
			}
			gateway := &GatewayService{accountRepo: accounts, groupRepo: groups, settingService: settings}
			openai := &OpenAIGatewayService{accountRepo: accounts, settingService: settings}
			ctx := withRequestAccountFailoverCount(context.Background(), 2)
			require.Equal(t, want, RequestAccountFailoverEnabled(gateway.PrepareRequestAccountFailover(ctx, &group.ID, "", PlatformOpenAI)))
			require.Equal(t, want, RequestAccountFailoverEnabled(openai.PrepareRequestAccountFailover(ctx, &group.ID, group, "", PlatformOpenAI)))
		}
	}
}

func TestRequestAccountFailoverResetsForMissingOrFallbackGroup(t *testing.T) {
	settings := NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{SettingKeyAccountErrorFailoverEnabled: "true"}}, nil)
	fallbackID := int64(2)
	group := &Group{ID: 1, Platform: PlatformAnthropic, AccountErrorFailoverEnabled: true, ClaudeCodeOnly: true, FallbackGroupID: &fallbackID}
	groups := requestAccountFailoverGroupRepo{groups: map[int64]*Group{
		1: group,
		2: {ID: 2, Platform: PlatformAnthropic, AccountErrorFailoverEnabled: false},
	}}
	gateway := &GatewayService{groupRepo: groups, settingService: settings}
	openai := &OpenAIGatewayService{settingService: settings}
	ctx := withRequestAccountFailoverCount(context.Background(), 2)
	require.False(t, RequestAccountFailoverEnabled(gateway.PrepareRequestAccountFailover(ctx, &group.ID, "", PlatformAnthropic)))
	require.False(t, RequestAccountFailoverEnabled(gateway.PrepareRequestAccountFailover(ctx, nil, "", PlatformOpenAI)))
	require.False(t, RequestAccountFailoverEnabled(openai.PrepareRequestAccountFailover(ctx, nil, nil, "", PlatformOpenAI)))
	require.False(t, RequestAccountFailoverEnabled(openai.PrepareRequestAccountFailover(ctx, &fallbackID, group, "", PlatformOpenAI)))
}

func TestPrepareRequestAccountFailoverCountsOnlyEnabledAccounts(t *testing.T) {
	for _, activeCount := range []int{1, 2} {
		settings := NewSettingService(&gatewayTTLSettingRepo{data: map[string]string{SettingKeyAccountErrorFailoverEnabled: "true"}}, nil)
		accounts := []Account{
			{ID: 90, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusDisabled, Schedulable: true},
			{ID: 91, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: false},
		}
		for id := 1; id <= activeCount; id++ {
			accounts = append(accounts, Account{ID: int64(id), Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true})
		}
		repo := requestAccountFailoverAccountRepo{accounts: accounts}
		group := &Group{ID: 1, Platform: PlatformOpenAI, AccountErrorFailoverEnabled: true}
		groups := requestAccountFailoverGroupRepo{groups: map[int64]*Group{1: group}}
		gateway := &GatewayService{accountRepo: repo, groupRepo: groups, settingService: settings}
		openai := &OpenAIGatewayService{accountRepo: repo, settingService: settings}
		for _, ctx := range []context.Context{
			gateway.PrepareRequestAccountFailover(context.Background(), &group.ID, "", PlatformOpenAI),
			openai.PrepareRequestAccountFailover(context.Background(), &group.ID, group, "", PlatformOpenAI),
		} {
			require.Equal(t, activeCount > 1, RequestAccountFailoverEnabled(ctx))
			require.Equal(t, activeCount-1, RequestAccountFailoverLimit(ctx, 0))
		}
		require.Equal(t, StatusDisabled, accounts[0].Status)
		require.False(t, accounts[1].Schedulable)
	}
}
