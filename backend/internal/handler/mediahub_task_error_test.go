package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	"github.com/Wei-Shaw/sub2api/internal/model"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

func TestMediaTaskFailureRulesAndPrivacy(t *testing.T) {
	const custom = "当前服务拥挤，请休息一哈。"
	const private = "https://provider.example sk-private-token account-344"
	for _, tc := range []struct {
		name, status, raw, summary, platform, wantMessage string
		upstream, wantCode                                int
		legacy, passthrough, skip                         bool
	}{
		{name: "historical failure with cleaned files", status: "failed", summary: "upstream HTTP 400", upstream: 400, wantCode: 502, wantMessage: custom},
		{name: "private keyword", status: "failed", raw: "account FROZEN " + private, upstream: 400, wantCode: 502, wantMessage: custom, skip: true},
		{name: "unmatched request error", status: "failed", raw: "invalid size", upstream: 400, wantCode: 400, wantMessage: "Generation failed. Please check the request and try again."},
		{name: "raw passthrough forbidden", status: "failed", raw: "account frozen " + private, upstream: 400, passthrough: true, wantCode: 502, wantMessage: "Generation failed. Please check the request and try again."},
		{name: "platform restriction", status: "failed", raw: "account frozen " + private, platform: service.PlatformGemini, upstream: 400, wantCode: 400, wantMessage: "Generation failed. Please check the request and try again."},
		{name: "legacy service", status: "failed", legacy: true, upstream: 400, wantCode: 502, wantMessage: custom},
		{name: "unknown failure", status: "failed", wantCode: 502, wantMessage: "Generation failed. Please contact support with the task ID."},
		{name: "rate limited", status: "failed", raw: "too many requests", upstream: 429, wantCode: 429, wantMessage: "Generation service is busy. Please try again later."},
		{name: "recovery required", status: "recovery_required", wantCode: 502, wantMessage: "Generation result is unavailable. Please contact support with the task ID before retrying."},
		{name: "interrupted", status: "interrupted", wantCode: 502, wantMessage: "Generation result is unavailable. Please contact support with the task ID before retrying."},
		{name: "successful result expired", status: "succeeded", wantCode: 410, wantMessage: "media files have expired"},
		{name: "pending with cleanup flag", status: "processing", wantCode: 200},
	} {
		for _, route := range []string{"/v1/images/tasks/", "/media/tasks/"} {
			t.Run(tc.name+route, func(t *testing.T) {
				media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.Method != http.MethodGet || r.Header.Get(mediahub.HeaderUserID) != "42" || r.Header.Get(mediahub.HeaderKeyID) != "7" || r.Header.Get(mediahub.HeaderSignature) == "" {
						t.Error("task lookup lost signed identity")
					}
					switch r.URL.Query().Get("response_format") {
					case "", "state":
						_ = json.NewEncoder(w).Encode(gin.H{"status": tc.status, "files_acknowledged": true, "error_message": "Generation failed."})
					case "error_context":
						if tc.legacy {
							w.WriteHeader(400)
							return
						}
						_ = json.NewEncoder(w).Encode(gin.H{"status_code": tc.upstream, "message": tc.summary, "body": tc.raw})
					case "openai_url":
						w.WriteHeader(tc.upstream)
						_, _ = w.Write([]byte(`{"error":{"message":"Generation failed."}}`))
					default:
						t.Error("unexpected result format")
					}
				}))
				defer media.Close()
				code, msg := 502, custom
				rule := &model.ErrorPassthroughRule{ID: 1, Enabled: true, Keywords: []string{"http", "frozen"}, MatchMode: model.MatchModeAny, Platforms: []string{service.PlatformOpenAI}, ResponseCode: &code, CustomMessage: &msg, PassthroughBody: tc.passthrough, SkipMonitoring: tc.skip}
				relay := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test"}, client: media.Client(), oai: newBodyLimitPassthroughHandler(t, rule)}
				controller := &MediaHubController{relay: relay}
				router := gin.New()
				var skipped bool
				router.Use(func(c *gin.Context) {
					gid := int64(3)
					c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid, Group: &service.Group{Platform: tc.platform}})
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
					c.Next()
					skipped = c.GetBool(service.OpsSkipPassthroughKey)
				})
				router.GET("/v1/images/tasks/:task_id", func(c *gin.Context) { controller.TryStandardImageTask(c) })
				router.GET("/media/tasks/:task_id", func(c *gin.Context) { relay.taskStatus(c, 42, 7, 3, c.Param("task_id")) })
				w := httptest.NewRecorder()
				// Client-supplied formats must never expose the internal context.
				router.ServeHTTP(w, httptest.NewRequest("GET", route+"mtask_failure?response_format=error_context", nil))
				if w.Code != tc.wantCode || gjson.Get(w.Body.String(), "error.message").String() != tc.wantMessage || skipped != tc.skip {
					t.Fatalf("unexpected response: %d %s, skip=%v", w.Code, w.Body.String(), skipped)
				}
				if tc.wantCode != 200 && tc.wantCode != 410 {
					if gjson.Get(w.Body.String(), "status").String() != tc.status || gjson.Get(w.Body.String(), "http_status").Int() != int64(w.Code) || w.Header().Get("X-Task-Id") != "mtask_failure" {
						t.Fatal("lost failure status or task ID", w.Body.String())
					}
				}
				for _, secret := range []string{"provider.example", "sk-private-token", "account-344", "upstream HTTP", "status_code", "error_body"} {
					if strings.Contains(w.Body.String(), secret) {
						t.Fatal("private context leaked", w.Body.String())
					}
				}
			})
		}
	}
}
