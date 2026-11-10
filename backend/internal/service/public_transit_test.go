//go:build unit

package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/stretchr/testify/require"
)

func TestBuildPublicTransitGroups_FiltersExclusiveGroupsAndExportsPricing(t *testing.T) {
	configuredGroups := []Group{
		{
			ID:               10,
			Name:             "public-pro",
			Platform:         "anthropic",
			SubscriptionType: "standard",
			RateMultiplier:   1.25,
			Status:           StatusActive,
		},
		{
			ID:             11,
			Name:           "private-vip",
			Platform:       "anthropic",
			RateMultiplier: 0.8,
			Status:         StatusActive,
			IsExclusive:    true,
		},
	}
	channels := []AvailableChannel{{
		ID:     1,
		Name:   "primary",
		Status: StatusActive,
		Groups: []AvailableGroupRef{
			{
				ID:               10,
				Name:             "public-pro",
				Platform:         "anthropic",
				SubscriptionType: "standard",
				RateMultiplier:   1.25,
			},
			{
				ID:             11,
				Name:           "private-vip",
				Platform:       "anthropic",
				RateMultiplier: 0.8,
				IsExclusive:    true,
			},
		},
		SupportedModels: []SupportedModel{{
			Name:          "claude-sonnet-4",
			Platform:      "anthropic",
			PricingSource: ModelPriceSourceCustom,
			CatalogSource: ModelCatalogSourceChannel,
			Pricing: &ChannelModelPricing{
				Platform:        "anthropic",
				Models:          []string{"claude-sonnet-4"},
				BillingMode:     BillingModeToken,
				InputPrice:      testPtrFloat64(3e-6),
				OutputPrice:     testPtrFloat64(1.5e-5),
				CacheWritePrice: testPtrFloat64(3.75e-6),
				CacheReadPrice:  testPtrFloat64(3e-7),
			},
		}},
	}}

	cacheUsage := map[int64]PublicTransitCacheUsage{
		10: publicCacheUsageFromSummary(usagestats.GroupCacheUsageSummary{
			GroupID: 10,
			Last24h: usagestats.GroupCacheUsageWindow{
				InputTokens:         100,
				CacheCreationTokens: 20,
				CacheReadTokens:     30,
				CacheHitRate:        20,
			},
			Last7d: usagestats.GroupCacheUsageWindow{
				InputTokens:         400,
				CacheCreationTokens: 80,
				CacheReadTokens:     120,
				CacheHitRate:        20,
			},
			Total: usagestats.GroupCacheUsageWindow{
				InputTokens:         900,
				CacheCreationTokens: 100,
				CacheReadTokens:     1000,
				CacheHitRate:        50,
			},
		}),
	}
	groups := buildPublicTransitGroups(configuredGroups, channels, cacheUsage, nil)

	require.Len(t, groups, 1)
	require.Equal(t, "public-pro", groups[0].Name)
	require.False(t, groups[0].IsExclusive)
	require.InDelta(t, 1.25, groups[0].RateMultiplier, 1e-12)
	require.Equal(t, int64(30), groups[0].CacheUsage.Last24h.CacheReadTokens)
	require.InDelta(t, 20, groups[0].CacheUsage.Last24h.CacheHitRate, 1e-12)
	require.Equal(t, int64(120), groups[0].CacheUsage.Last7d.CacheReadTokens)
	require.InDelta(t, 50, groups[0].CacheUsage.Total.CacheHitRate, 1e-12)
	require.Len(t, groups[0].Models, 1)

	model := groups[0].Models[0]
	require.Equal(t, "claude-sonnet-4", model.StandardModel)
	require.Equal(t, "anthropic", model.Platform)
	require.Equal(t, string(BillingModeToken), model.BillingMode)
	require.Equal(t, ModelPriceSourceCustom, model.PriceSource)
	require.Equal(t, ModelCatalogSourceChannel, model.CatalogSource)
	require.NotNil(t, model.Price)
	require.InDelta(t, 3e-6, *model.Price.InputUSDPerToken, 1e-12)
	require.InDelta(t, 1.5e-5, *model.Price.OutputUSDPerToken, 1e-12)
	require.True(t, hasCachePricing(groups))
}

