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
	ctx, s := client.Begin(c.Request.Context(), auditcore.Meta{UserID: key.UserID, APIKeyID: key.ID, GroupID: key.GroupID, Model: model, Endpoint: path, Session: session, Stream: gjson.GetBytes(body, "stream").Bool() || strings.Contains(path, ":streamGenerateContent")})
	if s == nil {
		return
	}
	c.Request = c.Request.WithContext(ctx)
	s.Metadata("client-request", 0, c.Request.Header, 0)
	s.Part("client-request", 0, body)
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
		w.session.Part("client-response", 0, p[:n])
	}
	if err != nil || n != len(p) {
		w.failed = true
	}
	return n, err
}
func (w *modelSessionWriter) WriteString(p string) (int, error) {
	n, err := w.ResponseWriter.WriteString(p)
	if n > 0 {
		w.session.Part("client-response", 0, []byte(p[:n]))
	}
	if err != nil || n != len(p) {
		w.failed = true
	}
	return n, err
}
func (w *modelSessionWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// ModelSessionCaptureCompletion is installed before gateway handlers. Its defer
// observes panics without swallowing or changing the application's recovery.
func ModelSessionCaptureCompletion() gin.HandlerFunc {
	_ = auditclient.Default()
	return func(c *gin.Context) {
		defer func() {
			panicValue := recover()
			if c.Request != nil {
				if s := auditclient.FromContext(c.Request.Context()); s != nil {
					_, streamFailed := service.GetOpsStreamError(c)
					failed := panicValue != nil || streamFailed || len(c.Errors) > 0 || c.Request.Context().Err() != nil
					if w, ok := c.Writer.(*modelSessionWriter); ok {
						failed = failed || w.failed
					}
					s.Metadata("client-response", 0, c.Writer.Header(), c.Writer.Status())
					s.Finish(c.Writer.Status(), failed)
				}
			}
			if panicValue != nil {
				panic(panicValue)
			}
		}()
		c.Next()
	}
}
