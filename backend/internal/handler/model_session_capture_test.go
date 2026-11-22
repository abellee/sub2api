package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/audit-sdk/auditclient"
	"github.com/Wei-Shaw/sub2api/audit-sdk/auditcore"
	"github.com/Wei-Shaw/sub2api/audit-sdk/auditrpc"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func TestModelSessionCaptureLeavesBusinessStatusHeadersAndBodyUnchanged(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		status      int
		streamError bool
		panicValue  bool
		canceled    bool
	}{{"success", 200, false, false, false}, {"business failure", 429, false, false, false}, {"stream failure", 200, true, false, false}, {"panic", 200, false, true, false}, {"canceled", 200, false, false, true}} {
		t.Run(scenario.name, func(t *testing.T) {
			offline := httptest.NewServer(auditrpc.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }), false))
			defer offline.Close()
			dir := t.TempDir()
			client, err := auditclient.New(auditclient.Config{RelayURL: offline.URL, Token: strings.Repeat("t", 32), SpoolDir: dir, SpoolBytes: 1 << 20})
			if err != nil {
				t.Fatal(err)
			}
			defer client.Close()
			client.SetPolicy(auditcore.Policy{Version: 1, Enabled: true, Models: []string{"test-model"}})
			router := gin.New()
			router.Use(gin.CustomRecovery(func(c *gin.Context, v any) { c.Abort() }))
			router.Use(ModelSessionCaptureCompletion())
			// Exercise the real production nesting: ops releases its pooled
			// writer before outer audit completion, while modelSessionWriter
			// remains installed in c.Writer.
			router.Use(OpsErrorLoggerMiddleware(nil))
			requestContext, cancel := context.WithCancel(context.Background())
			defer cancel()
			router.POST("/v1/responses", func(c *gin.Context) {
				input := []byte("input")
				ctx, s := client.CaptureRequest(c.Request.Context(), auditcore.Meta{UserID: 1, Model: "test-model"}, c.Request.Header, input)
				clear(input)
				c.Request = c.Request.WithContext(ctx)
				c.Writer = &modelSessionWriter{ResponseWriter: c.Writer, session: s}
				c.Header("X-Business", "unchanged")
				c.Status(scenario.status)
				chunk := []byte("data: ")
				_, _ = c.Writer.Write(chunk)
				clear(chunk)
				_, _ = c.Writer.WriteString("hello\n\n")
				c.Writer.Flush()
				if scenario.streamError {
					service.MarkOpsStreamError(c, "error", "test", 500)
				}
				if scenario.canceled {
					cancel()
				}
				if scenario.panicValue {
					panic("business panic")
				}
			})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("input")).WithContext(requestContext))
			if w.Code != scenario.status || w.Body.String() != "data: hello\n\n" || w.Header().Get("X-Business") != "unchanged" {
				t.Fatalf("business output changed: %d %q", w.Code, w.Body.String())
			}
			w.Header().Set("X-Business", "reused-after-handler")
			// Backup admission is asynchronous; a clean shutdown commits its
			// journal before another queue instance reopens the durable events.
			client.Close()
			queue, err := auditcore.OpenJournalQueue(filepath.Join(dir, "pending"), 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := queue.Close(); err != nil {
					t.Error(err)
				}
			})
			items, _ := queue.Items(0)
			var end auditcore.Event
			var response auditcore.Event
			var body []byte
			var input []byte
			for i, item := range items {
				b, err := queue.Read(item.ID)
				if err != nil {
					t.Fatal(err)
				}
				frame, err := auditcore.DecodeCaptureFrame(b)
				if err != nil {
					t.Fatal(err)
				}
				if frame.Sequence != int64(i+1) {
					t.Fatalf("out of order frame %d", frame.Sequence)
				}
				e := frame.Event
				if e.Direction == "client-request" {
					input = append(input, e.Body...)
				}
				if e.Direction == "client-response" {
					body = append(body, e.Body...)
				}
				if e.Kind == "end" {
					end = e
				}
				if e.Direction == "client-response" && e.Headers != nil {
					response = e
				}
			}
			if end.Kind != "end" || end.Status != scenario.status || end.Failed != (scenario.streamError || scenario.panicValue || scenario.canceled) {
				t.Fatalf("bad completion event %+v", end)
			}
			if response.Status != scenario.status || response.Headers.Get("X-Business") != "unchanged" {
				t.Fatalf("lost final response metadata %+v", response)
			}
			if !bytes.Equal(body, []byte("data: hello\n\n")) || string(input) != "input" || !w.Flushed {
				t.Fatal("buffer reuse, stream bytes or Flush changed")
			}
		})
	}
}

func TestModelSessionCaptureDoesNotCaptureUnrelatedRoutes(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/api/v1/admin/settings", nil)
	before := c.Request.Context()
	beginModelSessionCapture(c, &service.APIKey{ID: 1, UserID: 1}, "test", nil, "http")
	if c.Request.Context() != before || auditclient.FromContext(c.Request.Context()) != nil {
		t.Fatal("unrelated route captured")
	}
	_ = context.Background()
}
