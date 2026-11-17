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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiSuccessfulEmptyResponseBillsPerRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, model := range []string{"gemini-3-flash-preview", "gemini-3-pro-image-preview"} {
		for _, protocol := range []string{"native", "messages", "chat"} {
			for _, format := range []string{"json", "stream"} {
				t.Run(model+"/"+protocol+"/"+format, func(t *testing.T) {
					stream := format == "stream"
					body := []byte(`{"model":"` + model + `","max_tokens":16,"messages":[{"role":"user","content":"hi"}]}`)
					if stream {
						body = bytes.Replace(body, []byte(`"max_tokens":16`), []byte(`"max_tokens":16,"stream":true`), 1)
					}
					c, _ := gin.CreateTestContext(httptest.NewRecorder())
					c.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
					svc, upstream := newGeminiVertexAPIKeyTestService()
					response := `{}`
					if stream {
						// A completed empty response, with neither content nor usage metadata.
						response = "data: {\"candidates\":[{\"finishReason\":\"STOP\"}]}\n\n"
					}
					upstream.response = &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(response))}
					var result *ForwardResult
					var err error
					switch protocol {
					case "native":
						action := "generateContent"
						if stream {
							action = "streamGenerateContent"
						}
						result, err = svc.ForwardNative(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), model, action, stream, []byte(`{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`))
					case "messages":
						result, err = svc.Forward(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
					case "chat":
						result, err = svc.ForwardAsChatCompletions(c.Request.Context(), c, geminiVertexAPIKeyTestAccount(""), body)
					}
					require.NoError(t, err)
					require.NotNil(t, result, "successful empty responses must reach per-request billing")
					require.Equal(t, ClaudeUsage{}, result.Usage)
					require.Zero(t, result.SearchCount)
					require.Zero(t, observedGeminiImageOutputs(c))
					for _, source := range []string{PricingSourceGroup, PricingSourceChannel} {
						t.Run(source, func(t *testing.T) {
							usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
							users := &openAIRecordUsageUserRepoStub{}
							billing := newGatewayRecordUsageServiceForTest(usageRepo, users, &openAIRecordUsageSubRepoStub{})
							groupID := int64(77)
							price := 0.25
							pricing := ChannelModelPricing{Models: []string{model}, BillingMode: BillingModePerRequest, PerRequestPrice: &price}
							group := &Group{ID: groupID, Platform: PlatformGemini, Status: StatusActive, Hydrated: true, RateMultiplier: 0.5}
							if source == PricingSourceGroup {
								group.ModelPricing = []ChannelModelPricing{pricing}
								billing.resolver = NewModelPricingResolver(nil, billing.billingService)
							} else {
								billing.resolver = newOpenAIImageChannelPricingResolverForTest(t, groupID, model, price)
								cache := billing.resolver.channelService.cache.Load().(*channelCache)
								cache.pricingByGroupModel[channelModelKey{groupID: groupID, model: model}] = &pricing
							}
							require.NoError(t, billing.RecordUsage(context.Background(), &RecordUsageInput{
								Result: result, APIKey: &APIKey{ID: 10, GroupID: &groupID, Group: group},
								User: &User{ID: 2}, Account: geminiVertexAPIKeyTestAccount(""),
							}))
							require.Equal(t, 1, usageRepo.calls)
							require.Equal(t, 1, users.deductCalls)
							require.NotNil(t, usageRepo.lastLog)
							require.NotNil(t, usageRepo.lastLog.BillingMode)
							require.Equal(t, string(BillingModePerRequest), *usageRepo.lastLog.BillingMode)
							require.InDelta(t, price, usageRepo.lastLog.TotalCost, 1e-12)
							require.InDelta(t, price*group.RateMultiplier, usageRepo.lastLog.ActualCost, 1e-12)
							require.InDelta(t, price*group.RateMultiplier, users.lastAmount, 1e-12)
						})
					}
				})
			}
		}
	}
}