func TestBuildPublicTransitGroups_ExportsConfiguredGroupsWithoutAvailableChannels(t *testing.T) {
	configuredGroups := []Group{
		{
			ID:               1,
			Name:             "gpt pro号池",
			Platform:         "openai",
			SubscriptionType: "standard",
			RateMultiplier:   0.2,
			Status:           StatusActive,
		},
		{
			ID:             2,
			Name:           "disabled",
			Platform:       "openai",
			RateMultiplier: 9,
			Status:         StatusDisabled,
		},
		{
			ID:             3,
			Name:           "exclusive",
			Platform:       "openai",
			RateMultiplier: 0.1,
			Status:         StatusActive,
			IsExclusive:    true,
		},
	}

	groups := buildPublicTransitGroups(configuredGroups, nil, nil, nil)

	require.Len(t, groups, 1)
	require.Equal(t, "gpt pro号池", groups[0].Name)
	require.Equal(t, "openai", groups[0].Platform)
	require.InDelta(t, 0.2, groups[0].RateMultiplier, 1e-12)
	require.Equal(t, "last_24h", groups[0].CacheUsage.Last24h.Period)
	require.Zero(t, groups[0].CacheUsage.Last24h.CacheHitRate)
	require.Empty(t, groups[0].Models)
}

func TestBuildPublicTransitGroups_ExportsEnabledGroupModelsList(t *testing.T) {
	groups := buildPublicTransitGroups([]Group{{
		ID:             1,
		Name:           "gpt free号池",
		Platform:       "openai",
		RateMultiplier: 0.1,
		Status:         StatusActive,
		ModelAllowlist: GroupModelAllowlist{
			Enabled: true,
			Models:  []string{"gpt-5.5", "gpt-image-2", "claude-*"},
		},
	}}, nil, nil, nil)

	require.Len(t, groups, 1)
	require.Len(t, groups[0].Models, 2)
	require.Equal(t, "gpt-5.5", groups[0].Models[0].StandardModel)
	require.Equal(t, ModelCatalogSourceGroupModelsList, groups[0].Models[0].CatalogSource)
	require.Equal(t, ModelPriceSourceUnknown, groups[0].Models[0].PriceSource)
}

func TestToPublicTransitModel_NormalizesImageModeToPerRequest(t *testing.T) {
	price := 0.134
	group := Group{
		Platform:     "openai",
		ImagePrice1K: &price,
		ImagePrice2K: testPtrFloat64(0.201),
		ImagePrice4K: testPtrFloat64(0.268),
	}
	model := toPublicTransitModel(SupportedModel{
		Name:          "gpt-image-2",
		Platform:      "openai",
		PricingSource: ModelPriceSourceCustom,
		CatalogSource: ModelCatalogSourceChannel,
		Pricing: &ChannelModelPricing{
			BillingMode:     BillingModeImage,
			PerRequestPrice: &price,
		},
	}, group)

	require.Equal(t, string(BillingModePerRequest), model.BillingMode)
	require.NotNil(t, model.Price)
	require.Equal(t, &price, model.Price.PerRequestUSD)
	require.InDelta(t, 0.134, *model.Price.ImageSizePrices["1k"], 1e-12)
	require.InDelta(t, 0.201, *model.Price.ImageSizePrices["2k"], 1e-12)
	require.InDelta(t, 0.268, *model.Price.ImageSizePrices["4k"], 1e-12)
}

