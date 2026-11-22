package handler

import (
	"io"
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

func TestMediaGrokVideoLifecycle(t *testing.T) {
	status := "processing"
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(mediahub.HeaderUserID) != "42" {
			t.Error("missing ownership")
		}
		if r.Method == "POST" {
			if !strings.HasPrefix(r.URL.Path, "/relay/video-rule/") {
				t.Error("image rule used for video", r.URL.Path)
			}
			raw, _ := io.ReadAll(r.Body)
			if r.Header.Get(mediahub.HeaderModel) != "grok-imagine-video" || gjson.GetBytes(raw, "model").Exists() {
				t.Error("missing video model")
			}
			w.WriteHeader(202)
			_, _ = io.WriteString(w, `{"task_id":"mtask_video","result_url":"https://d.tokenfree.dev/downloads/mtask_video/result?token=test"}`)
			return
		}
		if r.URL.Query().Get("response_format") == "state" {
			_, _ = io.WriteString(w, `{"status":"`+status+`","callback_state":"confirmed"}`)
			return
		}
		if r.URL.Query().Get("response_format") != "grok_video" {
			t.Error("wrong result format")
		}
		if status != "succeeded" {
			_, _ = io.WriteString(w, `{"request_id":"mtask_video","status":"pending"}`)
			return
		}
		_, _ = io.WriteString(w, `{"request_id":"mtask_video","status":"done","video":{"url":"https://d.tokenfree.dev/downloads/video.mp4","duration":6}}`)
	}))
	defer media.Close()
	bindings, err := mediahub.NewBindingStore(filepath.Join(t.TempDir(), "bindings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = bindings.Put(mediahub.Binding{GroupID: 3, SchemaID: "image-rule", VideoSchemaID: "video-rule", Enabled: true}); err != nil {
		t.Fatal(err)
	}
	relay := &MediaHubRelayHandler{bindings: bindings, cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test", ForwardTimeout: time.Second}, client: media.Client(), upstreamResolver: func(int64, string) (string, string) { return "https://example.com", "test" }}
	controller := &MediaHubController{bindings: bindings, relay: relay}
	router := gin.New()
	router.Use(func(c *gin.Context) {
		gid := int64(3)
		c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid})
		c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
	})
	router.POST("/v1/videos/generations", func(c *gin.Context) { controller.TryStandardVideo(c) })
	router.GET("/v1/videos/:request_id", func(c *gin.Context) {
		if !controller.TryStandardVideoTask(c, false) {
			c.Status(418)
		}
	})
	router.GET("/v1/videos/:request_id/content", func(c *gin.Context) { controller.TryStandardVideoTask(c, true) })
	request := func(method, path string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, path, strings.NewReader(`{"prompt":"test"}`))
		r.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, r)
		return w
	}
	if w := request("POST", "/v1/videos/generations"); w.Code != 200 || gjson.Get(w.Body.String(), "request_id").String() != "mtask_video" || gjson.Get(w.Body.String(), "id").String() != "mtask_video" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/v1/videos/mtask_video"); w.Code != 200 || gjson.Get(w.Body.String(), "status").String() != "pending" {
		t.Fatal(w.Code, w.Body.String())
	}
	status = "succeeded"
	if w := request("GET", "/v1/videos/mtask_video"); w.Code != 200 || !strings.HasSuffix(gjson.Get(w.Body.String(), "video.url").String(), ".mp4") {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/v1/videos/mtask_video/content"); w.Code != 307 || w.Header().Get("Location") != "https://d.tokenfree.dev/downloads/video.mp4" {
		t.Fatal(w.Code, w.Body.String())
	}
	if w := request("GET", "/v1/videos/legacy-upstream-id"); w.Code != 418 {
		t.Fatal("legacy task intercepted")
	}
}
