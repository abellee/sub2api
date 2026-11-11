package repository

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/audit-sdk/auditclient"
	"github.com/Wei-Shaw/sub2api/audit-sdk/auditcore"
	"github.com/Wei-Shaw/sub2api/audit-sdk/auditrpc"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

func TestHTTPUpstreamAuditPreservesRetryStreamAndCopies(t *testing.T) {
	relay := httptest.NewServer(auditrpc.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }), false))
	defer relay.Close()
	dir := t.TempDir()
	client, err := auditclient.New(auditclient.Config{RelayURL: relay.URL, Token: strings.Repeat("t", 32), SpoolDir: dir, SpoolBytes: 8 << 20})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	client.SetPolicy(auditcore.Policy{Version: 1, Enabled: true})
	ctx, s := client.CaptureRequest(context.Background(), auditcore.Meta{UserID: 1, Model: "test"}, nil, []byte("client-input"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if string(b) != "request-input" || r.Header.Get("X-Copy") != "original" {
			t.Error("business request changed")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if r.Header.Get("X-Try") == "first" {
			w.WriteHeader(429)
			_, _ = io.WriteString(w, "retry")
			return
		}
		_, _ = io.WriteString(w, "data: hello\n\n")
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer upstream.Close()
	transport := NewHTTPUpstream(nil)
	for attempt := range 2 {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, strings.NewReader("request-input"))
		req.Header.Set("X-Copy", "original")
		var resp *http.Response
		if attempt == 0 {
			req.Header.Set("X-Try", "first")
			resp, err = transport.Do(req, "", 11, 2)
		} else {
			// The plain HTTP TLS-profile delegation must capture exactly once.
			resp, err = transport.DoWithTLS(req, "", 12, 2, &tlsfingerprint.Profile{Name: "unused-for-http"})
		}
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Copy", "reused")
		var result []byte
		buffer := make([]byte, 7)
		for {
			n, readErr := resp.Body.Read(buffer)
			result = append(result, buffer[:n]...)
			clear(buffer)
			if readErr != nil {
				if readErr != io.EOF {
					t.Fatal(readErr)
				}
				break
			}
		}
		if err := resp.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if attempt == 0 && (resp.StatusCode != 429 || string(result) != "retry") {
			t.Fatal("retry response changed")
		}
		if attempt == 1 && (resp.StatusCode != 200 || string(result) != "data: hello\n\ndata: [DONE]\n\n") {
			t.Fatal("stream changed")
		}
	}
	s.Complete(nil, 200, false)
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Shutdown(shutdown); err != nil {
		t.Fatal(err)
	}
	queue, err := auditcore.OpenJournalQueue(filepath.Join(dir, "pending"), 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Close()
	items, err := queue.Items(0)
	if err != nil {
		t.Fatal(err)
	}
	var attempts int
	var responses [3]string
	var last auditcore.CaptureFrame
	for i, item := range items {
		data, err := queue.Read(item.ID)
		if err != nil {
			t.Fatal(err)
		}
		f, err := auditcore.DecodeCaptureFrame(data)
		if err != nil {
			t.Fatal(err)
		}
		if f.Sequence != int64(i+1) {
			t.Fatal("capture reordered")
		}
		if e := f.Event; e.Direction == "upstream-request" && e.Headers != nil {
			attempts++
			if e.Attempt != attempts || e.AccountID != int64(10+attempts) || e.Headers.Get("X-Copy") != "original" {
				t.Fatal("retry metadata changed")
			}
		}
		if e := f.Event; e.Direction == "upstream-response" && e.Attempt > 0 && e.Attempt < 3 {
			responses[e.Attempt] += string(e.Body)
		}
		last = f
	}
	if attempts != 2 || responses[1] != "retry" || responses[2] != "data: hello\n\ndata: [DONE]\n\n" || last.Event.Kind != "end" {
		t.Fatal("retry, stream or final frame lost", attempts, responses)
	}
}
