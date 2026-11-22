package service

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
)

type recoveryAccountRepo struct {
	AccountRepository
	account  *Account
	siblings map[int64]*Account
}

func (r *recoveryAccountRepo) GetByID(_ context.Context, id int64) (*Account, error) {
	account := r.account
	if id != account.ID {
		account = r.siblings[id]
	}
	if account == nil {
		return nil, errors.New("account not found")
	}
	copy := *account
	return &copy, nil
}

func (r *recoveryAccountRepo) GetByIDs(ctx context.Context, ids []int64) ([]*Account, error) {
	var accounts []*Account
	for _, id := range ids {
		account, err := r.GetByID(ctx, id)
		if err == nil {
			accounts = append(accounts, account)
		}
	}
	return accounts, nil
}

func (r *recoveryAccountRepo) ClearError(_ context.Context, id int64) error {
	if id == r.account.ID {
		r.account.Status = StatusActive
	} else {
		r.siblings[id].Status = StatusActive
	}
	return nil
}

type recoveryPlanRepo struct {
	ScheduledTestPlanRepository
	plan            *ScheduledTestPlan
	accounts        *recoveryAccountRepo
	claim           bool
	advances, opens int
	claimUntil      time.Time
	due             []*ScheduledTestPlan
}

func (r *recoveryPlanRepo) ClaimDue(ctx context.Context, id int64, now, until time.Time) (bool, error) {
	r.claimUntil = until
	return r.claim, nil
}

func (r *recoveryPlanRepo) ListDue(context.Context, time.Time) ([]*ScheduledTestPlan, error) {
	return r.due, nil
}

func (r *recoveryPlanRepo) GetByID(context.Context, int64) (*ScheduledTestPlan, error) {
	copy := *r.plan
	return &copy, nil
}

func (r *recoveryPlanRepo) UpdateAfterRun(ctx context.Context, id int64, last, next, version time.Time) error {
	r.advances++
	return nil
}

func (r *recoveryPlanRepo) ListRecoveryAccountIDs(context.Context, *Account) ([]int64, error) {
	ids := []int64{r.accounts.account.ID}
	for id := range r.accounts.siblings {
		ids = append(ids, id)
	}
	return ids, nil
}

func (r *recoveryPlanRepo) EnableScheduling(ctx context.Context, plan *ScheduledTestPlan, probe *Account, accounts []*Account) (int, error) {
	if !r.plan.Enabled || !r.plan.RecoverScheduling || !r.plan.UpdatedAt.Equal(plan.UpdatedAt) {
		return 0, nil
	}
	for _, account := range accounts {
		r.opens++
		if account.ID == r.accounts.account.ID {
			r.accounts.account.Schedulable = true
		} else {
			r.accounts.siblings[account.ID].Schedulable = true
		}
	}
	return len(accounts), nil
}

type recoveryResultsRepo struct {
	ScheduledTestResultRepository
	results []ScheduledTestResult
	err     error
}

func (r *recoveryResultsRepo) Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error) {
	r.results = append(r.results, *result)
	return result, r.err
}

func (r *recoveryResultsRepo) PruneOldResults(context.Context, int64, int) error { return nil }

type recoveryTester struct {
	t             *testing.T
	calls, failAt int
	after         func(int)
	err           error
	accounts      *recoveryAccountRepo
}

func (p *recoveryTester) RunTestBackground(ctx context.Context, accountID int64, model string) (*ScheduledTestResult, error) {
	require.EqualValues(p.t, 42, accountID)
	require.Equal(p.t, "probe-model", model)
	p.calls++
	if p.after != nil {
		p.after(p.calls)
	}
	status := "success"
	if p.calls == p.failAt {
		status = "failed"
	}
	return &ScheduledTestResult{Status: status}, p.err
}

