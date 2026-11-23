//go:build unit

package service

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/stretchr/testify/require"
)

func directInflationInput() InflationDirectRequest {
	return InflationDirectRequest{BaseURL: "https://probe.example/v1", APIKey: "test-private-key", Protocol: "anthropic", ModelID: "test-model", Prompt: "Shared prompt", TokenizerEncoding: "o200k_base", TimeoutSeconds: 7}
}

func TestAccountInflationDirectProtocols(t *testing.T) {
	for _, tc := range []struct{ protocol, path, stream string }{
		{"anthropic", "/v1/messages", "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello world\"}}\n\ndata: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":4}}\n\ndata: {\"type\":\"message_stop\"}\n\n"},
		{"chat_completions", "/v1/chat/completions", "data: {\"choices\":[{\"delta\":{\"content\":\"Hello world\"}}]}\n\ndata: {\"usage\":{\"completion_tokens\":4}}\n\ndata: [DONE]\n\n"},
		{"responses", "/v1/responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello world\"}\n\ndata: {\"type\":\"response.completed\",\"response\":{\"usage\":{\"output_tokens\":4}}}\n\n"},
	} {
		t.Run(tc.protocol, func(t *testing.T) {
			upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(200, tc.stream)}}
			// No account repository: the direct mode cannot persist or change accounts.
			svc := &AccountTestService{cfg: &config.Config{}, httpUpstream: upstream}
			input := directInflationInput()
			input.Protocol = tc.protocol
			// Full endpoint addresses must not gain a second /v1 or suffix.
			input.BaseURL = "https://probe.example" + tc.path
			c, rec := newTestContext()
			require.NoError(t, svc.ProbeInflationDirect(c, input))
			require.Len(t, upstream.requests, 1)
			req := upstream.requests[0]
			require.Equal(t, tc.path, req.URL.Path)
			require.True(t, HTTPUpstreamRedirectsDisabled(req.Context()))
			deadline, ok := req.Context().Deadline()
			require.True(t, ok)
			require.InDelta(t, 7, time.Until(deadline).Seconds(), 1)
			if tc.protocol == "anthropic" {
				require.Equal(t, input.APIKey, req.Header.Get("x-api-key"))
				require.Empty(t, req.Header.Get("Authorization"))
			} else {
				require.Equal(t, "Bearer "+input.APIKey, req.Header.Get("Authorization"))
			}
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), "Shared prompt")
			require.NotContains(t, string(body), input.APIKey)
			if tc.protocol == "chat_completions" {
				require.Contains(t, string(body), `"include_usage":true`)
			}
			require.Contains(t, rec.Body.String(), `"token_count":2,"output_tokens":4`)
			require.NotContains(t, rec.Body.String(), input.APIKey)
		})
	}
}

func TestAccountInflationDirectValidation(t *testing.T) {
	for _, change := range []func(*InflationDirectRequest){
		func(r *InflationDirectRequest) { r.APIKey = "" },
		func(r *InflationDirectRequest) { r.APIKey = "key\r\nX-Test: injected" },
		func(r *InflationDirectRequest) { r.Protocol = "invalid" },
		func(r *InflationDirectRequest) { r.ModelID = "" },
		func(r *InflationDirectRequest) { r.Prompt = "" },
		func(r *InflationDirectRequest) { r.TokenizerEncoding = "invalid" },
		func(r *InflationDirectRequest) { r.TimeoutSeconds = -1 },
		func(r *InflationDirectRequest) { r.TimeoutSeconds = 3601 },
		func(r *InflationDirectRequest) { r.BaseURL = "file:///etc/passwd" },
		func(r *InflationDirectRequest) { r.BaseURL = "https://user:secret@probe.example" },
		func(r *InflationDirectRequest) { r.BaseURL = "https://probe.example?key=secret" },
	} {
		input := directInflationInput()
		change(&input)
		upstream := &queuedHTTPUpstream{}
		svc := &AccountTestService{cfg: &config.Config{}, httpUpstream: upstream}
		c, rec := newTestContext()
		require.Error(t, svc.ProbeInflationDirect(c, input))
		require.Empty(t, upstream.requests)
		require.Contains(t, rec.Body.String(), `"type":"error"`)
		require.NotContains(t, rec.Body.String(), "secret")
	}
}

type waitingInflationUpstream struct{ queuedHTTPUpstream }

func (u *waitingInflationUpstream) DoWithTLS(req *http.Request, _ string, _ int64, _ int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	<-req.Context().Done()
	return nil, req.Context().Err()
}

func TestAccountInflationDirectTimeoutAndRedaction(t *testing.T) {
	input := directInflationInput()
	input.TimeoutSeconds = 1
	svc := &AccountTestService{cfg: &config.Config{}, httpUpstream: &waitingInflationUpstream{}}
	c, rec := newTestContext()
	start := time.Now()
	require.Error(t, svc.ProbeInflationDirect(c, input))
	require.Less(t, time.Since(start), 3*time.Second)
	require.Contains(t, rec.Body.String(), "timed out after 1 seconds")
	stream := `data: {"type":"error","error":{"message":"bad key test-private-key"}}` + "\n\n"
	svc.httpUpstream = &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(200, stream)}}
	c, rec = newTestContext()
	require.Error(t, svc.ProbeInflationDirect(c, input))
	require.NotContains(t, rec.Body.String(), input.APIKey)
	require.Contains(t, rec.Body.String(), "[REDACTED]")
}

func TestAccountInflationSavedAccountTimeout(t *testing.T) {
	account := &Account{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Credentials: map[string]any{"access_token": "test-token"}}
	upstream := &queuedHTTPUpstream{responses: []*http.Response{newJSONResponse(200, "data: {\"type\":\"response.completed\"}\n\n")}}
	svc := &AccountTestService{accountRepo: &openAIAccountTestRepo{mockAccountRepoForGemini: mockAccountRepoForGemini{accountsByID: map[int64]*Account{2: account}}}, httpUpstream: upstream}
	c, _ := newTestContext()
	require.NoError(t, svc.TestAccountConnection(c, 2, "test-model", "prompt", "text", AccountTestOptions{MeasureTokens: true, TokenizerEncoding: "o200k_base", TimeoutSeconds: 9}))
	deadline, ok := upstream.requests[0].Context().Deadline()
	require.True(t, ok)
	require.InDelta(t, 9, time.Until(deadline).Seconds(), 1)
	c, _ = newTestContext()
	require.Error(t, svc.TestAccountConnection(c, 2, "test-model", "prompt", "text", AccountTestOptions{MeasureTokens: true, TokenizerEncoding: "o200k_base", TimeoutSeconds: -1}))
	require.Len(t, upstream.requests, 1)
}
