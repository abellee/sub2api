package admin

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestAppCatalogListPublicReturnsEmptyWhenUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := service.NewAppCatalogService(&config.Config{AppCatalog: config.AppCatalogConfig{BaseURL: "http://127.0.0.1:1"}})
	handler := NewAppCatalogHandler(svc)
	router := gin.New()
	router.GET("/api/v1/app-catalog", handler.ListPublic)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/app-catalog", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	payload := decodeAppCatalogList(t, recorder)
	if payload.Total != 0 || len(payload.Items) != 0 {
		t.Fatalf("payload=%+v", payload)
	}
}

func TestAppCatalogListPublicReturnsItems(t *testing.T) {
	gin.SetMode(gin.TestMode)
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": []map[string]any{{"id": 4, "name": "Grok App"}},
			"total": 1,
		})
	})
	upstream := httptest.NewServer(mux)
	t.Cleanup(upstream.Close)

	svc := service.NewAppCatalogService(&config.Config{AppCatalog: config.AppCatalogConfig{BaseURL: upstream.URL}})
	handler := NewAppCatalogHandler(svc)
	router := gin.New()
	router.GET("/api/v1/app-catalog", handler.ListPublic)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/app-catalog", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	payload := decodeAppCatalogList(t, recorder)
	if payload.Total != 1 || len(payload.Items) != 1 || payload.Items[0].Name != "Grok App" {
		t.Fatalf("payload=%+v", payload)
	}
}

func decodeAppCatalogList(t *testing.T, recorder *httptest.ResponseRecorder) struct {
	Items []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"items"`
	Total int `json:"total"`
} {
	t.Helper()
	var body response.Response
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(body.Data)
	if err != nil {
		t.Fatal(err)
	}
	var payload struct {
		Items []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		} `json:"items"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal(encoded, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Items == nil {
		payload.Items = []struct {
			ID   int64  `json:"id"`
			Name string `json:"name"`
		}{}
	}
	return payload
}
