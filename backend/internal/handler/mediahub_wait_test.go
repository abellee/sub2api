package handler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/mediahub"
	middleware "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type mediaWaitTransport func(*http.Request) (*http.Response, error)

func (f mediaWaitTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func mediaWaitResponse(code int, body string) *http.Response {
	return &http.Response{StatusCode: code, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}

func TestMediaLongGenerationContinuesThroughPollingWithoutResubmit(t *testing.T) {
	bindings, err := mediahub.NewBindingStore(filepath.Join(t.TempDir(), "bindings.json"))
	require.NoError(t, err)
	require.NoError(t, bindings.Put(mediahub.Binding{GroupID: 3, SchemaID: "test", Enabled: true}))
	synctest.Test(t, func(t *testing.T) {
		posts, waits := 0, 0
		var completed atomic.Bool
		client := &http.Client{Transport: mediaWaitTransport(func(r *http.Request) (*http.Response, error) {
			if r.Method == "POST" {
				posts++
				// The job belongs to mediad, independently of the result GET.
				go func() { time.Sleep(2 * time.Minute); completed.Store(true) }()
				time.Sleep(5 * time.Second)
				return mediaWaitResponse(202, `{"task_id":"mtask_long","query_expires_at":"2030-01-01T00:00:00Z"}`), nil
			}
			require.Equal(t, "/relay/tasks/mtask_long", r.URL.Path)
			if r.URL.Query().Get("wait") == "true" {
				waits++
				<-r.Context().Done()
				return nil, r.Context().Err()
			}
			if r.URL.Query().Get("response_format") == "state" {
				if !completed.Load() {
					return mediaWaitResponse(200, `{"status":"processing"}`), nil
				}
				return mediaWaitResponse(200, `{"status":"succeeded","callback_state":"confirmed"}`), nil
			}
			return mediaWaitResponse(200, `{"data":[{"url":"https://media.example/image.png"}]}`), nil
		})}
		h := &MediaHubRelayHandler{bindings: bindings, cfg: MediaHubConfig{MediadBaseURL: "http://media.example", ForwardTimeout: 15 * time.Minute}, client: client, upstreamResolver: func(int64, string) (string, string) { return "https://provider.example", "fixture" }}
		controller := &MediaHubController{bindings: bindings, relay: h}
		router := gin.New()
		router.Use(func(c *gin.Context) {
			gid := int64(3)
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{ID: 7, GroupID: &gid})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 2})
		})
		router.POST("/v1/images/generations", func(c *gin.Context) { controller.TryStandardImage(c, false) })
		router.GET("/v1/images/tasks/:task_id", func(c *gin.Context) { controller.TryStandardImageTask(c) })
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/v1/images/generations", strings.NewReader(`{"model":"gpt-image-2","prompt":"test"}`))
		r.Header.Set("Content-Type", "application/json")
		start := time.Now()
		router.ServeHTTP(w, r)
		require.Equal(t, 90*time.Second, time.Since(start), "acceptance time must count toward the wait")
		require.Equal(t, 202, w.Code)
		require.Equal(t, "mtask_long", w.Header().Get("X-Task-Id"))
		require.Equal(t, "/v1/images/tasks/mtask_long", w.Header().Get("Location"))
		require.Equal(t, "3", w.Header().Get("Retry-After"))
		require.Equal(t, "2030-01-01T00:00:00Z", gjson.Get(w.Body.String(), "query_expires_at").String())
		poll := func() string {
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest("GET", "/v1/images/tasks/mtask_long", nil))
			require.Equal(t, 200, w.Code)
			return gjson.Get(w.Body.String(), "status").String()
		}
		require.Equal(t, "processing", poll())
		time.Sleep(31 * time.Second)
		synctest.Wait()
		require.Equal(t, "completed", poll())
		require.Equal(t, 1, posts)
		require.Equal(t, 1, waits)
	})
}

type mediaSlowResultBody struct {
	ctx    context.Context
	delay  time.Duration
	reader io.Reader
}

func (b *mediaSlowResultBody) Read(p []byte) (int, error) {
	time.Sleep(b.delay)
	b.delay = 0
	if err := b.ctx.Err(); err != nil {
		return 0, err
	}
	return b.reader.Read(p)
}
func (*mediaSlowResultBody) Close() error { return nil }

func TestMediaResultBodyKeepsFullTimeoutAfterHeaders(t *testing.T) {
	for _, stream := range []bool{false, true} {
		synctest.Test(t, func(t *testing.T) {
			body := `{"data":[{"url":"https://media.example/image.png"}]}`
			if stream {
				body = "event: image_generation.completed\ndata: {}\n\n"
			}
			h := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: "http://media.example", ForwardTimeout: 15 * time.Minute}, client: &http.Client{Transport: mediaWaitTransport(func(r *http.Request) (*http.Response, error) {
				response := mediaWaitResponse(200, "")
				response.Body = &mediaSlowResultBody{ctx: r.Context(), delay: 2 * time.Minute, reader: strings.NewReader(body)}
				return response, nil
			})}}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/v1/images/generations", nil)
			c.Set(mediaProtocolKey, "openai")
			requestBody := []byte(`{}`)
			if stream {
				requestBody = []byte(`{"stream":true}`)
			}
			h.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_slow_body"}`)}, requestBody, 2, 7, 3)
			require.Equal(t, 200, w.Code)
			require.Equal(t, body, w.Body.String())
			require.Empty(t, c.Errors)
		})
	}
}

func TestMediaCanceledWaitIsNotUpstreamFailure(t *testing.T) {
	setupOpsErrorLogTestQueue(t, 2)
	t.Cleanup(func() { resetOpsErrorLoggerStateForTest(t) })
	ops := service.NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	h := &MediaHubRelayHandler{cfg: MediaHubConfig{MediadBaseURL: "http://media.example", ForwardTimeout: time.Minute}, client: &http.Client{Transport: mediaWaitTransport(func(r *http.Request) (*http.Response, error) {
		cancel()
		return nil, r.Context().Err()
	})}}
	router := gin.New()
	router.Use(OpsErrorLoggerMiddleware(ops))
	router.POST("/", func(c *gin.Context) {
		c.Set(mediaProtocolKey, "openai")
		h.finishStandard(c, &mediahub.ForwardResponse{StatusCode: 202, Body: []byte(`{"task_id":"mtask_disconnected"}`)}, nil, 2, 7, 3)
		require.Empty(t, c.Errors)
		_, upstream := c.Get(service.OpsUpstreamStatusCodeKey)
		require.False(t, upstream)
	})
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest("POST", "/", nil).WithContext(ctx))
	require.Equal(t, statusClientClosedRequest, w.Code)
	require.Empty(t, w.Body.String())
	require.Zero(t, OpsErrorLogQueueLength())
}
