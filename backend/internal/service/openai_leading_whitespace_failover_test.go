package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOpenAILeadingWhitespaceFailureRetriesBeforeOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, enabled := range []bool{false, true} {
			t.Run(fmt.Sprintf("passthrough=%t/enabled=%t", passthrough, enabled), func(t *testing.T) {
				t.Parallel()
				ctx := context.Background()
				if enabled {
					ctx = withRequestAccountFailoverCount(ctx, 2)
				}
				rec := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(rec)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
				svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
				account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
				calls := 0
				_, err := ForwardWithAccountRetries(ctx, c, func() (*OpenAIForwardResult, error) {
					calls++
					stream := `data: {"type":"response.created","response":{"id":"failed-attempt"}}` + "\n\n" +
						`data: {"type":"response.output_text.delta","delta":" "}` + "\n\n" +
						`data: {"type":"response.output_text.done","text":" "}` + "\n\n" +
						`data: {"type":"response.output_item.done","item":{"type":"message","content":[{"type":"output_text","text":" "}]}}` + "\n\n" +
						`data: {"type":"response.failed","response":{"status":"failed","error":{"code":"upstream_error","message":"Upstream service temporarily unavailable"}}}` + "\n\n"
					resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(stream))}
					var err error
					if passthrough {
						_, err = svc.handleStreamingResponsePassthrough(ctx, resp, c, account, time.Now(), "model", "model")
					} else {
						_, err = svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "model", "model")
					}
					return nil, err
				})
				var failure *UpstreamFailoverError
				if enabled {
					require.ErrorAs(t, err, &failure)
					require.Equal(t, 3, calls)
					require.True(t, failure.SkipAccountCooldown)
					require.False(t, failure.RetryableOnSameAccount)
					require.Empty(t, rec.Body.String(), "failed attempts must remain private")
				} else {
					require.Error(t, err)
					require.False(t, errors.As(err, &failure))
					require.Equal(t, 1, calls)
					require.Contains(t, rec.Body.String(), "response.failed")
				}
			})
		}
	}
}

func TestOpenAILeadingWhitespacePreservesSuccessfulOutputAndStopsReplay(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, terminal := range []string{"response.completed", "response.failed"} {
			for _, content := range []string{
				`{"type":"response.output_text.delta","delta":"hello"}`,
				`{"type":"response.function_call_arguments.delta","delta":" "}`,
				`{"type":"response.custom_tool_call_input.delta","delta":" "}`,
			} {
				t.Run(fmt.Sprintf("passthrough=%t/%s/%s", passthrough, terminal, content), func(t *testing.T) {
					ctx := withRequestAccountFailoverCount(context.Background(), 2)
					rec := httptest.NewRecorder()
					c, _ := gin.CreateTestContext(rec)
					c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
					svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}
					account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
					calls := 0
					_, err := ForwardWithAccountRetries(ctx, c, func() (*OpenAIForwardResult, error) {
						calls++
						stream := "data: {\"type\":\"response.output_text.delta\",\"delta\":\" \\n\"}\n\n" +
							"data: " + content + "\n\n" +
							`data: {"type":"` + terminal + `","response":{"status":"` + strings.TrimPrefix(terminal, "response.") + `","usage":{"input_tokens":1,"output_tokens":1},"error":{"code":"upstream_error","message":"unavailable"}}}` + "\n\n"
						resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(stream))}
						var err error
						if passthrough {
							_, err = svc.handleStreamingResponsePassthrough(ctx, resp, c, account, time.Now(), "model", "model")
						} else {
							_, err = svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "model", "model")
						}
						return nil, err
					})
					require.Equal(t, 1, calls)
					if terminal == "response.completed" {
						require.NoError(t, err)
					} else {
						require.Error(t, err)
						var failure *UpstreamFailoverError
						require.False(t, errors.As(err, &failure))
					}
					var deltas []string
					for _, line := range strings.Split(rec.Body.String(), "\n") {
						if strings.HasPrefix(line, "data: ") {
							data := strings.TrimPrefix(line, "data: ")
							require.True(t, json.Valid([]byte(data)))
							if gjson.Get(data, "type").String() == "response.output_text.delta" {
								deltas = append(deltas, gjson.Get(data, "delta").String())
							}
						}
					}
					require.NotEmpty(t, deltas)
					require.Equal(t, " \n", deltas[0], "leading whitespace must be preserved when output commits")
				})
			}
		}
	}
}

func TestOpenAIStreamIdleTimeoutRetriesOnlyBeforeOutput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name                       string
		enabled, output, keepalive bool
		wantCalls                  int
	}{
		{name: "whitespace only retries", enabled: true, wantCalls: 1},
		{name: "heartbeat allows retry", enabled: true, keepalive: true, wantCalls: 1},
		{name: "real output never replayed", enabled: true, output: true, wantCalls: 1},
		{name: "disabled unchanged", wantCalls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if tt.enabled {
				ctx = withRequestAccountFailoverCount(ctx, 2)
			}
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
			cfg := &config.Config{Gateway: config.GatewayConfig{StreamDataIntervalTimeout: 1, MaxLineSize: defaultMaxLineSize}}
			if tt.keepalive {
				cfg.Gateway.StreamKeepaliveInterval = 1
			}
			svc := &OpenAIGatewayService{cfg: cfg}
			account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
			calls := 0
			_, err := ForwardWithAccountRetries(ctx, c, func() (*OpenAIForwardResult, error) {
				calls++
				pr, pw := io.Pipe()
				body := &firstOutputCloseTrackingBody{ReadCloser: pr, closed: make(chan struct{})}
				done := make(chan struct{})
				go func() {
					defer close(done)
					defer func() { _ = pw.Close() }()
					text := " "
					if tt.output {
						text = "hello"
					}
					_, _ = fmt.Fprintf(pw, "data: {\"type\":\"response.output_text.delta\",\"delta\":%q}\n\n", text)
					select {
					case <-body.closed:
					case <-ctx.Done():
					}
				}()
				resp := &http.Response{StatusCode: 200, Header: http.Header{}, Body: body}
				_, err := svc.handleStreamingResponse(ctx, resp, c, account, time.Now(), "model", "model")
				if tt.enabled && !tt.output {
					select {
					case <-body.closed:
					default:
						t.Error("pre-output timeout must close the upstream body")
					}
				}
				_ = body.Close()
				<-done
				return nil, err
			})
			require.Equal(t, tt.wantCalls, calls)
			var failure *UpstreamFailoverError
			if tt.enabled && !tt.output {
				require.ErrorAs(t, err, &failure)
				require.Equal(t, http.StatusGatewayTimeout, failure.StatusCode)
				require.True(t, failure.SkipAccountCooldown)
				require.True(t, failure.SafeToFailoverAfterWrite)
				require.NotContains(t, rec.Body.String(), "data:")
			} else {
				require.Error(t, err)
				require.False(t, errors.As(err, &failure))
				require.Contains(t, rec.Body.String(), "stream_timeout")
			}
		})
	}
}