func TestBuildPublicTransitPassiveDisclosureAggregatesModelsAndGroups(t *testing.T) {
	now := time.Now().UTC()
	bucketA := now.Add(-time.Hour).Truncate(time.Hour)
	bucketB := now.Truncate(time.Hour)
	rows := []PublicTransitPassiveAggregate{
		{BucketStart: &bucketA, Platform: "openai", GroupName: "gpt", Model: "gpt-5", RequestCount: 3, SuccessCount: 2, ErrorCount: 1, TotalLatencyMs: 300, LatencySamples: 2, TotalTTFTMs: 80, TTFTSamples: 2, InputTokens: 100, CacheRead: 50, LastRequestAt: &now},
		{BucketStart: &bucketB, Platform: "openai", GroupName: "gpt", Model: "gpt-5", RequestCount: 2, SuccessCount: 2, TotalLatencyMs: 100, LatencySamples: 2, TotalTTFTMs: 20, TTFTSamples: 1, InputTokens: 50, CacheCreate: 25, CacheRead: 25, LastRequestAt: &now},
	}
	out := buildPublicTransitPassiveDisclosure(rows, PublicTransitPassiveWindow{Period: "last_24h", BucketSeconds: 3600})
	require.Len(t, out.Models, 1)
	require.Equal(t, "gpt", out.Models[0].GroupName)
	require.Equal(t, int64(5), out.Models[0].RequestCount)
	require.InDelta(t, 0.8, out.Models[0].SuccessRate, 1e-12)
	require.Equal(t, 100, *out.Models[0].AvgLatencyMs)
	require.Equal(t, int64(5), out.Window.RequestCount)
	require.Equal(t, 100, *out.Window.AvgLatencyMs)
	require.Len(t, out.Groups, 1)
	require.Equal(t, "gpt", out.Groups[0].Name)
	require.Len(t, out.Groups[0].Buckets, 2)
	require.Equal(t, bucketA.Format(time.RFC3339), out.Groups[0].Buckets[0].Start)
	require.InDelta(t, 50.0/150.0, out.Groups[0].Buckets[0].CacheHitRate, 1e-12)
}

func TestPublicTransitLatencyResult(t *testing.T) {
	for _, tt := range []struct {
		name       string
		firstToken bool
		input      float64
		want       float64 // Negative means a random value in the target range.
	}{
		{"ttft zero", true, 0, -1},
		{"ttft below range", true, 2999, -1},
		{"ttft lower bound", true, 3000, 3000},
		{"ttft within range", true, 3500, 3500},
		{"ttft upper bound", true, 4000, 4000},
		{"ttft above range", true, 4001, -1},
		{"ttft divided below range", true, 8999, -1},
		{"ttft divided lower bound", true, 9000, 3000},
		{"ttft divided within range", true, 10500, 3500},
		{"ttft divided upper bound", true, 12000, 4000},
		{"ttft divided above range", true, 12001, -1},
		{"duration zero", false, 0, -1},
		{"duration below range", false, 5999, -1},
		{"duration lower bound", false, 6000, 6000},
		{"duration within range", false, 7000, 7000},
		{"duration upper bound", false, 8000, 8000},
		{"duration below division threshold", false, 9999, -1},
		{"duration division threshold", false, 10000, -1},
		{"duration divided lower bound", false, 12000, 6000},
		{"duration divided within range", false, 14000, 7000},
		{"duration divided upper bound", false, 16000, 8000},
		{"duration divided above range", false, 16001, -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			minMs, maxMs := 6000.0, 8000.0
			if tt.firstToken {
				minMs, maxMs = 3000, 4000
			}
			for range 32 {
				input := tt.input
				got := publicTransitLatencyResult(&input, tt.firstToken)
				require.Equal(t, tt.input, input, "must not alter source measurements")
				require.GreaterOrEqual(t, *got, minMs)
				require.LessOrEqual(t, *got, maxMs)
				if tt.want >= 0 {
					require.Equal(t, tt.want, *got)
				}
				intInput := int(tt.input)
				intResult := publicTransitLatencyResult(&intInput, tt.firstToken)
				require.GreaterOrEqual(t, *intResult, int(minMs))
				require.LessOrEqual(t, *intResult, int(maxMs))
				if tt.want >= 0 {
					require.Equal(t, int(tt.want), *intResult)
				}
			}
		})
	}
	require.Nil(t, publicTransitLatencyResult[int](nil, true))
	require.Nil(t, publicTransitLatencyResult[float64](nil, false))
	fraction := 3500.75
	require.Equal(t, fraction, *publicTransitLatencyResult(&fraction, true))
}

