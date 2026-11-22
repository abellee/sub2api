package latencytrace

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type memoryRepo struct {
	Repository
	sessions []Session
}

func (r *memoryRepo) CreateLatencySession(_ context.Context, s *Session) error {
	s.StartedAt = time.Now()
	r.sessions = append(r.sessions, *s)
	return nil
}
func (r *memoryRepo) ActiveLatencySessions(context.Context) ([]Session, error) {
	return append([]Session(nil), r.sessions...), nil
}
func (r *memoryRepo) StopLatencySession(_ context.Context, id string) error {
	for i, s := range r.sessions {
		if s.ID == id {
			r.sessions = append(r.sessions[:i], r.sessions[i+1:]...)
			return nil
		}
	}
	return ErrNotFound
}

func testCollector(t *testing.T) (*Collector, Session) {
	t.Helper()
	c := New(&memoryRepo{})
	s, err := c.Create(context.Background(), Scope{2, 145, 748}, 1)
	require.NoError(t, err)
	return c, *s
}
func beginTest(c *Collector, user, group int64) (context.Context, *Trace) {
	return Begin(WithCollector(context.Background(), c), user, group, "request-1", time.Now())
}

func TestCollectionScopeAndUniqueLifecycle(t *testing.T) {
	c, s := testCollector(t)
	_, err := uuid.Parse(s.ID)
	require.NoError(t, err)
	for _, scope := range [][2]int64{{3, 145}, {2, 146}} {
		_, trace := beginTest(c, scope[0], scope[1])
		require.Nil(t, trace)
	}
	_, trace := beginTest(c, 2, 145)
	require.NotNil(t, trace)
	trace.StartAttempt(749, "http", 123)
	trace.Finish("client-1", 200)
	require.Empty(t, c.queue)
	_, trace = beginTest(c, 2, 145)
	trace.StartAttempt(748, "http", 123)
	trace.Finish("client-1", 200)
	record := <-c.queue
	require.Equal(t, s.ID, record.CollectionID)
	require.Equal(t, "client-1", record.ClientRequestID)
	_, pending := beginTest(c, 2, 145)
	pending.StartAttempt(748, "http", 1)
	require.NoError(t, c.Stop(context.Background(), s.ID))
	pending.Mark("after_stop")
	pending.Finish("client-2", 200)
	require.Empty(t, c.queue)
	_, trace = beginTest(c, 2, 145)
	require.Nil(t, trace)
	next, err := c.Create(context.Background(), s.Scope, 1)
	require.NoError(t, err)
	require.NotEqual(t, s.ID, next.ID)
	_, trace = Begin(WithCollector(context.Background(), c), 2, 145, "old-request", s.StartedAt.Add(-time.Second))
	require.Nil(t, trace)
}

func TestCollectionInvalidAndStaleConfiguration(t *testing.T) {
	c, s := testCollector(t)
	_, err := c.Create(context.Background(), Scope{0, 145, 748}, 1)
	require.ErrorIs(t, err, ErrInvalidScope)
	c.cache.Store(&snapshot{sessions: []Session{s}, loaded: time.Now().Add(-11 * time.Second)})
	_, trace := beginTest(c, 2, 145)
	require.Nil(t, trace)
	require.False(t, c.Health().Ready)
}

func TestCollectionBoundsAndConcurrentEvents(t *testing.T) {
	c, _ := testCollector(t)
	_, trace := beginTest(c, 2, 145)
	a := trace.StartAttempt(748, "http", 123)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				a.Mark("first_response_byte")
				trace.MarkCurrent("first_semantic_output")
				a.Reused(true)
			}
		}()
	}
	wg.Wait()
	for i := 0; i < 400; i++ {
		trace.Mark(fmt.Sprintf("bounded_%d", i))
		trace.StartAttempt(748, "http", 1)
	}
	trace.Finish("c", 200)
	record := <-c.queue
	require.Len(t, record.Events, 256)
	require.Len(t, record.Attempts, 32)
	require.True(t, record.Truncated)
	first := 0
	for _, event := range record.Events {
		if event.Name == "first_response_byte" {
			first++
		}
	}
	require.Equal(t, 1, first)
	trace.Finish("c", 200)
	require.Empty(t, c.queue)
	for i := 0; i < cap(c.queue)+1; i++ {
		_, tr := beginTest(c, 2, 145)
		tr.StartAttempt(748, "http", 0)
		tr.Finish("c", 200)
	}
	require.Equal(t, cap(c.queue), len(c.queue))
	require.Equal(t, uint64(1), c.Health().Dropped)
}

func TestCollectionHTTPPreservesPayloadAndTraceAndConnectionReuse(t *testing.T) {
	const secret = "private prompt and sk-secret-credential"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != secret {
			t.Error("upstream body changed")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: "+secret+"\n\n")
	}))
	defer server.Close()
	c, _ := testCollector(t)
	ctx, trace := beginTest(c, 2, 145)
	client := server.Client()
	defer client.CloseIdleConnections()
	var existingCallback int
	ctx = httptrace.WithClientTrace(ctx, &httptrace.ClientTrace{GotConn: func(httptrace.GotConnInfo) { existingCallback++ }})
	for i := 0; i < 2; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, server.URL, strings.NewReader(secret))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer sk-secret-credential")
		req, done := ObserveHTTP(req, 748)
		resp, err := client.Do(req)
		done(resp, err)
		require.NoError(t, err)
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		require.Contains(t, string(body), secret)
		require.NoError(t, resp.Body.Close())
	}
	trace.Finish("c", 200)
	record := <-c.queue
	require.Len(t, record.Attempts, 2)
	require.Equal(t, 2, existingCallback)
	require.NotNil(t, record.Attempts[1].Reused)
	require.True(t, *record.Attempts[1].Reused)
	for attempt := 1; attempt <= 2; attempt++ {
		for _, name := range []string{"upstream_start", "connection_ready", "upload_complete", "first_response_byte", "upstream_headers", "first_body_byte", "upstream_body_complete"} {
			found := false
			for _, event := range record.Events {
				if event.Attempt == attempt && event.Name == name {
					found = true
				}
			}
			require.True(t, found, "missing %s for attempt %d", name, attempt)
		}
	}
	encoded, err := json.Marshal(record)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), secret)
	require.NotContains(t, string(encoded), "Authorization")
}

func TestCollectionTransportFailureDoesNotInventHeaders(t *testing.T) {
	c, _ := testCollector(t)
	ctx, trace := beginTest(c, 2, 145)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://example.invalid", nil)
	require.NoError(t, err)
	_, done := ObserveHTTP(req, 748)
	done(nil, fmt.Errorf("url with secret"))
	trace.Finish("c", 502)
	r := <-c.queue
	require.True(t, r.Attempts[0].Failed)
	for _, e := range r.Events {
		require.NotEqual(t, "upstream_headers", e.Name)
	}
}
