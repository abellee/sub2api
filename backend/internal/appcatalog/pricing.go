package appcatalog

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

const providerPricingSchemaVersion = "1.1"

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
	InputCostPerToken *float64 `json:"input_cost_per_token"`
	Mode              string   `json:"mode"`
}

type providerPricingModel struct {
	ModelName  string  `json:"model_name"`
	GroupName  string  `json:"group_name"`
	InputPrice float64 `json:"input_price"`
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

	body, err := os.ReadFile(s.path)
	if err != nil {
		response.Message = "pricing source unavailable"
		return response
	}
	var source map[string]providerPricingSource
	if err := json.Unmarshal(body, &source); err != nil {
		response.Message = "pricing source invalid"
		return response
	}
	if info, err := os.Stat(s.path); err == nil {
		response.Data.UpdatedAt = info.ModTime().UTC().Format(time.RFC3339)
	}

	for modelName, pricing := range source {
		if pricing.InputCostPerToken == nil || *pricing.InputCostPerToken < 0 {
			continue
		}
		// Image-only entries can carry a zero token price and are not valid
		// token rows for this endpoint.
		if strings.EqualFold(strings.TrimSpace(pricing.Mode), "image_generation") {
			continue
		}
		response.Data.Models = append(response.Data.Models, providerPricingModel{
			ModelName:  modelName,
			GroupName:  "福利",
			InputPrice: *pricing.InputCostPerToken * 1_000_000 * s.multiplier,
		})
	}
	sort.Slice(response.Data.Models, func(i, j int) bool {
		return response.Data.Models[i].ModelName < response.Data.Models[j].ModelName
	})
	return response
}
