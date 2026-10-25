//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type geminiDrainHTTPUpstream struct {
	HTTPUpstream
	client *http.Client
	url    string
}

func (u *geminiDrainHTTPUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	clone := req.Clone(req.Context())
	clone.URL, _ = req.URL.Parse(u.url)
	clone.Host = ""
	return u.client.Do(clone)
}

// Use real HTTP streaming: the downstream cancels while the upstream is still
// generating, and its final usage arrives only after the cancellation.
func TestGeminiBillingDrain_CollectsLateUsageAfterClientCancel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages", "native"} {
		t.Run(protocol, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			upstreamSurvived := make(chan bool, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, geminiAbortTextChunk)
				w.(http.Flusher).Flush()
				cancel()
				select {
				case <-r.Context().Done():
					upstreamSurvived <- false
					return
				case <-time.After(20 * time.Millisecond):
				}
				upstreamSurvived <- true
				_, _ = io.WriteString(w, "data: "+geminiSearchResponse+"\n\n"+geminiAbortFinalChunk)
			}))
			defer server.Close()
			svc := &GeminiMessagesCompatService{httpUpstream: &geminiDrainHTTPUpstream{client: server.Client(), url: server.URL}, cfg: &config.Config{}}
			body := []byte(`{"model":"gemini-3-flash-preview","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
			originalWriter := c.Writer
			var result *ForwardResult
			var err error
			switch protocol {
			case "chat":
				result, err = svc.ForwardAsChatCompletions(ctx, c, geminiVertexAPIKeyTestAccount(""), body)
			case "messages":
				result, err = svc.Forward(ctx, c, geminiVertexAPIKeyTestAccount(""), body)
			case "native":
				body = []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`)
				result, err = svc.ForwardNative(ctx, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", "streamGenerateContent", true, body)
			}
			require.NoError(t, err)
			require.True(t, <-upstreamSurvived, "client cancellation must not cancel dispatched upstream")
			require.NotNil(t, result)
			require.True(t, result.ClientDisconnect)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Equal(t, 5, result.Usage.OutputTokens)
			require.Equal(t, 2, result.SearchCount)
			assertGeminiGoogleSearchBill(t, result)
			require.NotContains(t, recorder.Body.String(), "world", "late output must only be drained for billing")
			require.Same(t, originalWriter, c.Writer)
			require.Empty(t, GetOpsStreamErrors(c))

			// Feed the recovered result through the real billing service, using
			// the same detached context as asynchronous usage recording.
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			userRepo := &openAIRecordUsageUserRepoStub{}
			billing := newGatewayRecordUsageServiceForTest(usageRepo, userRepo, &openAIRecordUsageSubRepoStub{})
			billing.resolver = NewModelPricingResolver(nil, billing.billingService)
			groupID := int64(105)
			key := &APIKey{ID: 1278, GroupID: &groupID, Group: &Group{
				ID: groupID, Platform: PlatformGemini, Status: StatusActive, Hydrated: true, RateMultiplier: 1.75,
				ModelPricing: []ChannelModelPricing{{
					Models: []string{result.Model}, BillingMode: BillingModeToken,
					InputPrice: testPtrFloat64(0.05), OutputPrice: testPtrFloat64(0.1),
				}},
			}}
			billingCtx, billingCancel := detachedBillingContext(ctx)
			defer billingCancel()
			require.NoError(t, billing.RecordUsage(billingCtx, &RecordUsageInput{
				Result: result, APIKey: key, User: &User{ID: 857}, Account: geminiVertexAPIKeyTestAccount(""),
			}))
			require.Equal(t, 1, usageRepo.calls)
			require.Equal(t, 1, userRepo.deductCalls)
			require.Equal(t, 1.75, usageRepo.lastLog.RateMultiplier)
			require.Greater(t, usageRepo.lastLog.ActualCost, 0.0)
			require.InDelta(t, usageRepo.lastLog.TotalCost*1.75, userRepo.lastAmount, 1e-12)
			require.NoError(t, usageRepo.lastCtxErr)
			require.NoError(t, userRepo.lastCtxErr)
		})
	}
}

func TestGeminiBillingDrain_WriteFailureStillCollectsFinalUsage(t *testing.T) {
	body := []byte(`{"model":"gemini-3-flash-preview","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	c, _ := gin.CreateTestContext(&geminiFailingResponseWriter{header: http.Header{}})
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	svc, stub := newGeminiVertexAPIKeyTestService()
	stub.response = &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(geminiAbortTextChunk + geminiAbortFinalChunk))}
	result, err := svc.ForwardAsChatCompletions(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
	require.NoError(t, err)
	require.True(t, result.ClientDisconnect)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
}

func TestGeminiBillingDrain_AlreadyCanceledNeverDispatches(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	body := []byte(`{"model":"gemini-3-flash-preview","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	svc, stub := newGeminiVertexAPIKeyTestService()
	result, err := svc.ForwardAsChatCompletions(ctx, c, geminiVertexAPIKeyTestAccount(""), body)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, result)
	require.Zero(t, stub.calls)
	require.Empty(t, GetOpsStreamErrors(c), "requests never dispatched do not need reconciliation")
}

func TestGeminiBillingDrain_MissingUsageMarksReconciliation(t *testing.T) {
	body := []byte(`{"model":"gemini-3-flash-preview","stream":true,"messages":[{"role":"user","content":"hi"}]}`)
	svc, c, _ := newGeminiAbortingStreamService(t, "/v1/chat/completions", body, geminiAbortTextChunk, true)
	result, err := svc.ForwardAsChatCompletions(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
	require.Error(t, err)
	require.Nil(t, result)
	marks := GetOpsStreamErrors(c)
	require.Len(t, marks, 1)
	require.Equal(t, "gemini_usage_unavailable_after_disconnect", marks[0].Code)
	require.True(t, marks[0].RequestScoped)
	require.Equal(t, 499, marks[0].IntendedStatus)
}

func TestGeminiBillingDrain_BoundsHungUpstreamAndCleansUp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil).WithContext(ctx)
	upstream, drain := beginGeminiBillingDrain(ctx, c, true, nil)
	drain.limit = 20 * time.Millisecond
	defer drain.finish(c, geminiVertexAPIKeyTestAccount(""), nil)
	cancel()
	select {
	case <-upstream.Done():
	case <-time.After(time.Second):
		t.Fatal("billing drain did not cancel a hung upstream")
	}
	require.ErrorIs(t, upstream.Err(), context.Canceled)
}