func recoveryRunner(t *testing.T) (*ScheduledTestRunnerService, *recoveryPlanRepo, *recoveryTester, *recoveryResultsRepo) {
	accounts := &recoveryAccountRepo{account: &Account{ID: 42, Status: StatusActive, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}}}
	plan := &ScheduledTestPlan{ID: 7, AccountID: 42, ModelID: "probe-model", CronExpression: "*/5 * * * *", Enabled: true, RecoverScheduling: true, MaxResults: 100, UpdatedAt: time.Now()}
	plans := &recoveryPlanRepo{plan: plan, accounts: accounts, claim: true}
	results := &recoveryResultsRepo{}
	tester := &recoveryTester{t: t, accounts: accounts}
	runner := &ScheduledTestRunnerService{planRepo: plans, scheduledSvc: NewScheduledTestService(plans, results), accountTestSvc: tester, accountRepo: accounts, rateLimitSvc: &RateLimitService{accountRepo: accounts}}
	return runner, plans, tester, results
}

func TestScheduledRecoveryRequiresDetectionAndThreeConfirmationSuccesses(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner, plans, tester, results := recoveryRunner(t)
		plan := *plans.plan
		runner.runOnePlan(context.Background(), &plan)
		require.Equal(t, 4, tester.calls)
		require.Len(t, results.results, 4)
		require.Equal(t, 1, plans.opens)
		require.True(t, plans.accounts.account.Schedulable)
		require.Equal(t, 1, plans.advances)
		// Subsequent scheduled ticks skip an already-open account without upstream requests.
		runner.runOnePlan(context.Background(), plans.plan)
		require.Equal(t, 4, tester.calls)
		require.Len(t, results.results, 4)
	})
}

func TestScheduledRecoveryKeepsLegacyAutoRecoverSeparate(t *testing.T) {
	runner, plans, tester, results := recoveryRunner(t)
	plans.plan.RecoverScheduling = false
	plans.plan.AutoRecover = true
	plans.accounts.account.Status = StatusError
	runner.runOnePlan(context.Background(), plans.plan)
	require.Equal(t, 1, tester.calls)
	require.Len(t, results.results, 1)
	require.Equal(t, StatusActive, plans.accounts.account.Status)
	require.False(t, plans.accounts.account.Schedulable)
	require.Zero(t, plans.opens)
}

func TestScheduledRecoveryFailureStopsAndNextCycleStartsFresh(t *testing.T) {
	for failAt := 1; failAt <= 4; failAt++ {
		t.Run(time.Duration(failAt).String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				runner, plans, tester, results := recoveryRunner(t)
				tester.failAt = failAt
				runner.runOnePlan(context.Background(), plans.plan)
				require.Equal(t, failAt, tester.calls)
				require.Len(t, results.results, failAt)
				require.Zero(t, plans.opens)
				require.False(t, plans.accounts.account.Schedulable)
				tester.failAt = 0
				runner.runOnePlan(context.Background(), plans.plan)
				require.Equal(t, failAt+4, tester.calls)
				require.Equal(t, 1, plans.opens)
			})
		})
	}
}

func TestScheduledRecoverySkipsIneligibleAccountsAndUnclaimedPlans(t *testing.T) {
	for _, state := range []string{"open", "inactive", "expired", "unclaimed"} {
		t.Run(state, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				runner, plans, tester, results := recoveryRunner(t)
				switch state {
				case "open":
					plans.accounts.account.Schedulable = true
				case "inactive":
					plans.accounts.account.Status = StatusDisabled
				case "expired":
					expired := time.Now().Add(-time.Minute)
					plans.accounts.account.ExpiresAt = &expired
				case "unclaimed":
					plans.claim = false
				}
				runner.runOnePlan(context.Background(), plans.plan)
				require.Zero(t, tester.calls)
				require.Empty(t, results.results)
				require.Zero(t, plans.opens)
			})
		})
	}
}

func TestScheduledRecoveryStopsAfterChangesOrLostResults(t *testing.T) {
	for _, change := range []string{"plan disabled", "plan edited", "account opened", "account paused", "credentials changed", "save failed", "test failed", "context canceled", "disabled on last test"} {
		t.Run(change, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				runner, plans, tester, results := recoveryRunner(t)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				tester.after = func(call int) {
					if change == "disabled on last test" {
						if call == 4 {
							plans.plan.Enabled = false
						}
						return
					}
					if call != 1 {
						return
					}
					switch change {
					case "plan disabled":
						plans.plan.Enabled = false
					case "plan edited":
						plans.plan.UpdatedAt = plans.plan.UpdatedAt.Add(time.Second)
					case "account opened":
						plans.accounts.account.Schedulable = true
					case "account paused":
						plans.accounts.account.Status = StatusDisabled
					case "credentials changed":
						plans.accounts.account.Credentials = map[string]any{"api_key": "changed"}
					case "save failed":
						results.err = errors.New("storage unavailable")
					case "test failed":
						tester.err = errors.New("upstream unavailable")
					case "context canceled":
						cancel()
					}
				}
				plan := *plans.plan
				runner.runOnePlan(ctx, &plan)
				require.Zero(t, plans.opens)
				require.Equal(t, 1, plans.advances, "failures still advance the schedule")
			})
		})
	}
}

