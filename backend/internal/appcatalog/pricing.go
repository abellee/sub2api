package appcatalog

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"time"
)

const providerPricingSchemaVersion = "1.1"

// ProviderPricingService serves the Hvoy provider-pricing document from a JSON
// file. Snapshot reads that file on every call, so price edits show up without
// a restart or a new image.
type ProviderPricingService struct {
	path string
}

func NewProviderPricingService(path string) *ProviderPricingService {
	if strings.TrimSpace(path) == "" {
		path = DefaultProviderPricingPath
	}
	return &ProviderPricingService{path: path}
}

type providerPricingFile struct {
	SchemaVersion string                     `json:"schema_version"`
	Currency      string                     `json:"currency"`
	PriceUnit     string                     `json:"price_unit"`
	Models        []providerPricingFileModel `json:"models"`
}

type providerPricingFileModel struct {
	ModelName           string   `json:"model_name"`
	GroupName           string   `json:"group_name"`
	InputPrice          float64  `json:"input_price"`
	OutputPrice         float64  `json:"output_price"`
	CacheInputPrice     *float64 `json:"cache_input_price"`
	CacheCreatePrice    *float64 `json:"cache_create_price"`
	CacheCreatePrice1Hr *float64 `json:"cache_create_price_1h"`
	Enabled             *bool    `json:"enabled"`
	Note                string   `json:"note"`
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

	body, err := os.ReadFile(s.path)
	if err != nil {
		response.Success = false
		response.Message = "pricing source unavailable"
		return response
	}
	var file providerPricingFile
	if err := json.Unmarshal(body, &file); err != nil {
		response.Success = false
		response.Message = "pricing source invalid"
		return response
	}
	if info, statErr := os.Stat(s.path); statErr == nil {
		response.Data.UpdatedAt = info.ModTime().UTC().Format(time.RFC3339)
	}
	if strings.TrimSpace(file.SchemaVersion) != "" {
		response.SchemaVersion = strings.TrimSpace(file.SchemaVersion)
	}
	if strings.TrimSpace(file.Currency) != "" {
		response.Data.Currency = strings.TrimSpace(file.Currency)
	}
	if strings.TrimSpace(file.PriceUnit) != "" {
		response.Data.PriceUnit = strings.TrimSpace(file.PriceUnit)
	}

	for _, model := range file.Models {
		modelName := strings.ToLower(strings.TrimSpace(model.ModelName))
		if modelName == "" {
			continue
		}
		enabled := true
		if model.Enabled != nil {
			enabled = *model.Enabled
		}
		response.Data.Models = append(response.Data.Models, providerPricingModel{
			ModelName:           modelName,
			GroupName:           model.GroupName,
			InputPrice:          model.InputPrice,
			OutputPrice:         model.OutputPrice,
			CacheInputPrice:     model.CacheInputPrice,
			CacheCreatePrice:    model.CacheCreatePrice,
			CacheCreatePrice1Hr: model.CacheCreatePrice1Hr,
			Enabled:             enabled,
			Note:                model.Note,
		})
	}
	sort.Slice(response.Data.Models, func(i, j int) bool {
		return response.Data.Models[i].ModelName < response.Data.Models[j].ModelName
	})
	return response
}
