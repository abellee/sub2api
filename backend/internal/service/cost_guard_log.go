package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"go.uber.org/zap"
)

type costGuardEstimateDetails struct {
	model                       string
	outputSource                string
	input, output               int
	tokensKnown, historyUsed    bool
	cacheRead, cacheCreation    float64
	textRate, imageRate         float64
	inputPrice, outputPrice     float64
	inputCost, outputCost, cost float64
	longContext                 bool
}

// CostGuardCheck is owned by the synchronous admission path, with one log per check.
type CostGuardCheck struct {
	userID, apiKeyID, groupID      int64
	model                          string
	requestTime, checkedAt         time.Time
	estimate                       costGuardEstimateDetails
	inputSource, reason            string
	decision                       string
	balance, inflight, reservation float64
	balanceKnown, inflightKnown    bool
}

type costGuardCheckCtxKey struct{}

// BeginCostGuardCheck does no work when precheck is disabled.
func BeginCostGuardCheck(ctx context.Context, key *APIKey, model string) (context.Context, *CostGuardCheck) {
	if !CostGuardEnabled() || key == nil || key.User == nil {
		return ctx, nil
	}
	now := timezone.Now()
	requestTime, _ := ctx.Value(ctxkey.RequestStartedAt).(time.Time)
	if requestTime.IsZero() {
		requestTime = now
	}
	check := &CostGuardCheck{userID: key.User.ID, apiKeyID: key.ID, model: model, requestTime: requestTime, checkedAt: now, inputSource: "legacy_body_bytes"}
	if key.GroupID != nil {
		check.groupID = *key.GroupID
	}
	return context.WithValue(ctx, costGuardCheckCtxKey{}, check), check
}

func costGuardCheckFromContext(ctx context.Context) *CostGuardCheck {
	check, _ := ctx.Value(costGuardCheckCtxKey{}).(*CostGuardCheck)
	return check
}

func (c *CostGuardCheck) skip(reason string) {
	if c != nil {
		c.decision, c.reason = "skip", reason
	}
}

func (c *CostGuardCheck) setReason(reason string) {
	if c != nil {
		c.reason = reason
	}
}

func (c *CostGuardCheck) recordCandidate(details costGuardEstimateDetails) {
	if c != nil && details.cost > c.estimate.cost {
		c.estimate = details
	}
}

// Finish logs captured values rather than estimating again or rereading balances.
func (c *CostGuardCheck) Finish(ctx context.Context, estimate float64, priced bool, err error) {
	if c == nil {
		return
	}
	decision, reason := c.decision, c.reason
	if err != nil {
		decision = "reject"
		if reason == "" {
			reason = "unpriced_model"
		}
	} else if decision == "" {
		decision = "allow"
		if !priced {
			decision, reason = "skip", "unpriced_model"
		} else if estimate <= 0 {
			decision, reason = "skip", "no_reservation_required"
		}
	}
	d := c.estimate
	cacheSource := "no_history"
	if d.historyUsed {
		cacheSource = "previous_usage_ratio"
	}
	var balance, inflight, available any
	if c.balanceKnown {
		balance = c.balance
	}
	if c.inflightKnown {
		inflight = c.inflight
		if c.balanceKnown {
			available = c.balance - c.inflight
		}
	}
	logger.FromContext(ctx).Info("cost_guard.precheck",
		zap.String("component", "service.cost_guard"),
		zap.Bool(logger.OpsSystemLogSkipField, true),
		zap.Int64("user_id", c.userID), zap.Int64("api_key_id", c.apiKeyID), zap.Int64("group_id", c.groupID),
		zap.String("model", c.model), zap.String("billing_model", d.model),
		zap.String("request_time", c.requestTime.Format(time.RFC3339Nano)),
		zap.String("checked_at", c.checkedAt.Format(time.RFC3339Nano)),
		zap.String("input_source", c.inputSource), zap.Bool("tokens_estimated", d.tokensKnown),
		zap.Int("estimated_input_tokens", d.input), zap.Int("estimated_output_tokens", d.output),
		zap.String("output_source", d.outputSource),
		zap.Float64("estimated_uncached_input_tokens", float64(d.input)-d.cacheRead-d.cacheCreation),
		zap.Float64("estimated_cache_read_tokens", d.cacheRead), zap.Float64("estimated_cache_creation_tokens", d.cacheCreation),
		zap.String("cache_source", cacheSource), zap.Float64("rate_multiplier", d.textRate), zap.Float64("image_rate_multiplier", d.imageRate),
		zap.Float64("effective_input_price_per_token", d.inputPrice), zap.Float64("output_price_per_token", d.outputPrice),
		zap.Bool("model_long_context_multiplier_applied", d.longContext),
		zap.Float64("estimated_input_cost_usd", d.inputCost), zap.Float64("estimated_output_cost_usd", d.outputCost),
		zap.Float64("estimated_cost_usd", estimate), zap.Float64("reservation_usd", c.reservation),
		zap.Any("balance_usd", balance), zap.Any("inflight_usd", inflight), zap.Any("available_balance_usd", available),
		zap.String("decision", decision), zap.String("reason", reason),
	)
}
