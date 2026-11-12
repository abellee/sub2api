package service

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

const SettingKeyAccountErrorFailoverEnabled = "account_error_failover_enabled"

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
	return context.WithValue(ctx, requestAccountFailoverKey{}, count)
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

// ForwardWithAccountRetries holds the selected account/slot for the original
// attempt and two retries. A partial result or response is never replayed.
func ForwardWithAccountRetries[T any](ctx context.Context, c *gin.Context, forward func() (*T, error)) (*T, error) {
	if !RequestAccountFailoverEnabled(ctx) {
		return forward()
	}
	for retry := 0; ; retry++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		compactSize := OpenAICompactKeepaliveAdjustedWrittenSize(c)
		imageSize := OpenAIImagesJSONKeepaliveAdjustedWrittenSize(c)
		result, err := forward()
		var failure *UpstreamFailoverError
		if result != nil || !errors.As(err, &failure) || !failure.ShouldRetryNextAccount() {
			return result, err
		}
		if OpenAICompactKeepaliveAdjustedWrittenSize(c) != compactSize && OpenAIImagesJSONKeepaliveAdjustedWrittenSize(c) != imageSize {
			return result, err
		}
		if retry == 2 {
			// The outer loop only switches accounts; it must not start another
			// retry budget or persist a cooldown after these three attempts.
			copy := *failure
			copy.RetryableOnSameAccount = false
			copy.SameAccountRetryDeadline = time.Time{}
			copy.SkipAccountCooldown = true
			return result, &copy
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
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