func TestPublicTransitSuccessRateResult(t *testing.T) {
	for _, input := range []float64{0, 0.5, 0.899999, 0.9, 0.91, 0.94, 0.949999, 0.95, 0.99, 1} {
		for range 32 {
			got := publicTransitSuccessRateResult(input)
			if input < 0.95 {
				require.Contains(t, []float64{0.95, 0.96, 0.97, 0.98, 0.99}, got)
			} else {
				require.Equal(t, input, got)
			}
		}
	}
}

func TestAdjustPublicTransitPassiveResultsCoversExportedFields(t *testing.T) {
	now := time.Now().UTC()
	passive := buildPublicTransitPassiveDisclosure([]PublicTransitPassiveAggregate{{
		BucketStart: &now, Platform: "openai", GroupName: "gpt", Model: "gpt-5",
		RequestCount: 50, SuccessCount: 40, ErrorCount: 10,
		TotalLatencyMs: 700000, LatencySamples: 50, TotalTTFTMs: 525000, TTFTSamples: 50,
	}}, PublicTransitPassiveWindow{Period: "last_24h"})
	metric := ChannelMonitorV2Metric{
		RequestCount: 50, SuccessRequests: 40, ErrorRequests: 10, SuccessRate: 0.8, CacheRate: 0.5,
		TTFT:     ChannelMonitorV2Latency{AvgMs: publicTransitPtr(10500.0), P50Ms: publicTransitPtr(int64(5000)), SampleCount: 50},
		Duration: ChannelMonitorV2Latency{AvgMs: publicTransitPtr(14000.0), P50Ms: publicTransitPtr(int64(9000)), SampleCount: 50},
	}
	health := ChannelMonitorV2HealthFor(metric)
	snapshot := &ChannelMonitorV2Snapshot{Metrics: metric, Health: health}
	matrix := &ChannelMonitorV2Matrix{Items: []ChannelMonitorV2MatrixRow{{
		Platform: "openai", GroupName: "gpt", Metrics: metric, Health: health,
		Buckets: []ChannelMonitorV2TrendPoint{{BucketStart: now, Metrics: metric, Health: health}},
	}}}
	modelMatrix := &ChannelMonitorV2Matrix{Items: []ChannelMonitorV2MatrixRow{{
		Platform: "openai", GroupName: "gpt", Model: "gpt-5", Metrics: metric, Health: health,
	}}}
	augmentPublicTransitPassiveDisclosure(&passive, snapshot, matrix, modelMatrix)
	adjustPublicTransitPassiveResults(&passive)

	encoded, err := json.Marshal(passive)
	require.NoError(t, err)
	var response any
	require.NoError(t, json.Unmarshal(encoded, &response))
	flatAverages, nestedAverages, successRates := 0, 0, 0
	var check func(any)
	check = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if rate, ok := value["success_rate"].(float64); ok {
				requests := value["request_count"].(float64)
				successKey, errorKey := "success_count", "error_count"
				if _, ok := value["success_requests"]; ok {
					successKey, errorKey = "success_requests", "error_requests"
					require.InDelta(t, 1-rate, value["error_rate"], 1e-12)
				}
				require.Equal(t, 50.0, requests)
				successes := math.Round(requests * rate)
				require.Equal(t, successes, value[successKey])
				require.Equal(t, requests-successes, value[errorKey])
				if metrics, ok := value["metrics"].(map[string]any); ok {
					require.Equal(t, rate, metrics["success_rate"])
					require.Equal(t, successes, metrics["success_requests"])
				}
			}
			for name, child := range value {
				switch name {
				case "success_rate":
					require.Contains(t, []float64{0.95, 0.96, 0.97, 0.98, 0.99}, child)
					successRates++
				case "avg_ttft_ms":
					require.Equal(t, 3500.0, child)
					flatAverages++
				case "avg_latency_ms":
					require.Equal(t, 7000.0, child)
					flatAverages++
				case "ttft", "duration":
					latency, ok := child.(map[string]any)
					if !ok {
						continue
					}
					wantAvg, wantP50 := 3500.0, 5000.0
					if name == "duration" {
						wantAvg, wantP50 = 7000, 9000
					}
					require.Equal(t, wantAvg, latency["avg_ms"])
					require.Equal(t, wantP50, latency["p50_ms"])
					nestedAverages++
				}
				check(child)
			}
		case []any:
			for _, child := range value {
				check(child)
			}
		}
	}
	check(response)
	require.Equal(t, 8, flatAverages)
	require.Equal(t, 12, nestedAverages)
	require.Equal(t, 10, successRates)
	require.Equal(t, int64(50), passive.Window.RequestCount)
	require.Equal(t, int64(math.Round(50*passive.Window.SuccessRate)), passive.Window.SuccessCount)
	require.Equal(t, int64(50)-passive.Window.SuccessCount, passive.Window.ErrorCount)
	require.Equal(t, *passive.Groups[0].Metrics, passive.Matrix.Items[0].Metrics)
	require.Equal(t, *passive.Groups[0].Buckets[0].Metrics, passive.Matrix.Items[0].Buckets[0].Metrics)
	require.Equal(t, 0.5, passive.Matrix.Items[0].Metrics.CacheRate)
	require.Equal(t, 10500.0, *snapshot.Metrics.TTFT.AvgMs)
	require.Equal(t, 14000.0, *snapshot.Metrics.Duration.AvgMs)
	require.Equal(t, 0.8, snapshot.Metrics.SuccessRate)
	require.Equal(t, int64(40), snapshot.Metrics.SuccessRequests)
	require.Equal(t, int64(10), snapshot.Metrics.ErrorRequests)

	empty := PublicTransitPassiveDisclosure{}
	adjustPublicTransitPassiveResults(&empty)
	require.Nil(t, empty.Window.AvgTTFTMs)
	require.Nil(t, empty.Window.AvgLatencyMs)
	require.Nil(t, empty.Window.Metrics)
	require.Nil(t, empty.Matrix)
	require.Zero(t, empty.Window.SuccessRate)
}