func TestBackgroundTestRequiresSuccessfulCompletionEvent(t *testing.T) {
	for _, body := range []string{"", `data: {"type":"content","text":"partial"}`, `data: {"type":"test_complete","success":false}`, "data: {\"type\":\"test_complete\",\"success\":true}\ndata: {\"type\":\"error\"}"} {
		require.False(t, successfulTestSSECompletion(body))
	}
	require.True(t, successfulTestSSECompletion("data: {\"type\":\"content\",\"text\":\"ok\"}\ndata: {\"type\":\"test_complete\",\"success\":true}"))
}

func TestScheduledRecoveryUsesConfiguredConfirmationInterval(t *testing.T) {
	for _, seconds := range []int{0, 17, 60, 3600} {
		t.Run(time.Duration(seconds).String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				runner, plans, tester, _ := recoveryRunner(t)
				plans.plan.RecoveryProbeIntervalSeconds = seconds
				started := time.Now()
				var starts []time.Time
				tester.after = func(int) {
					starts = append(starts, time.Now())
					time.Sleep(2 * time.Second)
				}
				runner.runOnePlan(context.Background(), plans.plan)
				require.Len(t, starts, 4)
				require.Equal(t, started, starts[0], "the initial detection probe is immediate")
				if seconds == 0 {
					seconds = 3
				}
				for i := 1; i < len(starts); i++ {
					require.Equal(t, time.Duration(seconds+2)*time.Second, starts[i].Sub(starts[i-1]), "wait after the preceding probe completes")
				}
				require.Equal(t, 1, plans.opens)
				require.True(t, plans.claimUntil.After(time.Now()), "the execution lease must cover all configured waits")
			})
		})
	}
}

func TestScheduledRecoveryRechecksStateAfterWaitingAndCanStopDuringWait(t *testing.T) {
	for _, change := range []string{"plan disabled", "account opened", "runner stopped"} {
		t.Run(change, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				runner, plans, tester, _ := recoveryRunner(t)
				plans.plan.RecoveryProbeIntervalSeconds = 17
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				runner.cancel = cancel
				tester.after = func(call int) {
					if call != 1 {
						return
					}
					go func() {
						time.Sleep(5 * time.Second)
						switch change {
						case "plan disabled":
							plans.plan.Enabled = false
						case "account opened":
							plans.accounts.account.Schedulable = true
						case "runner stopped":
							runner.Stop()
						}
					}()
				}
				started := time.Now()
				runner.runOnePlan(ctx, plans.plan)
				require.Equal(t, 1, tester.calls)
				require.Zero(t, plans.opens)
				if change == "runner stopped" {
					require.Equal(t, 5*time.Second, time.Since(started))
				}
			})
		})
	}
}

func TestScheduledRecoveryWaitingDoesNotBlockOtherDuePlans(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner, plans, tester, _ := recoveryRunner(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		runner.runCtx, runner.cancel = ctx, cancel
		runner.workers = make(chan struct{}, scheduledTestDefaultMaxWorkers)
		plans.plan.RecoveryProbeIntervalSeconds = 3600
		plans.due = []*ScheduledTestPlan{plans.plan}
		runner.runScheduled()
		synctest.Wait()
		require.Equal(t, 1, tester.calls)
		other := *plans.plan
		other.ID, other.RecoverScheduling = 8, false
		plans.due = []*ScheduledTestPlan{&other}
		runner.runScheduled()
		synctest.Wait()
		require.Equal(t, 2, tester.calls, "another plan runs while the recovery plan waits")
		require.Zero(t, plans.opens)
		runner.Stop()
	})
}

