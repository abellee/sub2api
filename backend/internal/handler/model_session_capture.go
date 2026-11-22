package handler

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/audit-sdk/auditclient"
	"github.com/Wei-Shaw/sub2api/audit-sdk/auditcore"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

// This adapter is deliberately separate from moderation. The one call in the
// parsed-request seam observes data; it never participates in allow/block logic.
func beginModelSessionCapture(c *gin.Context, key *service.APIKey, model string, body []byte, stage string) {
	defer func() {
		if v := recover(); v != nil {
			slog.Error("audit begin panic isolated; gateway continues", "panic", v)
		}
	}()
	if c == nil || c.Request == nil || key == nil || stage != "http" || auditclient.FromContext(c.Request.Context()) != nil {
		return
	}
	path := c.Request.URL.Path
	supportedEndpoint := strings.HasSuffix(path, "/messages") || strings.HasSuffix(path, "/responses") || strings.HasSuffix(path, "/chat/completions") || strings.Contains(path, ":generateContent") || strings.Contains(path, ":streamGenerateContent")
	if c.Request.Method != http.MethodPost || !supportedEndpoint {
		return
	}
	client := auditclient.Default()
	if client == nil {
		return
	}
	session := ""
	for _, h := range []string{"session_id", "session-id", "X-Session-Id", "X-Claude-Code-Session-Id", "conversation_id"} {
		if v := c.GetHeader(h); v != "" {
			session = v
			break
		}
	}
	if session == "" {
		for _, p := range []string{"session_id", "conversation_id", "metadata.session_id", "metadata.conversation_id", "client_metadata.session_id", "prompt_cache_key"} {
			v := gjson.GetBytes(body, p)
			if v.Type == gjson.String && v.String() != "" {
				session = v.String()
				break
			}
		}
	}
	if session != "" {
		session = "session-" + auditcore.Digest([]byte(session))
	}
	ctx, s := client.CaptureRequest(c.Request.Context(), auditcore.Meta{UserID: key.UserID, APIKeyID: key.ID, GroupID: key.GroupID, Model: model, Endpoint: path, Session: session, Stream: gjson.GetBytes(body, "stream").Bool() || strings.Contains(path, ":streamGenerateContent")}, c.Request.Header, body)
	if s == nil {
		return
	}
	c.Request = c.Request.WithContext(ctx)
	c.Writer = &modelSessionWriter{ResponseWriter: c.Writer, session: s}
}

type modelSessionWriter struct {
	gin.ResponseWriter
	session *auditclient.Session
	failed  bool
}

func (w *modelSessionWriter) Write(p []byte) (int, error) {
	n, err := w.ResponseWriter.Write(p)
	if n > 0 {
		captureModelSession(func() { w.session.Part("client-response", 0, p[:n]) })
	}
	if err != nil || n != len(p) {
		w.failed = true
	}
	return n, err
}
func (w *modelSessionWriter) WriteString(p string) (int, error) {
	n, err := w.ResponseWriter.WriteString(p)
	if n > 0 {
		captureModelSession(func() { w.session.Part("client-response", 0, []byte(p[:n])) })
	}
	if err != nil || n != len(p) {
		w.failed = true
	}
	return n, err
}
func (w *modelSessionWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// Called only on the owner goroutine; the SDK copies before returning and its
// own receivers do all later work. Capture faults cannot replace business I/O.
func captureModelSession(submit func()) {
	defer func() {
		if v := recover(); v != nil {
			slog.Error("audit capture panic isolated; gateway continues", "panic", v)
		}
	}()
	submit()
}

// ModelSessionCaptureCompletion is installed before gateway handlers. Its defer
// observes panics without swallowing or changing the application's recovery.
func ModelSessionCaptureCompletion() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Keep the request's outer writer while it is still owned by this
		// middleware. The capture writer may wrap an ops writer whose lease is
		// released before c.Next returns; querying that wrapper yields status 0.
		response := c.Writer
		defer func() {
			panicValue := recover()
			if c.Request != nil {
				if s := auditclient.FromContext(c.Request.Context()); s != nil {
					_, streamFailed := service.GetOpsStreamError(c)
					failed := panicValue != nil || streamFailed || len(c.Errors) > 0 || c.Request.Context().Err() != nil
					if w, ok := c.Writer.(*modelSessionWriter); ok {
						failed = failed || w.failed
					}
					captureModelSession(func() { s.Complete(response.Header(), response.Status(), failed) })
				}
			}
			if panicValue != nil {
				panic(panicValue)
			}
		}()
		c.Next()
	}
}
