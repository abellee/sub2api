package dto

import (
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageLogFromService_IncludesOpenAIWSMode(t *testing.T) {
	t.Parallel()

	wsLog := &service.UsageLog{
		RequestID:    "req_1",
		Model:        "gpt-5.3-codex",
		OpenAIWSMode: true,
	}
	httpLog := &service.UsageLog{
		RequestID:    "resp_1",
		Model:        "gpt-5.3-codex",
		OpenAIWSMode: false,
	}

	require.True(t, UsageLogFromService(wsLog).OpenAIWSMode)
	require.False(t, UsageLogFromService(httpLog).OpenAIWSMode)
	require.True(t, UsageLogFromServiceAdmin(wsLog).OpenAIWSMode)
	require.False(t, UsageLogFromServiceAdmin(httpLog).OpenAIWSMode)
}

func TestUsageLogFromService_PreservesNativeCompactionAndStream(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID:          "resp_compaction",
		Model:              "gpt-5.6-sol",
		RequestType:        service.RequestTypeStream,
		Stream:             true,
		NativeCompactionV2: true,
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)
	require.Equal(t, "stream", userDTO.RequestType)
	require.True(t, userDTO.Stream)
	require.True(t, userDTO.NativeCompactionV2)
	require.Equal(t, "stream", adminDTO.RequestType)
	require.True(t, adminDTO.Stream)
	require.True(t, adminDTO.NativeCompactionV2)
}

func TestUsageLogFromService_PrefersRequestTypeForLegacyFields(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID:    "req_2",
		Model:        "gpt-5.3-codex",
		RequestType:  service.RequestTypeWSV2,
		Stream:       false,
		OpenAIWSMode: false,
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.Equal(t, "ws_v2", userDTO.RequestType)
	require.True(t, userDTO.Stream)
	require.True(t, userDTO.OpenAIWSMode)
	require.Equal(t, "ws_v2", adminDTO.RequestType)
	require.True(t, adminDTO.Stream)
	require.True(t, adminDTO.OpenAIWSMode)
}

func TestUsageCleanupTaskFromService_RequestTypeMapping(t *testing.T) {
	t.Parallel()

	requestType := int16(service.RequestTypeStream)
	task := &service.UsageCleanupTask{
		ID:     1,
		Status: service.UsageCleanupStatusPending,
		Filters: service.UsageCleanupFilters{
			RequestType: &requestType,
		},
	}

	dtoTask := UsageCleanupTaskFromService(task)
	require.NotNil(t, dtoTask)
	require.NotNil(t, dtoTask.Filters.RequestType)
	require.Equal(t, "stream", *dtoTask.Filters.RequestType)
}

func TestRequestTypeStringPtrNil(t *testing.T) {
	t.Parallel()
	require.Nil(t, requestTypeStringPtr(nil))
}

