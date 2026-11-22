package app

import (
	"context"

	"lotteryd/internal/lottery"
)

func needsHistoricalRecharge(conditions []lottery.ConditionDef) bool {
	for _, condition := range conditions {
		if condition.Dimension == lottery.DimensionHistoricalRecharge {
			return true
		}
	}
	return false
}

// fillHistoricalRecharge caches only in the current request's usage map. Each
// participation request reads a fresh authoritative total from the main API.
func (ap *App) fillHistoricalRecharge(ctx context.Context, usages map[int64]*lottery.UserUsage, userID int64, email string, conditions []lottery.ConditionDef) error {
	if !needsHistoricalRecharge(conditions) {
		return nil
	}
	usage := usages[userID]
	if usage != nil && usage.TotalRecharged != nil {
		return nil
	}
	total, err := ap.Sub2API.FetchTotalRecharged(ctx, userID)
	if err != nil {
		return err
	}
	if usage == nil {
		usage = &lottery.UserUsage{UserID: userID, Email: email}
		usages[userID] = usage
	}
	usage.TotalRecharged = &total
	return nil
}
