package service

import (
	"context"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/robfig/cron/v3"
)

const scheduledTestDefaultMaxWorkers = 10

type scheduledAccountTester interface {
	RunTestBackground(context.Context, int64, string) (*ScheduledTestResult, error)
}

// ScheduledTestRunnerService periodically scans due test plans and executes them.
type ScheduledTestRunnerService struct {
	planRepo       ScheduledTestPlanRepository
	scheduledSvc   *ScheduledTestService
	accountTestSvc scheduledAccountTester
	rateLimitSvc   *RateLimitService
	cfg            *config.Config
	accountRepo    AccountRepository
	runMu          sync.Mutex
	workers        chan struct{}
	runCtx         context.Context
	cancel         context.CancelFunc

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
}

// NewScheduledTestRunnerService creates a new runner.
func NewScheduledTestRunnerService(
	planRepo ScheduledTestPlanRepository,
	scheduledSvc *ScheduledTestService,
	accountTestSvc *AccountTestService,
	rateLimitSvc *RateLimitService,
	cfg *config.Config,
) *ScheduledTestRunnerService {
	var accountRepo AccountRepository
	if accountTestSvc != nil {
		accountRepo = accountTestSvc.accountRepo
	}
	runCtx, cancel := context.WithCancel(context.Background())
	return &ScheduledTestRunnerService{
		planRepo:       planRepo,
		scheduledSvc:   scheduledSvc,
		accountTestSvc: accountTestSvc,
		rateLimitSvc:   rateLimitSvc,
		cfg:            cfg,
		accountRepo:    accountRepo,
		workers:        make(chan struct{}, scheduledTestDefaultMaxWorkers),
		runCtx:         runCtx,
		cancel:         cancel,
	}
}

// Start begins the cron ticker (every minute).
func (s *ScheduledTestRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] started (tick=every minute)")
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *ScheduledTestRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cancel != nil {
			s.cancel()
		}
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] cron stop timed out")
			}
		}
	})
}

func (s *ScheduledTestRunnerService) runScheduled() {
	if !s.runMu.TryLock() {
		return
	}
	defer s.runMu.Unlock()
	// Delay 10s so execution lands at ~:10 of each minute instead of :00.
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case <-s.runCtx.Done():
		return
	case <-timer.C:
	}

	ctx, cancel := context.WithTimeout(s.runCtx, 10*time.Second)
	defer cancel()

	now := time.Now()
	plans, err := s.planRepo.ListDue(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] found %d due plans", len(plans))

	for _, plan := range plans {
		select {
		case <-s.runCtx.Done():
			return
		case s.workers <- struct{}{}:
		default:
			return
		}
		go func(p *ScheduledTestPlan) {
			defer func() { <-s.workers }()
			s.runOnePlan(s.runCtx, p)
		}(plan)
	}
}

func (s *ScheduledTestRunnerService) runOnePlan(ctx context.Context, plan *ScheduledTestPlan) {
	budget := 5 * time.Minute
	if plan.RecoverScheduling {
		budget += 3 * recoveryProbeInterval(plan)
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	now := time.Now()
	claimed, err := s.planRepo.ClaimDue(ctx, plan.ID, now, now.Add(budget+time.Minute))
	if err != nil || !claimed {
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d claim failed: %v", plan.ID, err)
		}
		return
	}
	defer s.advancePlan(plan)

	if plan.RecoverScheduling {
		s.runSchedulingRecovery(ctx, plan)
		return
	}
	result, err := s.accountTestSvc.RunTestBackground(ctx, plan.AccountID, plan.ModelID)
	if err != nil || result == nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d RunTestBackground error: %v", plan.ID, err)
		return
	}
	if err := s.scheduledSvc.SaveResult(ctx, plan.ID, plan.MaxResults, result); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
		return
	}
	if result.Status == "success" && plan.AutoRecover {
		s.tryRecoverAccount(ctx, plan.AccountID, plan.ID)
	}
}

func (s *ScheduledTestRunnerService) advancePlan(plan *ScheduledTestPlan) {
	// A timed-out probe must still advance its schedule instead of running again each minute.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	now := time.Now()
	nextRun, err := computeNextRun(plan.CronExpression, now)
	if err == nil {
		err = s.planRepo.UpdateAfterRun(ctx, plan.ID, now, nextRun, plan.UpdatedAt)
	}
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d advance schedule failed: %v", plan.ID, err)
	}
}

func schedulingRecoveryTarget(account *Account) bool {
	return recoveryProbeEligible(account) && !account.Schedulable
}

func recoveryProbeEligible(account *Account) bool {
	return account != nil &&
		(account.Status == StatusActive || account.Status == StatusError) &&
		(account.ExpiresAt == nil || account.ExpiresAt.After(time.Now())) &&
		!account.IsSyntheticUITest()
}

func recoveryProbeInterval(plan *ScheduledTestPlan) time.Duration {
	seconds := plan.RecoveryProbeIntervalSeconds
	if seconds < 1 || seconds > maxRecoveryProbeIntervalSeconds {
		seconds = defaultRecoveryProbeIntervalSeconds
	}
	return time.Duration(seconds) * time.Second
}

