package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const accountInflationSecretKey = "account_inflation_secret"

type InflationDirectRequest struct {
	BaseURL           string `json:"base_url"`
	APIKey            string `json:"api_key"`
	Protocol          string `json:"protocol"`
	ModelID           string `json:"model_id"`
	Prompt            string `json:"prompt"`
	TokenizerEncoding string `json:"tokenizer_encoding"`
	TimeoutSeconds    int    `json:"timeout_seconds"`
}

func accountInflationTimeout(seconds int) (time.Duration, error) {
	if seconds == 0 {
		seconds = 120
	}
	if seconds < 1 || seconds > 3600 {
		return 0, fmt.Errorf("Timeout must be between 1 and 3600 seconds")
	}
	return time.Duration(seconds) * time.Second, nil
}

// Manual probes use the shared transport and parsers without creating or updating an account.
func (s *AccountTestService) ProbeInflationDirect(c *gin.Context, input InflationDirectRequest) error {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	key := strings.TrimSpace(input.APIKey)
	c.Set(accountInflationSecretKey, key)
	if key == "" || len(key) > 4096 || strings.ContainsAny(key, "\r\n") {
		return s.sendErrorAndEnd(c, "Invalid API key")
	}
	if strings.TrimSpace(input.Prompt) == "" || len(input.Prompt) > 32000 {
		return s.sendErrorAndEnd(c, "Prompt must contain 1 to 32000 bytes")
	}
	if input.TokenizerEncoding != "o200k_base" && input.TokenizerEncoding != "cl100k_base" {
		return s.sendErrorAndEnd(c, "Unsupported tokenizer encoding")
	}
	model := strings.TrimSpace(input.ModelID)
	if model == "" || len(model) > 200 {
		return s.sendErrorAndEnd(c, "A text model ID is required (maximum 200 bytes)")
	}
	duration, err := accountInflationTimeout(input.TimeoutSeconds)
	if err != nil {
		return s.sendErrorAndEnd(c, err.Error())
	}
	if len(input.BaseURL) > 2048 {
		return s.sendErrorAndEnd(c, "Invalid API address")
	}
	base, err := s.validateUpstreamBaseURL(input.BaseURL)
	if err != nil {
		return s.sendErrorAndEnd(c, "API address is invalid or disallowed by server policy")
	}
	u, err := url.Parse(base)
	if err != nil || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return s.sendErrorAndEnd(c, "API address must not contain credentials, query parameters or fragments")
	}
	var endpoint string
	var payload map[string]any
	switch input.Protocol {
	case "anthropic":
		endpoint = buildOpenAIEndpointURL(base, "/v1/messages")
		payload = map[string]any{"model": model, "messages": []map[string]any{{"role": "user", "content": input.Prompt}}, "max_tokens": 1024, "stream": true}
	case "chat_completions":
		endpoint = buildOpenAIChatCompletionsURL(base)
		payload = createOpenAIChatCompletionsTestPayload(model, input.Prompt)
	case "responses":
		endpoint = buildOpenAIResponsesURL(base)
		payload = map[string]any{"model": model, "input": input.Prompt, "stream": true}
	default:
		return s.sendErrorAndEnd(c, "Unsupported API protocol")
	}
	c.Set(accountInflationKey, &accountInflationProbe{prompt: input.Prompt, encoding: input.TokenizerEncoding})
	applyInflationPrompt(c, payload)
	body, err := json.Marshal(payload)
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to build request")
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), duration)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	ctx = WithHTTPUpstreamRedirectsDisabled(ctx)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return s.sendErrorAndEnd(c, "Failed to create request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if input.Protocol == "anthropic" {
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	} else {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	if s.httpUpstream == nil {
		return s.sendErrorAndEnd(c, "Upstream transport unavailable")
	}
	s.sendEvent(c, TestEvent{Type: "test_start", Model: model})
	resp, err := s.httpUpstream.DoWithTLS(req, "", 0, 1, nil)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return s.sendErrorAndEnd(c, fmt.Sprintf("Request timed out after %d seconds", int(duration/time.Second)))
		}
		return s.sendErrorAndEnd(c, "Upstream connection failed")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return s.sendErrorAndEnd(c, fmt.Sprintf("Upstream returned HTTP %d", resp.StatusCode))
	}
	switch input.Protocol {
	case "anthropic":
		return s.processClaudeStream(c, resp.Body)
	case "chat_completions":
		return s.processOpenAIChatCompletionsStream(c, resp.Body)
	default:
		return s.processOpenAIStream(c, resp.Body)
	}
}
