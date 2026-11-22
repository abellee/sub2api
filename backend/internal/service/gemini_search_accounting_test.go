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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const geminiSearchResponse = `{"candidates":[{"content":{"role":"model","parts":[{"text":"answer"}]},"groundingMetadata":{"webSearchQueries":["query one","query two"]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}`

func TestGeminiGoogleSearch_ForwardAndBill(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "messages", "native"} {
		for _, stream := range []bool{false, true} {
			t.Run(protocol+"/"+map[bool]string{true: "stream", false: "json"}[stream], func(t *testing.T) {
				body := []byte(`{"model":"gemini-3-flash-preview","max_tokens":16,"messages":[{"role":"user","content":"hi"}],"tools":[{"type":"web_search"}]}`)
				if stream {
					body = bytes.Replace(body, []byte(`"max_tokens":16`), []byte(`"max_tokens":16,"stream":true`), 1)
				}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
				svc, upstream := newGeminiVertexAPIKeyTestService()
				response := geminiSearchResponse
				if protocol == "messages" {
					response = `{"response":` + response + `}`
				}
				if stream {
					response = "data: " + response + "\n\ndata: " + response + "\n\ndata: {\"usageMetadata\":{\"promptTokenCount\":10,\"candidatesTokenCount\":5}}\n\n"
				}
				upstream.response = &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response))}
				var result *ForwardResult
				var err error
				switch protocol {
				case "chat":
					result, err = svc.ForwardAsChatCompletions(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
				case "messages":
					result, err = svc.Forward(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
				case "native":
					action := "generateContent"
					if stream {
						action = "streamGenerateContent"
					}
					result, err = svc.ForwardNative(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", action, stream, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"tools":[{"googleSearch":{}}]}`))
				}
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, 2, result.SearchCount, "repeated SSE snapshots must not multiply charges")
				require.Equal(t, "google", result.SearchProvider)
				assertGeminiGoogleSearchBill(t, result)
			})
		}
	}
}

func assertGeminiGoogleSearchBill(t *testing.T, result *ForwardResult) {
	t.Helper()
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	users := &openAIRecordUsageUserRepoStub{}
	billing := newGatewayRecordUsageServiceForTest(usageRepo, users, &openAIRecordUsageSubRepoStub{})
	billing.resolver = NewModelPricingResolver(nil, billing.billingService)
	groupID := int64(105)
	key := &APIKey{ID: 1278, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformGemini, Status: StatusActive, Hydrated: true, RateMultiplier: 1.75, SearchPricePer1k: testPtrFloat64(19.6), ModelPricing: []ChannelModelPricing{{Models: []string{result.Model}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(0.05), OutputPrice: testPtrFloat64(0.1)}}}}
	require.NoError(t, billing.RecordUsage(context.Background(), &RecordUsageInput{Result: result, APIKey: key, User: &User{ID: 857}, Account: geminiVertexAPIKeyTestAccount("")}))
	require.Equal(t, 1, usageRepo.calls)
	require.Equal(t, 1, users.deductCalls)
	log := usageRepo.lastLog
	require.Equal(t, result.SearchCount, log.SearchCount)
	require.InDelta(t, float64(result.SearchCount)*0.0196, log.SearchCost, 1e-12)
	require.InDelta(t, log.SearchCost+log.InputCost+log.OutputCost, log.TotalCost, 1e-12)
	require.InDelta(t, log.TotalCost*1.75, users.lastAmount, 1e-12)
	require.InDelta(t, log.ActualCost, users.lastAmount, 1e-12)
}

func TestGeminiGoogleSearch_ClientAbortWithOnlySearchMetadata(t *testing.T) {
	for _, protocol := range []string{"chat", "messages", "native"} {
		t.Run(protocol, func(t *testing.T) {
			body := []byte(`{"model":"gemini-3-flash-preview","max_tokens":16,"stream":true,"messages":[{"role":"user","content":"hi"}]}`)
			chunk := `{"candidates":[{"content":{"role":"model","parts":[{"text":"searching"}]},"groundingMetadata":{"webSearchQueries":["query one"]}}]}`
			if protocol == "messages" {
				chunk = `{"response":` + chunk + `}`
			}
			svc, c, _ := newGeminiAbortingStreamService(t, "/", body, "data: "+chunk+"\n\n", true)
			var result *ForwardResult
			var err error
			switch protocol {
			case "chat":
				result, err = svc.ForwardAsChatCompletions(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
			case "messages":
				result, err = svc.Forward(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
			case "native":
				result, err = svc.ForwardNative(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", "streamGenerateContent", true, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
			}
			require.NoError(t, err)
			require.Equal(t, 1, result.SearchCount)
			require.True(t, result.ClientDisconnect)
			assertGeminiGoogleSearchBill(t, result)
		})
	}
}

func TestGeminiGoogleSearch_CountOnlyObservedQueriesAndReset(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	beginGeminiSearchObservation(c)
	for _, payload := range []string{`{"tools":[{"googleSearch":{}}]}`, `{"candidates":[{"groundingMetadata":{"groundingChunks":[{"web":{"uri":"https://example.com"}}]}}]}`} {
		observeGeminiSearch(c, []byte(payload))
	}
	require.Zero(t, observedGeminiSearches(c), "advertised tools and citations are not search counts")
	observeGeminiSearch(c, []byte(geminiSearchResponse))
	observeGeminiSearch(c, []byte(geminiSearchResponse))
	observeGeminiSearch(c, []byte(`{"response":{"candidates":[{"grounding_metadata":{"web_search_queries":["query three",""]}}]}}`))
	require.Equal(t, 3, observedGeminiSearches(c))
	beginGeminiSearchObservation(c)
	require.Zero(t, observedGeminiSearches(c), "failover must not reuse the previous account's count")
}

func TestGeminiGoogleSearch_ExplicitRatesAndImageSurcharge(t *testing.T) {
	svc := &GatewayService{billingService: newTestBillingService()}
	result := &ForwardResult{Model: "claude-sonnet-4", SearchCount: 1, SearchProvider: "google", Usage: ClaudeUsage{InputTokens: 1000}}
	key := &APIKey{Group: &Group{}}
	for _, tc := range []struct {
		name  string
		price *float64
		want  float64
	}{
		{"unconfigured", nil, 0}, {"free", testPtrFloat64(0), 0}, {"configured", testPtrFloat64(19.6), 0.0196},
	} {
		t.Run(tc.name, func(t *testing.T) {
			key.Group.SearchPricePer1k = tc.price
			cost := svc.calculateRecordUsageCost(context.Background(), result, key, result.Model, 1.75, 2, time.Time{})
			require.InDelta(t, tc.want, cost.SearchCost, 1e-12)
			require.InDelta(t, (0.003+tc.want)*1.75, cost.ActualCost, 1e-12)
		})
	}
	result.ImageCount = 1
	result.Model = "gemini-3-pro-image"
	result.ImageSize = "1K"
	key.Group.SearchPricePer1k = testPtrFloat64(19.6)
	key.Group.ImagePrice1K = testPtrFloat64(0.1)
	cost := svc.calculateRecordUsageCost(context.Background(), result, key, result.Model, 1.75, 2, time.Time{})
	require.InDelta(t, 0.1196, cost.TotalCost, 1e-12)
	require.InDelta(t, 0.2+0.0343, cost.ActualCost, 1e-12)
}