func TestScheduledRecoveryOpensKeyCopiesWithOneProbeAccount(t *testing.T) {
	for _, representativeOpen := range []bool{false, true} {
		t.Run(map[bool]string{false: "representative closed", true: "representative open"}[representativeOpen], func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				runner, plans, tester, _ := recoveryRunner(t)
				anchor := plans.accounts.account
				anchor.Credentials = map[string]any{"api_key": "test", "base_url": "https://EXAMPLE.com:443/v1/"}
				anchor.Schedulable = representativeOpen
				plans.accounts.siblings = map[int64]*Account{}
				for id := int64(43); id <= 51; id++ {
					plans.accounts.siblings[id] = &Account{ID: id, Status: StatusError, Type: AccountTypeAPIKey, Platform: PlatformAnthropic, Credentials: map[string]any{"api_key": "test", "base_url": "https://example.com/v1"}}
				}
				plans.accounts.siblings[44].Credentials["api_key"] = "other"
				plans.accounts.siblings[45].Credentials["base_url"] = "https://other.example.com/v1"
				plans.accounts.siblings[46].Status = StatusDisabled
				expired := time.Now().Add(-time.Second)
				plans.accounts.siblings[47].ExpiresAt = &expired
				plans.accounts.siblings[48].Schedulable = true
				plans.accounts.siblings[48].Status = StatusActive
				tester.after = func(call int) {
					if call == 1 {
						plans.accounts.siblings[49].Credentials = map[string]any{"api_key": "changed", "base_url": "https://example.com/v1"}
						plans.accounts.siblings[50].UpdatedAt = time.Now().Add(time.Second)
					}
				}
				runner.runOnePlan(context.Background(), plans.plan)
				require.Equal(t, 4, tester.calls, "only account 42 is probed, regardless of copy count")
				require.True(t, anchor.Schedulable)
				for _, id := range []int64{43, 51} {
					require.True(t, plans.accounts.siblings[id].Schedulable)
					require.Equal(t, StatusActive, plans.accounts.siblings[id].Status)
				}
				for _, id := range []int64{44, 45, 46, 47, 49, 50} {
					require.False(t, plans.accounts.siblings[id].Schedulable, "account %d must stay closed", id)
				}
				require.Equal(t, StatusActive, plans.accounts.siblings[48].Status)
				runner.runOnePlan(context.Background(), plans.plan)
				require.Equal(t, 8, tester.calls, "unchanged eligible copies still closed at next cycle can be recovered")
				runner.runOnePlan(context.Background(), plans.plan)
				require.Equal(t, 8, tester.calls, "stop probing once all eligible copies are open")
			})
		})
	}
}

func TestScheduledRecoveryKeyIdentity(t *testing.T) {
	anchor := &Account{ID: 1, Type: AccountTypeAPIKey, Platform: PlatformOpenAI, Credentials: map[string]any{"api_key": "key", "base_url": "https://EXAMPLE.com:443/api/"}}
	for _, tc := range []struct {
		url, key string
		matches  bool
	}{
		{"https://example.com/api", "key", true},
		{"https://example.com:8443/api", "key", false},
		{"http://example.com/api", "key", false},
		{"https://example.com/API", "key", false},
		{"https://example.com/api", "other", false},
		{"", "key", false},
	} {
		other := &Account{ID: 2, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": tc.key, "base_url": tc.url}}
		require.Equal(t, tc.matches, sameRecoveryKey(anchor, other), tc.url)
	}
}

func TestScheduledRecoveryFailureKeepsAllKeyCopiesClosed(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		runner, plans, tester, _ := recoveryRunner(t)
		plans.accounts.account.Status = StatusError
		plans.accounts.siblings = map[int64]*Account{
			43: {ID: 43, Status: StatusError, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test"}},
		}
		tester.failAt = 3
		runner.runOnePlan(context.Background(), plans.plan)
		require.Equal(t, 3, tester.calls)
		require.Zero(t, plans.opens)
		for _, account := range []*Account{plans.accounts.account, plans.accounts.siblings[43]} {
			require.False(t, account.Schedulable)
			require.Equal(t, StatusError, account.Status, "failed confirmation must not clear errors")
		}
	})
}
