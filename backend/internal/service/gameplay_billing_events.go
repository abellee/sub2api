package service

import "lotteryd/api/events"

func billingEventData(cmd *UsageBillingCommand, p *postUsageBillingParams) map[string]string {
	data := map[string]string{"api_key_id": events.S(cmd.APIKeyID), "account_id": events.S(cmd.AccountID), "model": cmd.Model, "total_cost": events.Amount(p.Cost.TotalCost), "actual_cost": events.Amount(p.Cost.ActualCost), "input_tokens": events.S(cmd.InputTokens), "output_tokens": events.S(cmd.OutputTokens), "image_count": events.S(cmd.ImageCount), "platform": p.Platform, "source_id": cmd.RequestID}
	if p.APIKey.GroupID != nil {
		data["group_id"] = events.S(*p.APIKey.GroupID)
	}
	if cmd.SubscriptionID != nil {
		data["subscription_id"] = events.S(*cmd.SubscriptionID)
	}
	return data
}
func emitBillingEvent(kind string, cmd *UsageBillingCommand, p *postUsageBillingParams, result *UsageBillingApplyResult) {
	if !events.Enabled() || cmd == nil {
		return
	}
	data := billingEventData(cmd, p)
	ref := cmd.RequestID + ":" + events.S(cmd.APIKeyID)
	if result != nil && result.NewBalance != nil {
		data["balance_after"] = events.Amount(*result.NewBalance)
	}
	events.Publish(kind, cmd.UserID, ref, data)
	if kind == "billing.completed" && result != nil {
		if result.APIKeyQuotaExhausted {
			events.Publish("key.exhausted", cmd.UserID, ref, data)
		}
		if result.BalanceOverdrafted {
			events.Publish("billing.overdrafted", cmd.UserID, ref, data)
		}
		if q := result.QuotaState; q != nil {
			crossed := (q.TotalLimit > 0 && q.TotalUsed >= q.TotalLimit && q.TotalUsed-cmd.AccountQuotaCost < q.TotalLimit) || (q.DailyLimit > 0 && q.DailyUsed >= q.DailyLimit && q.DailyUsed-cmd.AccountQuotaCost < q.DailyLimit) || (q.WeeklyLimit > 0 && q.WeeklyUsed >= q.WeeklyLimit && q.WeeklyUsed-cmd.AccountQuotaCost < q.WeeklyLimit)
			if crossed {
				copy := make(map[string]string, len(data)+1)
				for k, v := range data {
					copy[k] = v
				}
				copy["status"] = "exhausted"
				events.Publish("account.quota_changed", cmd.UserID, ref, copy)
			}
		}
		if result.NewBalance != nil && p.User != nil && p.User.BalanceNotifyThreshold != nil {
			threshold := *p.User.BalanceNotifyThreshold
			if *result.NewBalance < threshold && *result.NewBalance+cmd.BalanceCost >= threshold {
				events.Publish("billing.balance_low", cmd.UserID, ref, data)
			}
		}
	}
}
