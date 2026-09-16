package appcatalog

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	providerPricingSchemaVersion  = "1.1"
	providerPricingGrokMultiplier = 0.09
)

var providerPricingFallbackCosts = map[string]providerPricingCosts{
	"gpt-5.6-sol": {
		Input:       0.000005,
		Output:      0.000030,
		CacheInput:  providerPricingCost(0.0000005),
		CacheCreate: providerPricingCost(0.00000625),
	},
	"gpt-6-astra": {
		Input:       0.000010,
		Output:      0.000050,
		CacheInput:  providerPricingCost(0.000001),
		CacheCreate: providerPricingCost(0.0000125),
	},
	"grok-4.5": {
		Input:      0.000002,
		Output:     0.000006,
		CacheInput: providerPricingCost(0.0000003),
	},
	"grok-4.6": {
		Input:      0.000002,
		Output:     0.000006,
		CacheInput: providerPricingCost(0.0000005),
	},
}

// ProviderPricingService adapts the local LiteLLM price catalog to Hvoy's
// minimal provider-pricing schema. The source prices are USD per token.
type ProviderPricingService struct {
	path       string
	multiplier float64
}

func NewProviderPricingService(path string, multiplier float64) *ProviderPricingService {
	if strings.TrimSpace(path) == "" {
		path = DefaultProviderPricingPath
	}
	if multiplier < 0 {
		multiplier = DefaultProviderPricingMultiplier
	}
	return &ProviderPricingService{path: path, multiplier: multiplier}
}

type providerPricingSource struct {
	InputCostPerToken                *float64 `json:"input_cost_per_token"`
	OutputCostPerToken               *float64 `json:"output_cost_per_token"`
	CacheReadInputTokenCost          *float64 `json:"cache_read_input_token_cost"`
	CacheCreationInputTokenCost      *float64 `json:"cache_creation_input_token_cost"`
	CacheCreationInputTokenCostAbove *float64 `json:"cache_creation_input_token_cost_above_1hr"`
	Mode                             string   `json:"mode"`
}

type providerPricingCosts struct {
	Input          float64
	Output         float64
	CacheInput     *float64
	CacheCreate    *float64
	CacheCreate1Hr *float64
}

type providerPricingModel struct {
	ModelName           string   `json:"model_name"`
	GroupName           string   `json:"group_name"`
	InputPrice          float64  `json:"input_price"`
	OutputPrice         float64  `json:"output_price"`
	CacheInputPrice     *float64 `json:"cache_input_price"`
	CacheCreatePrice    *float64 `json:"cache_create_price"`
	CacheCreatePrice1Hr *float64 `json:"cache_create_price_1h"`
	Enabled             bool     `json:"enabled"`
	Note                string   `json:"note"`
}

type providerPricingData struct {
	Currency  string                 `json:"currency"`
	PriceUnit string                 `json:"price_unit"`
	UpdatedAt string                 `json:"updated_at"`
	Models    []providerPricingModel `json:"models"`
}

type providerPricingResponse struct {
	SchemaVersion string              `json:"schema_version"`
	Success       bool                `json:"success"`
	Message       string              `json:"message"`
	Data          providerPricingData `json:"data"`
}

func (s *ProviderPricingService) Snapshot() providerPricingResponse {
	response := providerPricingResponse{
		SchemaVersion: providerPricingSchemaVersion,
		Success:       true,
		Message:       "",
		Data: providerPricingData{
			Currency:  "CNY",
			PriceUnit: "per_1m_tokens",
			UpdatedAt: time.Now().UTC().Format(time.RFC3339),
			Models:    []providerPricingModel{},
		},
	}

	costs := make(map[string]providerPricingCosts, len(providerPricingFallbackCosts))
	for modelName, cost := range providerPricingFallbackCosts {
		costs[modelName] = cost
	}

	body, err := os.ReadFile(s.path)
	if err != nil {
		response.Message = "pricing source unavailable; using built-in prices"
	}
	var source map[string]providerPricingSource
	if err == nil {
		if err := json.Unmarshal(body, &source); err != nil {
			response.Message = "pricing source invalid; using built-in prices"
			source = nil
		} else if info, statErr := os.Stat(s.path); statErr == nil {
			response.Data.UpdatedAt = info.ModTime().UTC().Format(time.RFC3339)
		}
	}

	for modelName, pricing := range source {
		modelName = strings.ToLower(strings.TrimSpace(modelName))
		if !isProviderPricingModel(modelName) {
			continue
		}
		// Image-only entries can carry a zero token price and are not valid
		// token rows for this endpoint.
		if strings.EqualFold(strings.TrimSpace(pricing.Mode), "image_generation") {
			continue
		}
		cost := costs[modelName]
		if isValidProviderPrice(pricing.InputCostPerToken) {
			cost.Input = *pricing.InputCostPerToken
		}
		if isValidProviderPrice(pricing.OutputCostPerToken) {
			cost.Output = *pricing.OutputCostPerToken
		}
		if isValidProviderPrice(pricing.CacheReadInputTokenCost) {
			cost.CacheInput = pricing.CacheReadInputTokenCost
		}
		if isValidProviderPrice(pricing.CacheCreationInputTokenCost) {
			cost.CacheCreate = pricing.CacheCreationInputTokenCost
		}
		if isValidProviderPrice(pricing.CacheCreationInputTokenCostAbove) {
			cost.CacheCreate1Hr = pricing.CacheCreationInputTokenCostAbove
		}
		costs[modelName] = cost
	}

	for modelName, cost := range costs {
		multiplier := s.multiplier
		if strings.HasPrefix(modelName, "grok-") {
			multiplier = providerPricingGrokMultiplier
		}
		response.Data.Models = append(response.Data.Models, providerPricingModel{
			ModelName:           modelName,
			GroupName:           "福利",
			InputPrice:          cost.Input * 1_000_000 * multiplier,
			OutputPrice:         cost.Output * 1_000_000 * multiplier,
			CacheInputPrice:     scaleProviderPrice(cost.CacheInput, multiplier),
			CacheCreatePrice:    scaleProviderPrice(cost.CacheCreate, multiplier),
			CacheCreatePrice1Hr: scaleProviderPrice(cost.CacheCreate1Hr, multiplier),
			Enabled:             true,
			Note:                "",
		})
	}
	sort.Slice(response.Data.Models, func(i, j int) bool {
		return response.Data.Models[i].ModelName < response.Data.Models[j].ModelName
	})
	return response
}

func isProviderPricingModel(modelName string) bool {
	_, ok := providerPricingFallbackCosts[modelName]
	return ok
}

func providerPricingCost(value float64) *float64 {
	return &value
}

func isValidProviderPrice(value *float64) bool {
	return value != nil && *value >= 0
}

func scaleProviderPrice(value *float64, multiplier float64) *float64 {
	if value == nil {
		return nil
	}
	scaled := *value * 1_000_000 * multiplier
	return &scaled
}
