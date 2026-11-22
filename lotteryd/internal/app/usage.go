package app

import (
	"context"
	"errors"
	"fmt"
	"lotteryd/internal/cache"
	"time"

	"lotteryd/internal/lottery"
)

var ErrUsageUnavailable = errors.New("usage data unavailable")

type userUsageKey struct{}
type freshUsageKey struct{}

func FreshUserUsage(ctx context.Context) context.Context {
	return context.WithValue(ctx, freshUsageKey{}, true)
}

type userUsageAuth struct {
	userID int64
	bearer string
}

// WithUserUsageBearer binds the already authenticated identity to its bearer.
// Credentials are request-local and never persisted with usage snapshots.
func WithUserUsageBearer(ctx context.Context, userID int64, bearer string) context.Context {
	return context.WithValue(ctx, userUsageKey{}, userUsageAuth{userID, bearer})
}

func (ap *App) userUsageWindow(ctx context.Context, userID int64, email string, acts []lottery.Activity) (map[int64]*lottery.UserUsage, error) {
	days := 0
	for _, a := range acts {
		for _, c := range a.Conditions {
			if (c.Dimension == lottery.DimensionTokenUsage || c.Dimension == lottery.DimensionActiveDays) && c.WindowDays > days {
				days = c.WindowDays
			}
		}
	}
	usages := map[int64]*lottery.UserUsage{}
	if days == 0 {
		return usages, nil
	}
	auth, ok := ctx.Value(userUsageKey{}).(userUsageAuth)
	if !ok {
		// Internal callers without an authenticated request use the synced snapshot.
		return ap.usageWindow(ctx, days)
	}
	if auth.userID != userID || auth.bearer == "" {
		return nil, ErrUsageUnavailable
	}
	end := time.Now().In(lottery.Beijing()).AddDate(0, 0, -1)
	start := end.AddDate(0, 0, -(days - 1))
	load := func() ([]lottery.DailyToken, error) {
		return ap.Sub2API.FetchUserDailyTokens(ctx, auth.bearer, dateStr(start), dateStr(end))
	}
	var daily []lottery.DailyToken
	var err error
	if fresh, _ := ctx.Value(freshUsageKey{}).(bool); fresh {
		daily, err = load()
	} else {
		key := fmt.Sprintf("usage:%d:%s:%s:%s", userID, cache.Hash(auth.bearer), dateStr(start), dateStr(end))
		daily, err = cache.Load(ap.Store.Cache(), ctx, key, 15*time.Second, load)
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrUsageUnavailable, err)
	}
	usages[userID] = &lottery.UserUsage{UserID: userID, Email: email, Daily: daily}
	return usages, nil
}