func TestAdjustPublicTransitPassiveResultsAdjustsSmallSamples(t *testing.T) {
	for _, tt := range []struct {
		name          string
		requests      int64
		minimumSample int64
		withHealth    bool
	}{
		{"single request", 1, 50, false},
		{"few requests", 7, 50, false},
		{"below default threshold", 49, 50, false},
		{"health below default threshold", 49, 50, true},
		{"custom threshold", 99, 100, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			metric := ChannelMonitorV2Metric{
				RequestCount: tt.requests, SuccessRate: 0.8, InputTokens: 100,
				TTFT:     ChannelMonitorV2Latency{AvgMs: publicTransitPtr(15000.0)},
				Duration: ChannelMonitorV2Latency{AvgMs: publicTransitPtr(20000.0)},
			}
			var health *ChannelMonitorV2Health
			if tt.withHealth {
				thresholds := DefaultChannelMonitorV2HealthThresholds()
				thresholds.MinimumSample = tt.minimumSample
				health = publicTransitPtr(ChannelMonitorV2HealthForWithThresholds(metric, thresholds))
			}
			metric = publicTransitMetric(metric)
			passive := PublicTransitPassiveDisclosure{
				Window: PublicTransitPassiveWindow{RequestCount: tt.requests, SuccessRate: 0.8, AvgTTFTMs: publicTransitPtr(15000), AvgLatencyMs: publicTransitPtr(20000), Metrics: publicTransitPtr(metric), Health: health},
				Models: []PublicTransitPassiveModel{{RequestCount: tt.requests, SuccessRate: 0.8, AvgTTFTMs: publicTransitPtr(15000), AvgLatencyMs: publicTransitPtr(20000), Metrics: publicTransitPtr(metric), Health: health}},
				Groups: []PublicTransitPassiveGroup{{RequestCount: tt.requests, SuccessRate: 0.8, AvgTTFTMs: publicTransitPtr(15000), AvgLatencyMs: publicTransitPtr(20000), Metrics: publicTransitPtr(metric), Health: health,
					Buckets: []PublicTransitPassiveBucket{{RequestCount: tt.requests, SuccessRate: 0.8, AvgTTFTMs: publicTransitPtr(15000), AvgLatencyMs: publicTransitPtr(20000), Metrics: publicTransitPtr(metric), Health: health}},
				}},
			}
			matrixHealth := ChannelMonitorV2Health{}
			if health != nil {
				matrixHealth = *health
			}
			passive.Matrix = &PublicTransitPassiveMatrix{Items: []ChannelMonitorV2MatrixRow{{Metrics: metric, Health: matrixHealth,
				Buckets: []ChannelMonitorV2TrendPoint{{Metrics: metric, Health: matrixHealth}},
			}}}
			adjustPublicTransitPassiveResults(&passive)
			for _, metrics := range []*ChannelMonitorV2Metric{
				passive.Window.Metrics, passive.Models[0].Metrics, passive.Groups[0].Metrics,
				passive.Groups[0].Buckets[0].Metrics, &passive.Matrix.Items[0].Metrics,
				&passive.Matrix.Items[0].Buckets[0].Metrics,
			} {
				require.Contains(t, []float64{0.95, 0.96, 0.97, 0.98, 0.99}, metrics.SuccessRate)
				require.Equal(t, tt.requests, metrics.RequestCount)
				require.Equal(t, int64(math.Round(float64(tt.requests)*metrics.SuccessRate)), metrics.SuccessRequests)
				require.Equal(t, tt.requests-metrics.SuccessRequests, metrics.ErrorRequests)
				require.InDelta(t, 1-metrics.SuccessRate, metrics.ErrorRate, 1e-12)
				require.Zero(t, metrics.InputTokens)
				require.GreaterOrEqual(t, *metrics.TTFT.AvgMs, 3000.0)
				require.LessOrEqual(t, *metrics.TTFT.AvgMs, 4000.0)
			}
			require.Equal(t, passive.Window.SuccessRate, passive.Window.Metrics.SuccessRate)
			require.Equal(t, passive.Window.SuccessCount, passive.Window.Metrics.SuccessRequests)
			require.Equal(t, tt.requests, passive.Window.SuccessCount+passive.Window.ErrorCount)
		})
	}
}

func TestAdjustPublicTransitPassiveResultsLeavesNoTrafficUnchanged(t *testing.T) {
	metric := ChannelMonitorV2Metric{}
	passive := PublicTransitPassiveDisclosure{
		Window: PublicTransitPassiveWindow{Metrics: &metric},
		Models: []PublicTransitPassiveModel{{Metrics: &metric}},
		Groups: []PublicTransitPassiveGroup{{Metrics: &metric, Buckets: []PublicTransitPassiveBucket{{Metrics: &metric}}}},
		Matrix: &PublicTransitPassiveMatrix{Items: []ChannelMonitorV2MatrixRow{{Buckets: []ChannelMonitorV2TrendPoint{{}}}}},
	}
	before, err := json.Marshal(passive)
	require.NoError(t, err)
	adjustPublicTransitPassiveResults(&passive)
	after, err := json.Marshal(passive)
	require.NoError(t, err)
	require.JSONEq(t, string(before), string(after))
}