func TestUsageLogFromService_IncludesServiceTierForUserAndAdmin(t *testing.T) {
	t.Parallel()

	serviceTier := "priority"
	inboundEndpoint := "/v1/chat/completions"
	upstreamEndpoint := "/v1/responses"
	log := &service.UsageLog{
		RequestID:             "req_3",
		Model:                 "gpt-5.4",
		ServiceTier:           &serviceTier,
		InboundEndpoint:       &inboundEndpoint,
		UpstreamEndpoint:      &upstreamEndpoint,
		AccountRateMultiplier: f64Ptr(1.5),
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.NotNil(t, userDTO.ServiceTier)
	require.Equal(t, serviceTier, *userDTO.ServiceTier)
	require.NotNil(t, userDTO.InboundEndpoint)
	require.Equal(t, inboundEndpoint, *userDTO.InboundEndpoint)
	require.Nil(t, userDTO.UpstreamEndpoint)
	require.NotNil(t, adminDTO.ServiceTier)
	require.Equal(t, serviceTier, *adminDTO.ServiceTier)
	require.NotNil(t, adminDTO.InboundEndpoint)
	require.Equal(t, inboundEndpoint, *adminDTO.InboundEndpoint)
	require.NotNil(t, adminDTO.UpstreamEndpoint)
	require.Equal(t, upstreamEndpoint, *adminDTO.UpstreamEndpoint)
	require.NotNil(t, adminDTO.AccountRateMultiplier)
	require.InDelta(t, 1.5, *adminDTO.AccountRateMultiplier, 1e-12)
}

func TestUsageLogFromService_UsesRequestedModelAndKeepsUpstreamAdminOnly(t *testing.T) {
	t.Parallel()

	upstreamModel := "claude-sonnet-4-20250514"
	upstreamResponseModel := "claude-sonnet-4-20250513"
	upstreamModelMismatch := true
	log := &service.UsageLog{
		RequestID:             "req_4",
		Model:                 upstreamModel,
		RequestedModel:        "claude-sonnet-4",
		UpstreamModel:         &upstreamModel,
		UpstreamResponseModel: &upstreamResponseModel,
		UpstreamModelMismatch: &upstreamModelMismatch,
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.Equal(t, "claude-sonnet-4", userDTO.Model)
	require.Equal(t, "claude-sonnet-4", adminDTO.Model)

	userJSON, err := json.Marshal(userDTO)
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "upstream_model")
	require.NotContains(t, string(userJSON), "upstream_response_model")
	require.NotContains(t, string(userJSON), "upstream_model_mismatch")

	adminJSON, err := json.Marshal(adminDTO)
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"upstream_model":"claude-sonnet-4-20250514"`)
	require.Contains(t, string(adminJSON), `"upstream_response_model":"claude-sonnet-4-20250513"`)
	require.Contains(t, string(adminJSON), `"upstream_model_mismatch":true`)
}

func TestUsageLogFromService_KeepsUserBillingAndIPWithoutAdminCostFields(t *testing.T) {
	t.Parallel()

	ipAddress := "203.0.113.10"
	accountRateMultiplier := 1.5
	accountStatsCost := 0.21
	log := &service.UsageLog{
		RequestID:             "req_user_visible_billing",
		Model:                 "gpt-5.4",
		InputCost:             0.01,
		OutputCost:            0.02,
		CacheCreationCost:     0.03,
		CacheReadCost:         0.04,
		TotalCost:             0.10,
		ActualCost:            0.08,
		RateMultiplier:        0.8,
		IPAddress:             &ipAddress,
		AccountRateMultiplier: &accountRateMultiplier,
		AccountStatsCost:      &accountStatsCost,
	}

	userDTO := UsageLogFromService(log)
	require.Equal(t, 0.01, userDTO.InputCost)
	require.Equal(t, 0.02, userDTO.OutputCost)
	require.Equal(t, 0.03, userDTO.CacheCreationCost)
	require.Equal(t, 0.04, userDTO.CacheReadCost)
	require.Equal(t, 0.10, userDTO.TotalCost)
	require.Equal(t, 0.08, userDTO.ActualCost)
	require.Equal(t, 0.8, userDTO.RateMultiplier)
	require.NotNil(t, userDTO.IPAddress)
	require.Equal(t, ipAddress, *userDTO.IPAddress)

	userJSON, err := json.Marshal(userDTO)
	require.NoError(t, err)
	require.NotContains(t, string(userJSON), "account_rate_multiplier")
	require.NotContains(t, string(userJSON), "account_stats_cost")
	require.NotContains(t, string(userJSON), "account_cost")
}

func TestUsageLogFromService_UsersSeeRequestedReasoningEffortOnly(t *testing.T) {
	t.Parallel()

	requested := "max"
	forwarded := "xhigh"
	log := &service.UsageLog{
		RequestID:                "req_effort",
		Model:                    "gpt-5.4",
		ReasoningEffort:          &forwarded,
		RequestedReasoningEffort: &requested,
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.NotNil(t, userDTO.ReasoningEffort)
	require.Equal(t, requested, *userDTO.ReasoningEffort)
	require.NotNil(t, adminDTO.ReasoningEffort)
	require.Equal(t, requested, *adminDTO.ReasoningEffort)
	require.NotNil(t, adminDTO.UpstreamReasoningEffort)
	require.Equal(t, forwarded, *adminDTO.UpstreamReasoningEffort)

	userJSON, err := json.Marshal(userDTO)
	require.NoError(t, err)
	require.Contains(t, string(userJSON), `"reasoning_effort":"max"`)
	require.NotContains(t, string(userJSON), "upstream_reasoning_effort")
	require.NotContains(t, string(userJSON), "requested_reasoning_effort")

	adminJSON, err := json.Marshal(adminDTO)
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"reasoning_effort":"max"`)
	require.Contains(t, string(adminJSON), `"upstream_reasoning_effort":"xhigh"`)
}

