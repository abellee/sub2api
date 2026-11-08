package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func TestMediaStandardEndpoints(t *testing.T) {
	for _, protocol := range []string{"openai", "openai_async", "gemini"} {
		t.Run(protocol, func(t *testing.T) {
			stateQueries := 0
			media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get(mediahub.HeaderUserID) != "42" || r.Header.Get(mediahub.HeaderKeyID) != "7" {
					t.Error("missing signed identity")
				}
				if r.Method == "POST" {
					want := "gpt-image-2.6"
					if protocol == "gemini" {
						want = "gemini-3-pro-image"
					}
					if r.Header.Get(mediahub.HeaderModel) != want {
						t.Error("model missing from relay", r.Header.Get(mediahub.HeaderModel))
					}
					w.WriteHeader(202)
					_, _ = w.Write([]byte(`{"task_id":"mtask_standard"}`))
					return
				}
				switch r.URL.Query().Get("response_format") {
				case "state":
					stateQueries++
					status, callback := "processing", "pending"
					if stateQueries > 1 || protocol == "openai_async" {
						status = "succeeded"
						callback = "confirmed"
					}
					_ = json.NewEncoder(w).Encode(gin.H{"status": status, "callback_state": callback})
				case "gemini":
					_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aW1hZ2U="}}]}}],"download_urls":["https://d.tokenfree.dev/downloads/test"]}`))
				case "openai_url":
					_, _ = w.Write([]byte(`{"created":123,"data":[{"url":"https://d.tokenfree.dev/downloads/test"}]}`))
				default:
					t.Error("unexpected result format", r.URL.RawQuery)
				}
			}))
			defer media.Close()
			bindings, err := mediahub.NewBindingStore(filepath.Join(t.TempDir(), "bindings.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err = bindings.Put(mediahub.Binding{GroupID: 3, SchemaID: "test", Enabled: true}); err != nil {
				t.Fatal(err)
			}
			relay := &MediaHubRelayHandler{bindings: bindings, cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test", ForwardTimeout: 2 * time.Second}, client: media.Client(), upstreamResolver: func(int64, string) (string, string) { return "https://upstream.example", "test" }}
			controller := &MediaHubController{bindings: bindings, relay: relay}
			router := gin.New()
			router.Use(func(c *gin.Context) {
				gid := int64(3)
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid})
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
			})
			router.POST("/v1/images/generations", func(c *gin.Context) { controller.TryStandardImage(c, false) })
			router.POST("/v1/images/generations/async", func(c *gin.Context) { controller.TryStandardImage(c, true) })
			router.GET("/v1/images/tasks/:task_id", func(c *gin.Context) { controller.TryStandardImageTask(c) })
			router.POST("/v1beta/models/*modelAction", func(c *gin.Context) { controller.TryStandardGemini(c) })
			path, body := "/v1/images/generations", `{"model":"gpt-image-2.6","prompt":"test","response_format":"url"}`
			if protocol == "gemini" {
				path = "/v1beta/models/gemini-3-pro-image:generateContent"
				body = `{"contents":[{"parts":[{"text":"test"}]}]}`
			}
			if protocol == "openai_async" {
				path += "/async"
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest("POST", path, strings.NewReader(body))
			r.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, r)
			if protocol == "openai_async" {
				if w.Code != 202 || gjson.Get(w.Body.String(), "poll_url").String() != "/v1/images/tasks/mtask_standard" {
					t.Fatal(w.Code, w.Body.String())
				}
				w = httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", "/v1/images/tasks/mtask_standard", nil))
				if w.Code != 200 || gjson.Get(w.Body.String(), "status").String() != "completed" {
					t.Fatal(w.Code, w.Body.String())
				}
			} else if w.Code != 200 {
				t.Fatal(w.Code, w.Body.String())
			} else if protocol == "gemini" && !gjson.Get(w.Body.String(), "candidates.0.content.parts.0.inlineData.data").Exists() {
				t.Fatal("native response lost", w.Body.String())
			} else if protocol == "openai" && !gjson.Get(w.Body.String(), "data.0.url").Exists() {
				t.Fatal("standard images response lost", w.Body.String())
			}
		})
	}
}
