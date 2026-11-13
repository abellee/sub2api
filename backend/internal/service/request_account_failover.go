package service

import (
	"context"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const SettingKeyAccountErrorFailoverEnabled = "account_error_failover_enabled"

const OpsAccountFailoverResultKey = "ops_account_failover_result"
const accountAttemptWriterSizeKey = "account_attempt_writer_size"

// IsAccountErrorFailoverEnabled caches the switch per service and defaults to off.
func (s *SettingService) IsAccountErrorFailoverEnabled(ctx context.Context) bool {
	if s == nil || s.settingRepo == nil {
		return false
	}
	s.accountErrorFailoverMu.Lock()
	defer s.accountErrorFailoverMu.Unlock()
	if time.Now().Before(s.accountErrorFailoverExpiresAt) {
		return s.accountErrorFailoverEnabled
	}
	dbCtx, cancel := context.WithTimeout(ctx, gatewayForwardingDBTimeout)
	defer cancel()
	value, err := s.settingRepo.GetValue(dbCtx, SettingKeyAccountErrorFailoverEnabled)
	s.accountErrorFailoverEnabled = err == nil && value == "true"
	ttl := gatewayForwardingCacheTTL
	if err != nil {
		ttl = gatewayForwardingErrorTTL
	}
	s.accountErrorFailoverExpiresAt = time.Now().Add(ttl)
	return s.accountErrorFailoverEnabled
}

type requestAccountFailoverKey struct{}
type requestAccountFailoverTraceKey struct{}

type RequestAccountFailoverTrace struct {
	mu               sync.Mutex
	Actions          []string
	FailedAccountIDs []int64
}

func MarkRequestAccountFailoverAction(ctx context.Context, action string, accountID int64) {
	if !RequestAccountFailoverEnabled(ctx) {
		return
	}
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	if trace == nil {
		return
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	trace.Actions = append(trace.Actions, action)
	if action == "switch_account" && accountID > 0 {
		for _, existing := range trace.FailedAccountIDs {
			if existing == accountID {
				return
			}
		}
		trace.FailedAccountIDs = append(trace.FailedAccountIDs, accountID)
	}
}

func GetRequestAccountFailoverTrace(ctx context.Context) (actions []string, failedAccountIDs []int64) {
	if ctx == nil {
		return nil, nil
	}
	trace, _ := ctx.Value(requestAccountFailoverTraceKey{}).(*RequestAccountFailoverTrace)
	if trace == nil {
		return nil, nil
	}
	trace.mu.Lock()
	defer trace.mu.Unlock()
	return append([]string(nil), trace.Actions...), append([]int64(nil), trace.FailedAccountIDs...)
}

// RequestAccountFailoverEnabled keeps retries and exclusions within this request.
func RequestAccountFailoverEnabled(ctx context.Context) bool {
	return RequestAccountFailoverLimit(ctx, 0) > 0
}

func RequestAccountFailoverLimit(ctx context.Context, fallback int) int {
	if ctx != nil {
		if count, ok := ctx.Value(requestAccountFailoverKey{}).(int); ok && count > 1 {
			return count - 1
		}
	}
	return fallback
}

func withRequestAccountFailoverCount(ctx context.Context, count int) context.Context {
	// Reset the decision when a request moves to a fallback group.
	ctx = context.WithValue(ctx, requestAccountFailoverKey{}, count)
	if count > 1 {
		return context.WithValue(ctx, requestAccountFailoverTraceKey{}, &RequestAccountFailoverTrace{})
	}
	return context.WithValue(ctx, requestAccountFailoverTraceKey{}, (*RequestAccountFailoverTrace)(nil))
}

func (s *GatewayService) PrepareRequestAccountFailover(ctx context.Context, groupID *int64, model, platform string) context.Context {
	if ctx.Err() != nil || !s.settingService.IsAccountErrorFailoverEnabled(ctx) {
		return withRequestAccountFailoverCount(ctx, 0)
	}
	group, resolvedGroupID, err := s.resolveGatewayGroup(ctx, groupID)
	if err != nil || group == nil || !group.AccountErrorFailoverEnabled {
		return withRequestAccountFailoverCount(ctx, 0)
	}
	groupID = resolvedGroupID
	ctx = s.withGroupContext(ctx, group)
	if group != nil {
		platform = group.Platform
	}
	forcedPlatform, forced := ctx.Value(ctxkey.ForcePlatform).(string)
	if forced && forcedPlatform != "" {
		platform = forcedPlatform
	}
	if group != nil && group.Platform == PlatformComposite && !forced {
		decision, ok, err := s.resolveCompositeRouteDecision(ctx, group, model, CompositeRouteEndpointAny)
		if err != nil || !ok {
			return withRequestAccountFailoverCount(ctx, 0)
		}
		platform, model = decision.TargetPlatform, decision.UpstreamModel
		ctx = WithCompositeRouteDecision(ctx, decision)
	}
	accounts, mixed, err := s.listSchedulableAccounts(ctx, groupID, platform, forced)
	if err != nil {
		return withRequestAccountFailoverCount(ctx, 0)
	}
	count := 0
	for i := range accounts {
		a := &accounts[i]
		if s.isAccountSchedulableForSelection(a) && s.isAccountAllowedForPlatform(a, platform, mixed) &&
			(model == "" || s.isModelSupportedByAccountWithContext(ctx, a, model)) &&
			s.isAccountSchedulableForModelSelection(ctx, a, model) && s.isAccountSchedulableForQuota(a) {
			count++
		}
	}
	return withRequestAccountFailoverCount(ctx, count)
}

func (s *OpenAIGatewayService) PrepareRequestAccountFailover(ctx context.Context, groupID *int64, group *Group, model, platform string, capabilities ...OpenAIEndpointCapability) context.Context {
	if ctx.Err() != nil || !s.settingService.IsAccountErrorFailoverEnabled(ctx) ||
		groupID == nil || group == nil || group.ID != *groupID || !group.AccountErrorFailoverEnabled {
		return withRequestAccountFailoverCount(ctx, 0)
	}
	platform = NormalizeOpenAICompatiblePlatform(platform)
	capability := OpenAIEndpointCapability("")
	if len(capabilities) > 0 {
		capability = capabilities[0]
	}
	accounts, err := s.listSchedulableAccounts(ctx, groupID, platform)
	if err != nil {
		return withRequestAccountFailoverCount(ctx, 0)
	}
	count := 0
	for i := range accounts {
		if OpenAIImagesEndpointFromContext(ctx) && !accounts[i].SupportsOpenAIImageCapability(OpenAIImagesCapabilityBasic) {
			continue
		}
		if openAICompatibleAccountEligibilityFailureReason(ctx, &accounts[i], platform, model, false, capability) == "" &&
			!s.isOpenAIAccountRequestRuntimeBlocked(&accounts[i], model) {
			count++
		}
	}
	return withRequestAccountFailoverCount(ctx, count)
}

// ForwardWithAccountRetries leaves retries to the handler's account-selection
// loop so an enabled failover request advances to the next account immediately.
func ForwardWithAccountRetries[T any](ctx context.Context, c *gin.Context, forward func() (*T, error)) (*T, error) {
	if !RequestAccountFailoverEnabled(ctx) {
		return forward()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if c != nil {
		c.Set(accountAttemptWriterSizeKey, c.Writer.Size())
		c.Set(ResponseCommittedKey, false)
		c.Set(OpsAccountFailoverResultKey, "failed")
	}
	result, err := forward()
	if ctx.Err() != nil {
		return result, ctx.Err()
	}
	switch partial := any(result).(type) {
	case *ForwardResult:
		if partial != nil && partial.ClientDisconnect {
			if c != nil {
				c.Set(OpsAccountFailoverResultKey, "canceled")
			}
			return result, err
		}
	case *OpenAIForwardResult:
		if partial != nil && partial.ClientDisconnect {
			if c != nil {
				c.Set(OpsAccountFailoverResultKey, "canceled")
			}
			return result, err
		}
	}
	err = requestAccountFailoverError(ctx, err)
	if c != nil && err == nil && result != nil {
		c.Set(OpsAccountFailoverResultKey, "success")
	}
	var failure *UpstreamFailoverError
	if RequestAccountFailoverEnabled(ctx) && errors.As(err, &failure) && failure.ShouldRetryNextAccount() {
		copy := *failure
		copy.RetryableOnSameAccount = false
		copy.SameAccountRetryDeadline = time.Time{}
		copy.SkipAccountCooldown = true
		err = &copy
	}
	return result, err
}

func accountAttemptWroteResponse(c *gin.Context) bool {
	if size, ok := c.Get(accountAttemptWriterSizeKey); ok {
		return c.Writer.Size() != size.(int)
	}
	return c.Writer.Written()
}

// An HTTP 200 or partial usage from an earlier account is not proof of success.
func RequestAccountFailoverResult(c *gin.Context) string {
	if c == nil || c.Request == nil || !RequestAccountFailoverEnabled(c.Request.Context()) {
		return ""
	}
	if c.Request.Context().Err() != nil {
		return "canceled"
	}
	return c.GetString(OpsAccountFailoverResultKey)
}

// Credential errors remain local to the request; never expose token-provider details.
func requestAccountCredentialError(ctx context.Context, err error) error {
	if err == nil || !RequestAccountFailoverEnabled(ctx) {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	var failure *UpstreamFailoverError
	if errors.As(err, &failure) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return &UpstreamFailoverError{
		StatusCode:        http.StatusServiceUnavailable,
		Stage:             GatewayFailureStageAccountAuth,
		Scope:             GatewayFailureScopeAccount,
		Reason:            "account_credential_unavailable",
		NextAccountAction: NextAccountRetry,
		ClientStatusCode:  http.StatusServiceUnavailable,
		ClientMessage:     "No healthy account credentials are currently available",
	}
}

func requestAccountFailoverEnabled(c *gin.Context) bool {
	return c != nil && c.Request != nil && RequestAccountFailoverEnabled(c.Request.Context())
}

func accountFailoverContext(c *gin.Context) context.Context {
	if c == nil || c.Request == nil {
		return nil
	}
	return c.Request.Context()
}
