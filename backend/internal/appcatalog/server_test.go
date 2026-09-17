package appcatalog

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestCreateAppRequiresURL(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "nested", "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	srv := &Server{
		Store:     store,
		Fetcher:   NewFetcher(),
		StartedAt: time.Now(),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/apps", bytes.NewBufferString(`{}`))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCreateAppKeepsManualFieldsWithoutFetch(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	fetcher := NewFetcher()
	fetcher.HTTP = &http.Client{Timeout: time.Millisecond}
	srv := &Server{
		Store:     store,
		Fetcher:   fetcher,
		StartedAt: time.Now(),
	}

	body := `{"github_url":"https://github.com/acme/demo","name":"Demo","description":"hello","changelog":"v1","icon_url":"https://cdn.example/icon.png","screenshots":["https://cdn.example/a.png"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/apps", bytes.NewBufferString(body))
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var app App
	if err := json.NewDecoder(rec.Body).Decode(&app); err != nil {
		t.Fatal(err)
	}
	if app.Name != "Demo" || app.Description != "hello" || app.Changelog != "v1" {
		t.Fatalf("manual fields not saved: %+v", app)
	}
	if app.FetchError != "" {
		t.Fatalf("create with filled fields should not fetch: %+v", app)
	}

	update := `{"github_url":"https://github.com/acme/demo","name":"Demo 2","description":"edited","changelog":"v2","icon_url":"https://cdn.example/icon.png","screenshots":["https://cdn.example/b.png"]}`
	req = httptest.NewRequest(http.MethodPut, "/v1/apps/"+strconv.FormatInt(app.ID, 10), bytes.NewBufferString(update))
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHealth(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	srv := &Server{
		Store:     store,
		Fetcher:   NewFetcher(),
		StartedAt: time.Now(),
		Version:   "test",
	}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var payload map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["status"] != "ok" {
		t.Fatalf("%+v", payload)
	}
}

func TestProviderPricing(t *testing.T) {
	pricingPath := filepath.Join(t.TempDir(), "prices.json")
	body := `{"gpt-5.6-sol":{"input_cost_per_token":0.000002,"output_cost_per_token":0.000003,"cache_read_input_token_cost":0.0000005,"cache_creation_input_token_cost":0.000004,"cache_creation_input_token_cost_above_1hr":0.000005,"mode":"chat"},"gpt-test":{"input_cost_per_token":0.000002,"mode":"chat"},"grok-imagine-image":{"input_cost_per_token":0.000001,"mode":"image_generation"}}`
	if err := os.WriteFile(pricingPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	srv := &Server{Store: store, ProviderPricing: NewProviderPricingService(pricingPath, 0.08), StartedAt: time.Now()}
	req := httptest.NewRequest(http.MethodGet, "/api/provider/pricing", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload providerPricingResponse
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.SchemaVersion != "1.1" || !payload.Success || payload.Data.Currency != "CNY" || payload.Data.PriceUnit != "per_1m_tokens" {
		t.Fatalf("unexpected envelope: %+v", payload)
	}
	if len(payload.Data.Models) != len(providerPricingFallbackCosts) {
		t.Fatalf("unexpected models: %+v", payload.Data.Models)
	}
	models := make(map[string]providerPricingModel, len(payload.Data.Models))
	for _, model := range payload.Data.Models {
		models[model.ModelName] = model
	}
	if _, ok := models["gpt-test"]; ok {
		t.Fatalf("unexpected non-curated model: %+v", models["gpt-test"])
	}
	if _, ok := models["grok-imagine-image"]; ok {
		t.Fatalf("unexpected media model: %+v", models["grok-imagine-image"])
	}
	if model := models["gpt-5.6-sol"]; model.GroupName != "福利" || !model.Enabled || model.InputPrice != 0.16 || model.OutputPrice != 0.24 || priceValue(model.CacheInputPrice) != 0.04 || priceValue(model.CacheCreatePrice) != 0.32 || priceValue(model.CacheCreatePrice1Hr) != 0.4 {
		t.Fatalf("unexpected source override: %+v", model)
	}
	if model := models["gpt-6-astra"]; model.GroupName != "福利" || model.InputPrice != 0.8 || model.OutputPrice != 4 || priceValue(model.CacheInputPrice) != 0.08 || priceValue(model.CacheCreatePrice) != 1 || model.CacheCreatePrice1Hr != nil {
		t.Fatalf("unexpected built-in price: %+v", model)
	}
	if model := models["grok-4.5"]; model.GroupName != "Grok - Heavy" || model.InputPrice != 0.18 || model.OutputPrice != 0.54 || priceValue(model.CacheInputPrice) != 0.027 || model.CacheCreatePrice != nil || model.CacheCreatePrice1Hr != nil {
		t.Fatalf("unexpected Grok 4.5 price: %+v", model)
	}
	if model := models["grok-4.6"]; model.GroupName != "Grok - Heavy" || model.InputPrice != 0.18 || model.OutputPrice != 0.54 || priceValue(model.CacheInputPrice) != 0.045 || model.CacheCreatePrice != nil || model.CacheCreatePrice1Hr != nil {
		t.Fatalf("unexpected Grok price: %+v", model)
	}
}

func priceValue(value *float64) float64 {
	if value == nil {
		return -1
	}
	return *value
}
