package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMediaFailurePersistsPrivateUpstreamDiagnostics(t *testing.T) {
	const invalidSize = "GPT 尺寸每边须为 480～3840 的 16 倍数"
	const requestMessage = invalidSize
	const genericMessage = "Generation failed. Please contact support with the task ID."
	for _, tc := range []struct {
		name, method, path, protocol, state, raw, message string
		upstream, clientStatus                            int
	}{
		{"sync edits", "POST", "/v1/images/edits", "openai", "failed", `{"error":{"message":"` + invalidSize + `","api_key":"sk-private-token"}}`, requestMessage, 400, 400},
		{"sync edits invalid upstream route", "POST", "/v1/images/edits", "openai", "failed", `{"error":{"message":"Invalid URL (POST /v1/v1/images/edits)"}}`, genericMessage, 404, 502},
		{"sync generation mapped status", "POST", "/v1/images/generations", "openai", "failed", `{"error":{"message":"account forbidden"}}`, genericMessage, 403, 502},
		{"sync gemini", "POST", "/v1beta/models/test:generateContent", "gemini", "failed", `{"error":{"message":"` + invalidSize + `"}}`, requestMessage, 400, 400},
		{"image polling", "GET", "/v1/images/tasks/mtask_diagnostic", "", "failed", `{"error":{"message":"account forbidden"}}`, genericMessage, 403, 502},
		{"media polling", "GET", "/media/tasks/mtask_diagnostic", "", "failed", `{"error":{"message":"` + invalidSize + `"}}`, requestMessage, 400, 400},
		{"recovery polling", "GET", "/v1/images/tasks/mtask_diagnostic", "", "recovery_required", "provider unavailable", "Generation result is unavailable. Please contact support with the task ID before retrying.", 503, 502},
		{"video polling", "GET", "/v1/videos/mtask_diagnostic", "", "failed", `{"error":{"message":"upstream route missing"}}`, genericMessage, 404, 502},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setupOpsErrorLogTestQueue(t, 2)
			t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
			var diagnosticRequests int
			media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodGet, r.Method)
				require.Equal(t, "/relay/tasks/mtask_diagnostic", r.URL.Path)
				require.Equal(t, "42", r.Header.Get(mediahub.HeaderUserID))
				require.Equal(t, "7", r.Header.Get(mediahub.HeaderKeyID))
				require.NotEmpty(t, r.Header.Get(mediahub.HeaderSignature))
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Query().Get("response_format") {
				case "error_context":
					diagnosticRequests++
					if r.URL.Query().Get("wait") == "true" {
						w.WriteHeader(http.StatusBadRequest)
						_, _ = w.Write([]byte(`{"error":{"type":"invalid_format","message":"unsupported result format"}}`))
						return
					}
					_ = json.NewEncoder(w).Encode(gin.H{"status_code": tc.upstream, "message": "upstream request failed", "body": tc.raw})
				case "state", "":
					_ = json.NewEncoder(w).Encode(gin.H{"status": tc.state, "error_message": "Generation failed.", "files_acknowledged": true})
				default:
					if tc.protocol != "" {
						require.Equal(t, "true", r.URL.Query().Get("wait"), "synchronous results must still wait")
					}
					w.WriteHeader(tc.clientStatus)
					_ = json.NewEncoder(w).Encode(gin.H{"task_id": "mtask_diagnostic", "error": gin.H{"type": "generation_failed", "message": tc.message}})
				}
			}))
			defer media.Close()
			relay := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test", ForwardTimeout: time.Second}, client: media.Client()}
			controller := &MediaHubController{relay: relay}
			ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
			router := gin.New()
			router.Use(OpsErrorLoggerMiddleware(ops))
			router.Use(func(c *gin.Context) {
				gid := int64(3)
				c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid})
				c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				c.Next()
			})
			if tc.protocol != "" {
				router.POST(tc.path, func(c *gin.Context) {
					c.Set(mediaProtocolKey, tc.protocol)
					relay.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_diagnostic"}`)}, nil, 42, 7, 3)
				})
			} else if strings.HasPrefix(tc.path, "/media/") {
				router.GET("/media/tasks/:task_id", func(c *gin.Context) { relay.taskStatus(c, 42, 7, 3, c.Param("task_id")) })
			} else if strings.HasPrefix(tc.path, "/v1/videos/") {
				router.GET("/v1/videos/:request_id", func(c *gin.Context) { controller.TryStandardVideoTask(c, false) })
			} else {
				router.GET("/v1/images/tasks/:task_id", func(c *gin.Context) { controller.TryStandardImageTask(c) })
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, tc.clientStatus, w.Code)
			require.Contains(t, w.Body.String(), tc.message)
			if tc.message != invalidSize {
				require.NotContains(t, w.Body.String(), invalidSize)
			}
			require.NotContains(t, w.Body.String(), "account forbidden")
			require.NotContains(t, w.Body.String(), "provider unavailable")
			require.NotContains(t, w.Body.String(), "upstream route missing")
			require.Equal(t, 1, diagnosticRequests)
			require.Equal(t, int64(1), OpsErrorLogQueueLength())
			job := <-opsErrorLogQueue
			require.Equal(t, tc.clientStatus, job.entry.StatusCode)
			require.NotNil(t, job.entry.UpstreamStatusCode)
			require.Equal(t, tc.upstream, *job.entry.UpstreamStatusCode)
			require.NotNil(t, job.entry.UpstreamErrorMessage)
			require.NotNil(t, job.entry.UpstreamErrorDetail)
			wantDetail, _ := service.SanitizeOpsErrorBodyForQueue(tc.raw)
			require.Equal(t, wantDetail, *job.entry.UpstreamErrorDetail)
			require.Equal(t, parseOpsErrorResponse([]byte(wantDetail)).Message, *job.entry.UpstreamErrorMessage)
			require.NotContains(t, *job.entry.UpstreamErrorDetail, "sk-private-token")
		})
	}
}

func TestMediaFailureUnavailableDiagnosticsPreservesClientResponse(t *testing.T) {
	media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("response_format") == "error_context" {
			w.WriteHeader(503)
			return
		}
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"error":{"message":"Generation failed."}}`))
	}))
	defer media.Close()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("POST", "/v1/images/edits", nil)
	c.Set(mediaProtocolKey, "openai")
	relay := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: media.URL, ForwardTimeout: time.Second}, client: media.Client()}
	relay.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_diagnostic"}`)}, nil, 42, 7, 3)
	require.Equal(t, 400, w.Code)
	require.JSONEq(t, `{"error":{"message":"Generation failed."}}`, w.Body.String())
	_, recorded := c.Get(service.OpsUpstreamStatusCodeKey)
	require.False(t, recorded, "a failed diagnostic lookup must not be attributed to the provider")
}