func (s *ScheduledTestRunnerService) runSchedulingRecovery(ctx context.Context, plan *ScheduledTestPlan) {
	if s.accountRepo == nil || s.rateLimitSvc == nil {
		return
	}
	original, err := s.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil || !recoveryProbeEligible(original) {
		return
	}
	ids, err := s.planRepo.ListRecoveryAccountIDs(ctx, original)
	if err != nil {
		return
	}
	members, err := s.accountRepo.GetByIDs(ctx, ids)
	if err != nil {
		return
	}
	targets := make([]*Account, 0, len(members))
	for _, member := range members {
		if schedulingRecoveryTarget(member) && sameRecoveryKey(original, member) {
			targets = append(targets, member)
		}
	}
	if len(targets) == 0 {
		return
	}
	// The first success detects recovery; three additional successes confirm it.
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(recoveryProbeInterval(plan))
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
		currentPlan, err := s.planRepo.GetByID(ctx, plan.ID)
		if err != nil || !currentPlan.Enabled || !currentPlan.RecoverScheduling || !currentPlan.UpdatedAt.Equal(plan.UpdatedAt) {
			return
		}
		account, err := s.accountRepo.GetByID(ctx, plan.AccountID)
		if err != nil || !recoveryProbeEligible(account) || account.Schedulable != original.Schedulable || !sameScheduledTestIdentity(original, account) {
			return
		}
		probeCtx, cancel := context.WithTimeout(ctx, time.Minute)
		result, err := s.accountTestSvc.RunTestBackground(probeCtx, plan.AccountID, plan.ModelID)
		probeErr := probeCtx.Err()
		cancel()
		if err != nil || result == nil {
			return
		}
		if probeErr != nil {
			result.Status, result.ErrorMessage = "failed", probeErr.Error()
		}
		if err := s.scheduledSvc.SaveResult(ctx, plan.ID, plan.MaxResults, result); err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
			return
		}
		if result.Status != "success" {
			return
		}
	}
	if ctx.Err() != nil {
		return
	}
	account, err := s.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil || !recoveryProbeEligible(account) || account.Schedulable != original.Schedulable || !sameScheduledTestIdentity(original, account) {
		return
	}
	currentPlan, err := s.planRepo.GetByID(ctx, plan.ID)
	if err != nil || !currentPlan.Enabled || !currentPlan.RecoverScheduling || !currentPlan.UpdatedAt.Equal(plan.UpdatedAt) {
		return
	}
	recovered := make([]*Account, 0, len(targets))
	for _, target := range targets {
		member, err := s.accountRepo.GetByID(ctx, target.ID)
		if err != nil || !schedulingRecoveryTarget(member) || !member.UpdatedAt.Equal(target.UpdatedAt) || !sameScheduledTestIdentity(target, member) {
			continue
		}
		if _, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, target.ID); err != nil {
			return
		}
		member, err = s.accountRepo.GetByID(ctx, target.ID)
		if err == nil && schedulingRecoveryTarget(member) && sameScheduledTestIdentity(target, member) {
			recovered = append(recovered, member)
		}
	}
	// Runtime recovery can update the representative account's version as well.
	probe, err := s.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil || !recoveryProbeEligible(probe) || probe.Schedulable != original.Schedulable || !sameScheduledTestIdentity(original, probe) {
		return
	}
	opened, err := s.planRepo.EnableScheduling(ctx, plan, probe, recovered)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d enable scheduling failed: %v", plan.ID, err)
	} else if opened > 0 {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d probe_account=%d enabled_accounts=%d after recovery and 3 confirmation tests", plan.ID, plan.AccountID, opened)
	}
}

// A key may be copied across platforms and groups, but never across upstreams.
func sameRecoveryKey(a, b *Account) bool {
	if a.ID == b.ID {
		return true
	}
	if a.Type != AccountTypeAPIKey || b.Type != AccountTypeAPIKey {
		return false
	}
	key := a.GetCredential("api_key")
	if strings.TrimSpace(key) == "" || key != b.GetCredential("api_key") {
		return false
	}
	aURL, bURL := recoveryBaseURL(a), recoveryBaseURL(b)
	return aURL != "" && aURL == bURL
}

func recoveryBaseURL(account *Account) string {
	raw := strings.TrimSpace(account.GetCredential("base_url"))
	if raw == "" {
		// Implicit defaults differ across platforms; don't merge those platforms.
		return "default:" + account.Platform
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return ""
	}
	u.Host = strings.ToLower(u.Host)
	if (u.Scheme == "https" && u.Port() == "443") || (u.Scheme == "http" && u.Port() == "80") {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String()
}

func sameScheduledTestIdentity(a, b *Account) bool {
	return a.Platform == b.Platform && a.Type == b.Type &&
		reflect.DeepEqual(a.Credentials, b.Credentials) && reflect.DeepEqual(a.ProxyID, b.ProxyID)
}

// tryRecoverAccount attempts to recover an account from recoverable runtime state.
func (s *ScheduledTestRunnerService) tryRecoverAccount(ctx context.Context, accountID int64, planID int64) {
	if s.rateLimitSvc == nil {
		return
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, accountID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover failed: %v", planID, err)
		return
	}
	if recovery == nil {
		return
	}

	if recovery.ClearedError {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d recovered from error status", planID, accountID)
	}
	if recovery.ClearedRateLimit {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d cleared rate-limit/runtime state", planID, accountID)
	}
}
