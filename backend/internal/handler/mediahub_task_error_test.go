package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
		{name: "unmatched request error", status: "failed", raw: "invalid size", upstream: 400, wantCode: 400, wantMessage: "invalid size"},
		{name: "non-parameter passthrough forbidden", status: "failed", raw: "account frozen " + private, upstream: 403, passthrough: true, wantCode: 502, wantMessage: "Generation failed. Please contact support with the task ID."},
		{name: "parameter keyword passthrough", status: "failed", raw: `{"error":{"message":"invalid size: expected 16 pixel multiples"},"api_key":"sk-private-token"}`, upstream: 400, passthrough: true, wantCode: 502, wantMessage: "invalid size: expected 16 pixel multiples"},
		{name: "plain parameter error", status: "failed", raw: "expected 16 pixel multiples", upstream: 422, passthrough: true, wantCode: 502, wantMessage: "expected 16 pixel multiples"},
		{name: "platform restriction", status: "failed", raw: "invalid size", platform: service.PlatformGemini, upstream: 400, wantCode: 400, wantMessage: "invalid size"},
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
				rule := &model.ErrorPassthroughRule{ID: 1, Enabled: true, Keywords: []string{"http", "frozen", "16 pixel"}, MatchMode: model.MatchModeAny, Platforms: []string{service.PlatformOpenAI}, ResponseCode: &code, CustomMessage: &msg, PassthroughBody: tc.passthrough, SkipMonitoring: tc.skip}
				relay := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test", ForwardTimeout: time.Second}, client: media.Client(), oai: newBodyLimitPassthroughHandler(t, rule)}
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

func TestMediaStandardFailureRules(t *testing.T) {
	const original = "GPT 尺寸每边须为 480～3840 的 16 倍数"
	const generic = "Generation failed. Please check the request and try again."
	const custom = "请调整图片尺寸后重试。"
	for _, tc := range []struct {
		name                   string
		upstream               int
		keyword                string
		passthrough, codeMatch bool
		want                   string
	}{
		{"parameter keyword passthrough", 400, "GPT 尺寸", true, false, original},
		{"case insensitive keyword", 422, "gpt", true, false, original},
		{"payload size keyword", 413, "尺寸", true, false, original},
		{"format keyword", 415, "尺寸", true, false, original},
		{"custom message", 400, "尺寸", false, false, custom},
		{"unmatched keyword", 400, "unsupported quality", true, false, original},
		{"OR code without keyword", 400, "unsupported quality", true, true, original},
		{"code only", 400, "", true, true, original},
		{"no rule service", 400, "", true, false, original},
		{"non-parameter error", 503, "尺寸", true, false, generic},
	} {
		for _, protocol := range []string{"openai", "openai_stream", "gemini", "grok_image", "video"} {
			t.Run(tc.name+"/"+protocol, func(t *testing.T) {
				media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					if r.URL.Query().Get("response_format") == "error_context" {
						if r.URL.Query().Get("wait") == "true" {
							t.Error("diagnostics must not wait")
						}
						_ = json.NewEncoder(w).Encode(gin.H{"status_code": tc.upstream, "body": `{"error":{"message":"` + original + `"},"api_key":"sk-private-token"}`})
						return
					}
					w.WriteHeader(tc.upstream)
					_ = json.NewEncoder(w).Encode(gin.H{"error": gin.H{"message": generic}})
				}))
				defer media.Close()
				code, msg := 400, custom
				rule := &model.ErrorPassthroughRule{Enabled: true, MatchMode: model.MatchModeAny, ResponseCode: &code, CustomMessage: &msg, PassthroughBody: tc.passthrough}
				if tc.keyword != "" {
					rule.Keywords = []string{tc.keyword}
				}
				if tc.codeMatch {
					rule.ErrorCodes = []int{tc.upstream}
				}
				relay := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test", ForwardTimeout: time.Second}, client: media.Client(), oai: newBodyLimitPassthroughHandler(t, rule)}
				if tc.name == "no rule service" {
					relay.oai = nil
				}
				controller := &MediaHubController{relay: relay}
				router := gin.New()
				router.Use(func(c *gin.Context) {
					gid := int64(3)
					c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid})
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				})
				method, path := "POST", "/v1/images/edits"
				if protocol == "video" {
					method, path = "GET", "/v1/videos/mtask_rule"
					router.GET("/v1/videos/:request_id", func(c *gin.Context) { controller.TryStandardVideoTask(c, false) })
				} else {
					router.POST(path, func(c *gin.Context) {
						body := []byte(nil)
						if protocol == "openai_stream" {
							protocol = "openai"
							body = []byte(`{"stream":true}`)
						}
						c.Set(mediaProtocolKey, protocol)
						relay.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_rule"}`)}, body, 42, 7, 3)
					})
				}
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest(method, path, nil))
				wantStatus := 400
				if tc.name == "unmatched keyword" || tc.name == "no rule service" {
					wantStatus = tc.upstream
				}
				if w.Code != wantStatus || gjson.Get(w.Body.String(), "error.message").String() != tc.want {
					t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
				}
				if w.Header().Get("X-Task-Id") != "mtask_rule" {
					t.Fatal("missing task ID")
				}
				if strings.Contains(w.Body.String(), "sk-private-token") {
					t.Fatal("full provider body exposed")
				}
			})
		}
	}
}
