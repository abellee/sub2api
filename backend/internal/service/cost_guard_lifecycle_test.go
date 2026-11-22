//go:build unit

package service

import (
	"context"
	"sync"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestCostGuardReservationRejectsLargeCostAndSwitchesOff(t *testing.T) {
	settings := withCostGuard(t, true)
	cache := newMemInflightCache(1)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, MaxReservationUSD: 0.1}
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(svc.Stop)
	user := &User{ID: 1}
	res, err := svc.ReserveInflight(context.Background(), user, nil, nil, 5)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Nil(t, res)
	require.Zero(t, cache.count())
	res, err = svc.ReserveInflight(context.Background(), user, nil, nil, 0.5)
	require.NoError(t, err)
	require.Equal(t, 1, cache.count())
	done := res.Acquire()
	res.HandlerDone()
	require.Equal(t, 1, cache.count())
	done()
	require.Zero(t, cache.count())
	settings.costGuardEnabled.Store(false)
	res, err = svc.ReserveInflight(context.Background(), user, nil, nil, 5)
	require.NoError(t, err, "disabled fork must leave existing behavior intact")
	res.HandlerDone()
	require.Zero(t, cache.count())
}

func TestCostGuardEnablesReservationWithLegacySwitchOff(t *testing.T) {
	withCostGuard(t, true)
	cache := newMemInflightCache(1)
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	require.True(t, svc.InflightReservationEnabled())
	_, err := svc.ReserveInflight(context.Background(), &User{ID: 1}, nil, nil, 5)
	require.ErrorIs(t, err, ErrInsufficientBalance)
}

func TestCostGuard400kTenConcurrentRequestsAllowFive(t *testing.T) {
	settings := withCostGuard(t, true)
	settings.costGuardErrorResponse.Store(normalizedCostGuardErrorResponse(429, "可用余额不足"))
	billing := costGuardGPT61Billing()
	d := inflightEstimateDeps{billing: billing, resolver: NewModelPricingResolver(nil, billing)}
	key := &APIKey{User: &User{ID: 1}, Group: &Group{RateMultiplier: 0.039, LongContextPricingEnabled: true}}
	req := InflightEstimateRequest{Model: "gpt-6.1-sol", BodyBytes: 400000 * 4}
	estimate := d.estimateOne(context.Background(), key, req.Model, req, 0.039, 0.039)
	require.InDelta(t, 0.13728, estimate, 1e-12)
	cache := newMemInflightCache(0.8)
	svc := NewBillingCacheService(cache, nil, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)
	type outcome struct {
		reservation *InflightReservation
		err         error
	}
	results := make(chan outcome, 10)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := svc.ReserveInflight(context.Background(), key.User, key.Group, nil, estimate)
			results <- outcome{res, err}
		}()
	}
	wg.Wait()
	close(results)
	var accepted []*InflightReservation
	rejected := 0
	for result := range results {
		if result.err != nil {
			var response *CostGuardRejectedError
			require.ErrorAs(t, result.err, &response)
			require.Equal(t, 429, response.StatusCode)
			require.Equal(t, "可用余额不足", response.Message)
			rejected++
		} else {
			accepted = append(accepted, result.reservation)
		}
	}
	require.Len(t, accepted, 5)
	require.Equal(t, 5, rejected)
	for _, res := range accepted {
		res.HandlerDone()
	}
	require.Zero(t, cache.count())
}
