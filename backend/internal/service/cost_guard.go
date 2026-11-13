package service

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Wei-Shaw/sub2api/cost-guard-sdk/costguard"
	"github.com/Wei-Shaw/sub2api/internal/config"
)

const (
	SettingKeyCostGuardEnabled         = "cost_guard_enabled"
	SettingKeyCostGuardErrorStatusCode = "cost_guard_error_status_code"
	SettingKeyCostGuardErrorMessage    = "cost_guard_error_message"
	DefaultCostGuardErrorStatusCode    = http.StatusForbidden
	DefaultCostGuardErrorMessage       = "Insufficient balance for the estimated request cost."
)

type costGuardErrorResponse struct {
	statusCode int
	message    string
}

func normalizedCostGuardErrorResponse(status int, message string) *costGuardErrorResponse {
	if status < 400 || status > 599 {
		status = DefaultCostGuardErrorStatusCode
	}
	if strings.TrimSpace(message) == "" {
		message = DefaultCostGuardErrorMessage
	}
	return &costGuardErrorResponse{statusCode: status, message: message}
}

// CostGuardRejectedError preserves the configured response at rejection time.
type CostGuardRejectedError struct {
	Unpriced   bool // Missing pricing is not a user balance shortage.
	StatusCode int
	Message    string
}

func (e *CostGuardRejectedError) Error() string { return e.Message }
func (e *CostGuardRejectedError) Unwrap() error { return ErrInsufficientBalance }

// CostGuardRejection leaves the host's original error intact when disabled.
func CostGuardRejection() error {
	s := costGuardDefault.Load()
	if s == nil || s.settings == nil || !s.settings.costGuardEnabled.Load() {
		return ErrInsufficientBalance
	}
	response := s.settings.costGuardErrorResponse.Load()
	if response == nil {
		response = normalizedCostGuardErrorResponse(0, "")
	}
	return &CostGuardRejectedError{StatusCode: response.statusCode, Message: response.message}
}

func CostGuardUnpricedRejection() error {
	err := CostGuardRejection()
	if rejection, ok := err.(*CostGuardRejectedError); ok {
		rejection.Unpriced = true
		return rejection
	}
	return &CostGuardRejectedError{StatusCode: http.StatusForbidden, Message: err.Error(), Unpriced: true}
}

func IsUserBalanceError(err error) bool {
	var rejection *CostGuardRejectedError
	if errors.As(err, &rejection) && rejection.Unpriced {
		return false
	}
	return errors.Is(err, ErrInsufficientBalance)
}

var costGuardDefault atomic.Pointer[CostGuardService]

// CostGuardService keeps optional request estimates entirely in memory.
type CostGuardService struct {
	settings  *SettingService
	history   *costguard.History
	stopCh    chan struct{}
	startOnce sync.Once
	stopOnce  sync.Once
}

func NewCostGuardService(settings *SettingService) *CostGuardService {
	s := &CostGuardService{settings: settings, history: costguard.NewHistory(), stopCh: make(chan struct{})}
	costGuardDefault.Store(s)
	return s
}

func (s *CostGuardService) Start(ctx context.Context) error {
	if s == nil || s.settings == nil {
		return nil
	}
	s.startOnce.Do(func() {
		s.refresh(ctx)
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-s.stopCh:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					s.refresh(ctx)
				}
			}
		}()
	})
	return nil
}

func (s *CostGuardService) refresh(ctx context.Context) {
	if s.settings.settingRepo == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Serialize with setting saves so an old background read cannot undo a save.
	s.settings.costGuardMu.Lock()
	defer s.settings.costGuardMu.Unlock()
	values, err := s.settings.settingRepo.GetMultiple(ctx, []string{SettingKeyCostGuardEnabled, SettingKeyCostGuardErrorStatusCode, SettingKeyCostGuardErrorMessage})
	if err == nil {
		status, _ := strconv.Atoi(values[SettingKeyCostGuardErrorStatusCode])
		s.settings.costGuardErrorResponse.Store(normalizedCostGuardErrorResponse(status, values[SettingKeyCostGuardErrorMessage]))
		s.settings.costGuardEnabled.Store(values[SettingKeyCostGuardEnabled] == "true")
	}
}

func (s *CostGuardService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() { close(s.stopCh); costGuardDefault.CompareAndSwap(s, nil) })
}

