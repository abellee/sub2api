package handler

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func conversationForwardModel(account *service.Account, mapping service.ChannelMappingResult, model string) string {
	if mapping.Mapped && mapping.MappedModel != "" {
		model = mapping.MappedModel
	}
	return account.GetMappedModel(model)
}

// Capture billing metadata before the handler moves on. Background accounting
// must not retain or read the live Gin context or the original request body.
func conversationUsageSnapshot(c *gin.Context, apiKey *service.APIKey, account *service.Account, subscription *service.UserSubscription, body []byte, pricingAt time.Time, fields service.ChannelUsageFields, updater service.APIKeyQuotaUpdater) service.RecordUsageInput {
	return service.RecordUsageInput{
		APIKey: apiKey, User: apiKey.User, Account: account, Subscription: subscription,
		PricingAt: pricingAt, InboundEndpoint: GetInboundEndpoint(c), UpstreamEndpoint: GetUpstreamEndpoint(c, account.Platform),
		UserAgent: c.GetHeader("User-Agent"), IPAddress: ip.GetClientIP(c), SessionID: service.ExtractClientSessionID(c),
		RequestPayloadHash: service.HashUsageRequestPayload(body), QuotaPlatform: service.QuotaPlatform(c.Request.Context(), apiKey),
		APIKeyService: updater, ChannelUsageFields: fields,
	}
}

func lateGatewayConversationBilling(c *gin.Context, gateway *service.GatewayService, updater service.APIKeyQuotaUpdater, apiKey *service.APIKey, account *service.Account, subscription *service.UserSubscription, body []byte, pricingAt time.Time, fields service.ChannelUsageFields, forceCache bool) func(context.Context, *service.ForwardResult) {
	if !service.ConversationBackgroundBillingEnabled(c.Request.Context()) {
		return nil
	}
	input := conversationUsageSnapshot(c, apiKey, account, subscription, body, pricingAt, fields, updater)
	input.ForceCacheBilling = forceCache
	return func(ctx context.Context, result *service.ForwardResult) {
		input.Result = result
		if result.UpstreamEndpoint != "" {
			input.UpstreamEndpoint = result.UpstreamEndpoint
		}
		if err := gateway.RecordUsage(ctx, &input); err != nil {
			logger.FromContext(ctx).Error("gateway.late_conversation_billing_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}
	}
}

func lateOpenAIConversationBilling(c *gin.Context, gateway *service.OpenAIGatewayService, updater service.APIKeyQuotaUpdater, apiKey *service.APIKey, account *service.Account, subscription *service.UserSubscription, body []byte, pricingAt time.Time, fields service.ChannelUsageFields) func(context.Context, *service.OpenAIForwardResult) {
	if !service.ConversationBackgroundBillingEnabled(c.Request.Context()) {
		return nil
	}
	snapshot := conversationUsageSnapshot(c, apiKey, account, subscription, body, pricingAt, fields, updater)
	input := service.OpenAIRecordUsageInput{
		APIKey: snapshot.APIKey, User: snapshot.User, Account: snapshot.Account, Subscription: snapshot.Subscription,
		PricingAt: snapshot.PricingAt, InboundEndpoint: snapshot.InboundEndpoint, UpstreamEndpoint: snapshot.UpstreamEndpoint,
		UserAgent: snapshot.UserAgent, IPAddress: snapshot.IPAddress, SessionID: snapshot.SessionID,
		RequestPayloadHash: snapshot.RequestPayloadHash, QuotaPlatform: snapshot.QuotaPlatform,
		APIKeyService: snapshot.APIKeyService, ChannelUsageFields: snapshot.ChannelUsageFields,
	}
	return func(ctx context.Context, result *service.OpenAIForwardResult) {
		input.Result = result
		if result.UpstreamEndpoint != "" {
			input.UpstreamEndpoint = result.UpstreamEndpoint
		}
		if err := gateway.RecordUsage(ctx, &input); err != nil {
			logger.FromContext(ctx).Error("gateway.late_conversation_billing_failed", zap.Int64("account_id", account.ID), zap.Error(err))
		}
	}
}
