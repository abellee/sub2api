package middleware

import (
	"io"
	"net/http"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	pkghttputil "github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/latencytrace"
	"github.com/gin-gonic/gin"
)

func LatencyCollection(collector *latencytrace.Collector) gin.HandlerFunc {
	return func(c *gin.Context) {
		if collector == nil {
			c.Next()
			return
		}
		c.Request = c.Request.WithContext(latencytrace.WithCollector(c.Request.Context(), collector))
		defer func() {
			if trace := latencytrace.From(c.Request.Context()); trace != nil {
				clientID, _ := c.Request.Context().Value(ctxkey.ClientRequestID).(string)
				trace.Finish(clientID, c.Writer.Status())
			}
		}()
		c.Next()
	}
}

func beginLatencyCollection(c *gin.Context) {
	if c.Request == nil || latencytrace.From(c.Request.Context()) != nil {
		return
	}
	key, ok := GetAPIKeyFromContext(c)
	if !ok || key == nil || key.GroupID == nil {
		return
	}
	ctx := c.Request.Context()
	start, ok := ctx.Value(ctxkey.RequestStartedAt).(time.Time)
	if !ok {
		start = time.Now()
	}
	requestID, _ := ctx.Value(ctxkey.RequestID).(string)
	ctx, trace := latencytrace.Begin(ctx, key.UserID, *key.GroupID, requestID, start)
	if trace == nil {
		return
	}
	c.Request = c.Request.WithContext(ctx)
	if preread, ok := c.Request.Body.(*pkghttputil.PrereadBody); ok {
		// Keep the zero-copy/replay behavior of an already buffered body.
		trace.AddRequestBytes(len(preread.Bytes()))
	} else if c.Request.Body != nil {
		c.Request.Body = &latencyRequestBody{ReadCloser: c.Request.Body, trace: trace}
	}
	c.Writer = &latencyResponseWriter{ResponseWriter: c.Writer, trace: trace}
}

type latencyRequestBody struct {
	io.ReadCloser
	trace *latencytrace.Trace
}

func (b *latencyRequestBody) Read(p []byte) (int, error) {
	b.trace.Mark("body_read_start")
	n, err := b.ReadCloser.Read(p)
	b.trace.AddRequestBytes(n)
	if err == io.EOF {
		b.trace.Mark("body_read_complete")
	} else if err != nil {
		b.trace.Mark("body_read_failed")
	}
	return n, err
}

type latencyResponseWriter struct {
	gin.ResponseWriter
	trace   *latencytrace.Trace
	wrote   bool
	flushed bool
}

func (w *latencyResponseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *latencyResponseWriter) Write(p []byte) (int, error) {
	if !w.wrote {
		w.trace.Mark("downstream_write_start")
	}
	n, err := w.ResponseWriter.Write(p)
	if !w.wrote && n > 0 {
		w.trace.Mark("downstream_first_write")
		w.wrote = true
	}
	return n, err
}
func (w *latencyResponseWriter) WriteString(p string) (int, error) {
	if !w.wrote {
		w.trace.Mark("downstream_write_start")
	}
	n, err := w.ResponseWriter.WriteString(p)
	if !w.wrote && n > 0 {
		w.trace.Mark("downstream_first_write")
		w.wrote = true
	}
	return n, err
}
func (w *latencyResponseWriter) Flush() {
	w.ResponseWriter.Flush()
	if !w.flushed {
		w.trace.Mark("downstream_first_flush")
		w.flushed = true
	}
}