func TestUsageLogFromService_OmitsUpstreamReasoningEffortWhenUnmapped(t *testing.T) {
	t.Parallel()

	effort := "high"
	log := &service.UsageLog{
		RequestID:                "req_effort_same",
		Model:                    "gpt-5.4",
		ReasoningEffort:          &effort,
		RequestedReasoningEffort: &effort,
	}

	adminDTO := UsageLogFromServiceAdmin(log)
	require.NotNil(t, adminDTO.ReasoningEffort)
	require.Equal(t, effort, *adminDTO.ReasoningEffort)
	require.Nil(t, adminDTO.UpstreamReasoningEffort)

	adminJSON, err := json.Marshal(adminDTO)
	require.NoError(t, err)
	require.NotContains(t, string(adminJSON), "upstream_reasoning_effort")
}

func TestUsageLogFromService_FallsBackToLegacyModelWhenRequestedModelMissing(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID: "req_legacy",
		Model:     "claude-3",
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	require.Equal(t, "claude-3", userDTO.Model)
	require.Equal(t, "claude-3", adminDTO.Model)
}

func TestUsageLogFromService_IncludesImageBillingMetadataForUserAndAdmin(t *testing.T) {
	t.Parallel()

	imageSize := "4K"
	inputSize := "1024x1024"
	outputSize := "3840x2160"
	source := "output"
	log := &service.UsageLog{
		RequestID:          "req_image_metadata",
		Model:              "gpt-image-2",
		ImageCount:         2,
		ImageSize:          &imageSize,
		ImageInputSize:     &inputSize,
		ImageOutputSize:    &outputSize,
		ImageSizeSource:    &source,
		ImageSizeBreakdown: map[string]int{"4K": 2},
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	for _, got := range []*UsageLog{userDTO, &adminDTO.UsageLog} {
		require.Equal(t, 2, got.ImageCount)
		require.NotNil(t, got.ImageSize)
		require.Equal(t, imageSize, *got.ImageSize)
		require.NotNil(t, got.ImageInputSize)
		require.Equal(t, inputSize, *got.ImageInputSize)
		require.NotNil(t, got.ImageOutputSize)
		require.Equal(t, outputSize, *got.ImageOutputSize)
		require.NotNil(t, got.ImageSizeSource)
		require.Equal(t, source, *got.ImageSizeSource)
		require.Equal(t, map[string]int{"4K": 2}, got.ImageSizeBreakdown)
	}
}

func TestUsageLogFromService_PreservesHistoricalMissingImageSize(t *testing.T) {
	t.Parallel()

	log := &service.UsageLog{
		RequestID:  "req_legacy_image_missing_size",
		Model:      "gpt-image-2",
		ImageCount: 1,
		ImageSize:  nil,
	}

	dto := UsageLogFromService(log)
	require.Equal(t, 1, dto.ImageCount)
	require.Nil(t, dto.ImageSize)
	require.Nil(t, dto.ImageInputSize)
	require.Nil(t, dto.ImageOutputSize)
	require.Nil(t, dto.ImageSizeSource)
	require.Nil(t, dto.ImageSizeBreakdown)

	body, err := json.Marshal(dto)
	require.NoError(t, err)
	require.Contains(t, string(body), `"image_size":null`)
	require.NotContains(t, string(body), `"image_size":"2K"`)
}

func TestAccountSummaryFromService_IncludesUpstreamDeclaredRateAndBaseURL(t *testing.T) {
	t.Parallel()

	billingRate := 0.35
	account := &service.Account{
		ID:             42,
		Name:           "relay-a",
		RateMultiplier: &billingRate,
		Credentials: map[string]any{
			"base_url":      "https://api.example.com/v1",
			"api_key":       "sk-secret",
			"refresh_token": "rt-secret",
		},
		Extra: map[string]any{
			"upstream_billing_probe": map[string]any{
				"status": "ok",
				"data": map[string]any{
					"billing_scope":             "token",
					"resolved_rate_multiplier":  0.065,
					"peak_rate_enabled":         false,
					"effective_rate_multiplier": 0.065,
				},
			},
		},
	}

	summary := AccountSummaryFromService(account)
	require.NotNil(t, summary)
	require.Equal(t, int64(42), summary.ID)
	require.Equal(t, "relay-a", summary.Name)
	require.NotNil(t, summary.UpstreamRateMultiplier)
	require.InDelta(t, 0.065, *summary.UpstreamRateMultiplier, 1e-12)
	require.Equal(t, "https://api.example.com/v1", summary.BaseURL)

	body, err := json.Marshal(summary)
	require.NoError(t, err)
	require.Contains(t, string(body), `"upstream_rate_multiplier":0.065`)
	require.NotContains(t, string(body), `"rate_multiplier":0.35`)
	require.Contains(t, string(body), `"base_url":"https://api.example.com/v1"`)
	require.NotContains(t, string(body), "sk-secret")
	require.NotContains(t, string(body), "rt-secret")
}

func TestUsageLogFromServiceAdmin_IncludesAccountHoverFields(t *testing.T) {
	t.Parallel()

	billingRate := 1.25
	log := &service.UsageLog{
		RequestID: "req_account_hover",
		Model:     "gpt-5.4",
		User: &service.User{
			ID:    3,
			Email: "alice@example.com",
			Notes: "张三",
		},
		Account: &service.Account{
			ID:             7,
			Name:           "upstream-b",
			RateMultiplier: &billingRate,
			Credentials: map[string]any{
				"base_url": " https://relay.llmfree.work ",
				"api_key":  "sk-do-not-leak",
			},
			Extra: map[string]any{
				"upstream_billing_probe": map[string]any{
					"status": "ok",
					"data": map[string]any{
						"billing_scope":             "token",
						"resolved_rate_multiplier":  0.08,
						"peak_rate_enabled":         false,
						"effective_rate_multiplier": 0.08,
					},
				},
			},
		},
	}

	userDTO := UsageLogFromService(log)
	adminDTO := UsageLogFromServiceAdmin(log)

	userJSON, err := json.Marshal(userDTO)
	require.NoError(t, err)
	var userMap map[string]any
	require.NoError(t, json.Unmarshal(userJSON, &userMap))
	_, hasAccount := userMap["account"]
	require.False(t, hasAccount)
	require.NotContains(t, string(userJSON), "base_url")
	require.NotContains(t, string(userJSON), "sk-do-not-leak")
	require.NotContains(t, string(userJSON), "upstream_rate_multiplier")
	require.NotContains(t, string(userJSON), "张三")
	require.NotContains(t, string(userJSON), `"notes"`)

	require.NotNil(t, adminDTO.Account)
	require.Equal(t, int64(7), adminDTO.Account.ID)
	require.Equal(t, "upstream-b", adminDTO.Account.Name)
	require.NotNil(t, adminDTO.Account.UpstreamRateMultiplier)
	require.InDelta(t, 0.08, *adminDTO.Account.UpstreamRateMultiplier, 1e-12)
	require.Equal(t, "https://relay.llmfree.work", adminDTO.Account.BaseURL)
	require.NotNil(t, adminDTO.User)
	require.NotNil(t, adminDTO.User.Notes)
	require.Equal(t, "张三", *adminDTO.User.Notes)

	adminJSON, err := json.Marshal(adminDTO)
	require.NoError(t, err)
	require.Contains(t, string(adminJSON), `"upstream_rate_multiplier":0.08`)
	require.NotContains(t, string(adminJSON), `"rate_multiplier":1.25`)
	require.Contains(t, string(adminJSON), `"base_url":"https://relay.llmfree.work"`)
	require.Contains(t, string(adminJSON), `"notes":"张三"`)
	require.NotContains(t, string(adminJSON), "sk-do-not-leak")
}

func f64Ptr(value float64) *float64 {
	return &value
}
