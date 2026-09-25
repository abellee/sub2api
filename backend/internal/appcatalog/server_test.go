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
	body := `{
		"schema_version": "1.1",
		"currency": "CNY",
		"price_unit": "per_1m_tokens",
		"models": [
			{"model_name": "gpt-5.6-sol", "group_name": "福利", "input_price": 0.16, "output_price": 0.24, "cache_input_price": 0.04, "cache_create_price": 0.32, "cache_create_price_1h": 0.4, "enabled": true},
			{"model_name": " GPT-6-Astra ", "group_name": "福利", "input_price": 0.8, "output_price": 4, "cache_input_price": 0.08, "cache_create_price": 1, "enabled": true},
			{"model_name": "grok-4.5", "group_name": "Grok - Heavy", "input_price": 0.18, "output_price": 0.54, "cache_input_price": 0.027},
			{"model_name": "grok-4.6", "group_name": "Grok - Heavy", "input_price": 0.18, "output_price": 0.54, "cache_input_price": 0.045, "enabled": true},
			{"model_name": "  ", "input_price": 9}
		]
	}`
	if err := os.WriteFile(pricingPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	srv := &Server{Store: store, ProviderPricing: NewProviderPricingService(pricingPath), StartedAt: time.Now()}
	payload := getProviderPricing(t, srv)
	if payload.SchemaVersion != "1.1" || !payload.Success || payload.Data.Currency != "CNY" || payload.Data.PriceUnit != "per_1m_tokens" {
		t.Fatalf("unexpected envelope: %+v", payload)
	}
	if len(payload.Data.Models) != 4 {
		t.Fatalf("unexpected models: %+v", payload.Data.Models)
	}
	models := make(map[string]providerPricingModel, len(payload.Data.Models))
	for _, model := range payload.Data.Models {
		models[model.ModelName] = model
	}
	if model := models["gpt-5.6-sol"]; model.GroupName != "福利" || !model.Enabled || model.InputPrice != 0.16 || model.OutputPrice != 0.24 || priceValue(model.CacheInputPrice) != 0.04 || priceValue(model.CacheCreatePrice) != 0.32 || priceValue(model.CacheCreatePrice1Hr) != 0.4 {
		t.Fatalf("unexpected sol price: %+v", model)
	}
	if model := models["gpt-6-astra"]; model.GroupName != "福利" || model.InputPrice != 0.8 || model.OutputPrice != 4 || priceValue(model.CacheInputPrice) != 0.08 || priceValue(model.CacheCreatePrice) != 1 || model.CacheCreatePrice1Hr != nil {
		t.Fatalf("unexpected astra price: %+v", model)
	}
	if model := models["grok-4.5"]; model.GroupName != "Grok - Heavy" || !model.Enabled || model.InputPrice != 0.18 || model.OutputPrice != 0.54 || priceValue(model.CacheInputPrice) != 0.027 || model.CacheCreatePrice != nil || model.CacheCreatePrice1Hr != nil {
		t.Fatalf("unexpected Grok 4.5 price: %+v", model)
	}
	if model := models["grok-4.6"]; model.GroupName != "Grok - Heavy" || model.InputPrice != 0.18 || model.OutputPrice != 0.54 || priceValue(model.CacheInputPrice) != 0.045 || model.CacheCreatePrice != nil || model.CacheCreatePrice1Hr != nil {
		t.Fatalf("unexpected Grok price: %+v", model)
	}

	if err := os.WriteFile(pricingPath, []byte(`{"models":[{"model_name":"gpt-5.6-sol","group_name":"福利","input_price":1.25,"output_price":2,"enabled":true}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	reloaded := getProviderPricing(t, srv)
	if len(reloaded.Data.Models) != 1 || reloaded.Data.Models[0].InputPrice != 1.25 {
		t.Fatalf("pricing file was not reloaded: %+v", reloaded.Data.Models)
	}
}

func TestProviderPricingMissingOrInvalid(t *testing.T) {
	store, err := OpenStore(filepath.Join(t.TempDir(), "apps.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	missing := &Server{Store: store, ProviderPricing: NewProviderPricingService(filepath.Join(t.TempDir(), "missing.json")), StartedAt: time.Now()}
	if payload := getProviderPricing(t, missing); payload.Success || payload.Message != "pricing source unavailable" {
		t.Fatalf("unexpected missing file response: %+v", payload)
	}
	invalidPath := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(invalidPath, []byte(`{"models":`), 0o644); err != nil {
		t.Fatal(err)
	}
	invalid := &Server{Store: store, ProviderPricing: NewProviderPricingService(invalidPath), StartedAt: time.Now()}
	if payload := getProviderPricing(t, invalid); payload.Success || payload.Message != "pricing source invalid" {
		t.Fatalf("unexpected invalid file response: %+v", payload)
	}
}

func getProviderPricing(t *testing.T, srv *Server) providerPricingResponse {
	t.Helper()
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
	return payload
}

func priceValue(value *float64) float64 {
	if value == nil {
		return -1
	}
	return *value
}
