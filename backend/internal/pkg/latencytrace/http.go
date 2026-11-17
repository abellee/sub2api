package latencytrace

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptrace"
)

// ObserveHTTP composes with existing httptrace callbacks. No URLs, headers,
// payloads, or error strings (which can contain credentials) are retained.
func ObserveHTTP(req *http.Request, account int64) (*http.Request, func(*http.Response, error)) {
	noop := func(*http.Response, error) {}
	if req == nil {
		return req, noop
	}
	a := From(req.Context()).StartAttempt(account, "http", req.ContentLength)
	if a == nil {
		return req, noop
	}
	trace := &httptrace.ClientTrace{
		GetConn:           func(string) { a.Mark("connection_wait_start") },
		GotConn:           func(info httptrace.GotConnInfo) { a.Reused(info.Reused); a.Mark("connection_ready") },
		DNSStart:          func(httptrace.DNSStartInfo) { a.Mark("dns_start") },
		DNSDone:           func(httptrace.DNSDoneInfo) { a.Mark("dns_done") },
		ConnectStart:      func(string, string) { a.Mark("connect_start") },
		ConnectDone:       func(string, string, error) { a.Mark("connect_done") },
		TLSHandshakeStart: func() { a.Mark("tls_start") },
		TLSHandshakeDone:  func(tls.ConnectionState, error) { a.Mark("tls_done") },
		WroteHeaders:      func() { a.Mark("request_headers_sent") },
		WroteRequest: func(info httptrace.WroteRequestInfo) {
			if info.Err == nil {
				a.Mark("upload_complete")
			} else {
				a.Mark("upload_failed")
			}
		},
		GotFirstResponseByte: func() { a.Mark("first_response_byte") },
	}
	req = req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
	return req, func(resp *http.Response, err error) {
		status := 0
		if resp != nil {
			status = resp.StatusCode
		}
		a.Done(status, err != nil)
		if resp != nil && resp.Body != nil {
			resp.Body = &observedBody{ReadCloser: resp.Body, attempt: a}
		}
	}
}

type observedBody struct {
	io.ReadCloser
	attempt *AttemptTrace
	first   bool
}

func (b *observedBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if n > 0 && !b.first {
		b.first = true
		b.attempt.Mark("first_body_byte")
	}
	if err == io.EOF {
		b.attempt.Mark("upstream_body_complete")
	}
	return n, err
}
