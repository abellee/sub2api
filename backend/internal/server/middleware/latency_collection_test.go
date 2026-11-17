package middleware

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/latencytrace"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type latencyMiddlewareRepo struct {
	latencytrace.Repository
	saved chan latencytrace.Record
}

func (r *latencyMiddlewareRepo) ActiveLatencySessions(context.Context) ([]latencytrace.Session, error) {
	return []latencytrace.Session{{ID: "collection", Scope: latencytrace.Scope{UserID: 2, GroupID: 145, AccountID: 748}, StartedAt: time.Now().Add(-time.Minute)}}, nil
}
func (r *latencyMiddlewareRepo) InsertLatencyRecord(_ context.Context, record *latencytrace.Record) error {
	r.saved <- *record
	return nil
}

func TestLatencyMiddlewareMatchesAndPreservesStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &latencyMiddlewareRepo{saved: make(chan latencytrace.Record, 2)}
	collector := latencytrace.New(repo)
	collector.Start(context.Background())
	defer collector.Close()
	router := gin.New()
	router.Use(RequestLogger(), LatencyCollection(collector), ClientRequestID())
	router.POST("/v1/responses", func(c *gin.Context) {
		group := &service.Group{ID: 145, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
		key := &service.APIKey{ID: 1, UserID: 2, GroupID: &group.ID, Group: group}
		c.Set(string(ContextKeyAPIKey), key)
		setGroupContext(c, group)
		trace := latencytrace.From(c.Request.Context())
		require.NotNil(t, trace)
		body, err := io.ReadAll(c.Request.Body)
		require.NoError(t, err)
		require.Equal(t, "private-body", string(body))
		trace.StartAttempt(748, "http", int64(len(body)))
		c.Header("Content-Type", "text/event-stream")
		_, _ = c.Writer.WriteString("data: first\n\n")
		c.Writer.Flush()
		_, _ = c.Writer.Write([]byte("data: second\n\n"))
		c.Writer.Flush()
	})
	response := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/v1/responses", strings.NewReader("private-body"))
	router.ServeHTTP(response, request)
	require.Equal(t, 200, response.Code)
	require.Equal(t, "data: first\n\ndata: second\n\n", response.Body.String())
	require.True(t, response.Flushed)
	select {
	case record := <-repo.saved:
		require.Equal(t, int64(12), record.RequestBytes)
		require.NotEmpty(t, record.ClientRequestID)
		require.Equal(t, "collection", record.CollectionID)
		names := map[string]int{}
		for _, e := range record.Events {
			names[e.Name]++
		}
		for _, name := range []string{"auth_complete", "body_read_start", "body_read_complete", "downstream_first_write", "downstream_first_flush", "request_complete"} {
			require.Equal(t, 1, names[name], name)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no collected record")
	}
}
