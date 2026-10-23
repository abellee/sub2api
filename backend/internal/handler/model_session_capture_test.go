package handler

import (
	"context"
	"encoding/json"
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
	}{{"success", 200, false, false}, {"business failure", 429, false, false}, {"stream failure", 200, true, false}, {"panic", 200, false, true}} {
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
			router.POST("/v1/responses", func(c *gin.Context) {
				ctx, s := client.Begin(c.Request.Context(), auditcore.Meta{UserID: 1, Model: "test-model"})
				c.Request = c.Request.WithContext(ctx)
				c.Writer = &modelSessionWriter{ResponseWriter: c.Writer, session: s}
				s.Part("client-request", 0, []byte("input"))
				c.Header("X-Business", "unchanged")
				c.Status(scenario.status)
				_, _ = c.Writer.WriteString("data: hello\n\n")
				c.Writer.Flush()
				if scenario.streamError {
					service.MarkOpsStreamError(c, "error", "test", 500)
				}
				if scenario.panicValue {
					panic("business panic")
				}
			})
			w := httptest.NewRecorder()
			router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader("input")))
			if w.Code != scenario.status || w.Body.String() != "data: hello\n\n" || w.Header().Get("X-Business") != "unchanged" {
				t.Fatalf("business output changed: %d %q", w.Code, w.Body.String())
			}
			// Completion returns independently of durable spool writes. Join the
			// SDK before inspecting its on-disk records through another queue.
			client.Close()
			queue, err := auditcore.OpenQueue(filepath.Join(dir, "pending"), 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			items, _ := queue.Items(0)
			var end auditcore.Event
			for _, item := range items {
				b, _ := queue.Read(item.ID)
				var e auditcore.Event
				_ = json.Unmarshal(b, &e)
				if e.Kind == "end" {
					end = e
				}
			}
			if end.Kind != "end" || end.Failed != (scenario.streamError || scenario.panicValue) {
				t.Fatalf("bad completion event %+v", end)
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
