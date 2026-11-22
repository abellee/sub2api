package sub2api

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"time"

	"lotteryd/internal/lottery"
)

// FetchUserDailyTokens reads the bearer owner's authoritative daily totals,
// including input, output and cache tokens. Empty days are valid zero usage.
func (c *Client) FetchUserDailyTokens(ctx context.Context, bearer, start, end string) ([]lottery.DailyToken, error) {
	if c == nil || bearer == "" {
		return nil, fmt.Errorf("missing usage credentials")
	}
	q := url.Values{"start_date": {start}, "end_date": {end}, "granularity": {"day"}, "timezone": {"Asia/Shanghai"}}
	var data struct {
		Trend *[]struct {
			Date        string   `json:"date"`
			TotalTokens *float64 `json:"total_tokens"`
		} `json:"trend"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/v1/usage/dashboard/trend", q, nil, &data, bearer); err != nil {
		return nil, err
	}
	if data.Trend == nil {
		return nil, fmt.Errorf("missing usage trend")
	}
	out := make([]lottery.DailyToken, 0, len(*data.Trend))
	seen := map[string]bool{}
	for _, p := range *data.Trend {
		if _, err := time.Parse("2006-01-02", p.Date); err != nil || p.Date < start || p.Date > end || seen[p.Date] {
			return nil, fmt.Errorf("invalid usage date %q", p.Date)
		}
		if p.TotalTokens == nil || *p.TotalTokens < 0 || math.IsNaN(*p.TotalTokens) || math.IsInf(*p.TotalTokens, 0) {
			return nil, fmt.Errorf("invalid daily token total")
		}
		seen[p.Date] = true
		out = append(out, lottery.DailyToken{Date: p.Date, TotalTokens: *p.TotalTokens})
	}
	return out, nil
}