// CostGuardEnabled only reads an atomic value; it never loads settings from DB.
func CostGuardEnabled() bool {
	s := costGuardDefault.Load()
	return s != nil && s.settings != nil && s.settings.costGuardEnabled.Load()
}

func costGuardLoggingEnabled() bool {
	s := costGuardDefault.Load()
	return s != nil && s.settings != nil
}

func costGuardHistoryKey(apiKey *APIKey, model string) costguard.HistoryKey {
	k := costguard.HistoryKey{APIKeyID: apiKey.ID, Model: model}
	if apiKey.GroupID != nil {
		k.GroupID = *apiKey.GroupID
	}
	return k
}

func costGuardInputPrice(pricing *ModelPricing, apiKey *APIKey, model string) float64 {
	return costGuardInputEstimate(pricing, apiKey, model).Price
}

func costGuardInputEstimate(pricing *ModelPricing, apiKey *APIKey, model string) costguard.InputEstimate {
	if pricing == nil {
		return costguard.InputEstimate{}
	}
	s := costGuardDefault.Load()
	if s == nil || apiKey == nil {
		return costguard.InputEstimate{Price: pricing.InputPricePerToken}
	}
	return s.history.EstimateInputPrice(costGuardHistoryKey(apiKey, model), costguard.TokenPrices{
		Input: pricing.InputPricePerToken, CacheRead: pricing.CacheReadPricePerToken, CacheCreation: pricing.CacheCreationPricePerToken,
	})
}

func costGuardObserveUsage(apiKey *APIKey, usage *UsageLog) {
	s := costGuardDefault.Load()
	if s == nil || apiKey == nil || usage == nil {
		return
	}
	model := usage.RequestedModel
	if model == "" {
		model = usage.Model
	}
	s.history.Observe(costGuardHistoryKey(apiKey, model), usage.InputTokens, usage.CacheReadTokens, usage.CacheCreationTokens)
	if usage.Model != model {
		s.history.Observe(costGuardHistoryKey(apiKey, usage.Model), usage.InputTokens, usage.CacheReadTokens, usage.CacheCreationTokens)
	}
}

func costGuardRejects(balance, estimate float64) bool {
	return CostGuardEnabled() && !math.IsNaN(balance) && !math.IsInf(balance, 0) && validCost(estimate) && estimate > balance
}

// Mirror billing's long-context rules without changing the shared price card.
func costGuardLongContextPricing(billing *BillingService, resolved *ResolvedPricing, pricing *ModelPricing, apiKey *APIKey, inputTokens int) *ModelPricing {
	if billing == nil || pricing == nil {
		return pricing
	}
	enabled := apiKey == nil || apiKey.Group == nil || apiKey.Group.LongContextPricingEnabled
	if resolved != nil {
		enabled = resolved.longContextPricingEnabled && len(resolved.Intervals) == 0
	}
	if !enabled || !billing.shouldApplySessionLongContextPricing(UsageTokens{InputTokens: inputTokens}, pricing) {
		return pricing
	}
	adjusted := *pricing
	inputMultiplier := longContextMultiplierOrOne(pricing.LongContextInputMultiplier)
	adjusted.InputPricePerToken *= inputMultiplier
	adjusted.CacheReadPricePerToken *= inputMultiplier
	adjusted.CacheCreationPricePerToken *= inputMultiplier
	adjusted.OutputPricePerToken *= longContextMultiplierOrOne(pricing.LongContextOutputMultiplier)
	return &adjusted
}

func costGuardTokenCounts(billing *BillingService, cfg config.InflightReservationConfig, model string, bodyBytes, maxTokens int) (int, int) {
	if !costGuardLoggingEnabled() {
		return tokenCounts(cfg, bodyBytes, maxTokens)
	}
	// Use this request's input without truncating its estimate to the old cap.
	input := max(bodyBytes, 0) / inflightInputBytesPerTokenEstimate
	modelLimit := 0
	if billing != nil && billing.pricingService != nil {
		if p := billing.pricingService.GetModelPricing(model); p != nil {
			modelLimit = p.MaxOutputTokens
		}
	}
	output := maxTokens
	if output <= 0 {
		output = modelLimit
	}
	if output <= 0 {
		output = cfg.DefaultMaxTokens
	}
	if output <= 0 {
		output = defaultInflightDefaultMaxTokens
	}
	if modelLimit > 0 && output > modelLimit {
		output = modelLimit
	}
	return input, output
}
