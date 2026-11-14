package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestMediaTaskQueryPolicyForwarding(t *testing.T) {
	for _, code := range []int{429, 410} {
		for _, path := range []string{"/v1/images/tasks/mtask_limits", "/v1/videos/mtask_limits", "/v1/videos/mtask_limits/content", "/media/tasks/mtask_limits"} {
			t.Run(fmt.Sprintf("%d%s", code, path), func(t *testing.T) {
				calls := 0
				body := `{"task_id":"mtask_limits","status":"expired","retryable":false,"stop_polling":true,"error":{"type":"task_query_expired","message":"stop polling"}}`
				if code == 429 {
					body = `{"task_id":"mtask_limits","status":"rate_limited","retry_after_seconds":27,"error":{"type":"task_query_rate_limited","message":"wait"}}`
				}
				media := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					calls++
					marker := "status"
					if path == "/v1/videos/mtask_limits/content" {
						marker = "content"
					}
					require.Equal(t, marker, r.Header.Get("X-Media-Client-Query"))
					require.Equal(t, "42", r.Header.Get(mediahub.HeaderUserID))
					require.NotEmpty(t, r.Header.Get(mediahub.HeaderSignature))
					w.Header().Set("Retry-After", "27")
					w.Header().Set("X-Task-Query-Expires-At", "2026-11-14T08:00:00Z")
					w.Header().Set("Cache-Control", "private, no-store")
					w.WriteHeader(code)
					fmt.Fprint(w, body)
				}))
				defer media.Close()
				relay := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: media.URL, RelaySecret: "test"}, client: media.Client()}
				controller := &MediaHubController{relay: relay}
				router := gin.New()
				router.Use(func(c *gin.Context) {
					gid := int64(3)
					c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid})
					c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 42})
				})
				router.GET("/v1/images/tasks/:task_id", func(c *gin.Context) { controller.TryStandardImageTask(c) })
				router.GET("/v1/videos/:request_id", func(c *gin.Context) { controller.TryStandardVideoTask(c, false) })
				router.GET("/v1/videos/:request_id/content", func(c *gin.Context) { controller.TryStandardVideoTask(c, true) })
				router.GET("/media/tasks/:task_id", func(c *gin.Context) { relay.taskStatus(c, 42, 7, 3, c.Param("task_id")) })
				w := httptest.NewRecorder()
				router.ServeHTTP(w, httptest.NewRequest("GET", path+"?response_format=error_context", nil))
				require.Equal(t, code, w.Code)
				require.JSONEq(t, body, w.Body.String())
				require.Equal(t, "27", w.Header().Get("Retry-After"))
				require.Equal(t, "2026-11-14T08:00:00Z", w.Header().Get("X-Task-Query-Expires-At"))
				require.Equal(t, "private, no-store", w.Header().Get("Cache-Control"))
				require.Equal(t, 1, calls, "policy rejection must not trigger additional diagnostic queries")
			})
		}
	}
}
