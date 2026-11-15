package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func (r *usageLogRepository) GetAPIAccountSummary(ctx context.Context, baseURL string, start, end time.Time, granularity, tz string) (*usagestats.APIAccountSummary, error) {
	result := &usagestats.APIAccountSummary{Accounts: []usagestats.APIAccountSummaryItem{}, Periods: []usagestats.APIAccountPeriod{}}
	// Read only addresses, never API keys or other credentials.
	rows, err := r.sql.QueryContext(ctx, `SELECT id, name, COALESCE(credentials->>'base_url', ''),
		COALESCE(credentials->'api_base_urls', '{}'::jsonb)
		FROM accounts WHERE deleted_at IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	ids := []int64{}
	accountIndexes := map[int64]int{}
	for rows.Next() {
		var id int64
		var name, rawURL string
		var protocolJSON []byte
		if err := rows.Scan(&id, &name, &rawURL, &protocolJSON); err != nil {
			_ = rows.Close()
			return nil, err
		}
		addresses := map[string]string{}
		// Legacy malformed protocol maps do not invalidate a valid primary address.
		_ = json.Unmarshal(protocolJSON, &addresses)
		addresses["base_url"] = rawURL
		for _, address := range addresses {
			if normalized, err := service.NormalizeAccountSummaryURL(address); err == nil && normalized == baseURL {
				accountIndexes[id] = len(result.Accounts)
				result.Accounts = append(result.Accounts, usagestats.APIAccountSummaryItem{AccountID: id, Name: name})
				ids = append(ids, id)
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return result, nil
	}

	// One scan and one snapshot for the totals, account breakdown and time buckets.
	rows, err = r.sql.QueryContext(ctx, `WITH usage AS (
		SELECT account_id, TO_CHAR(created_at AT TIME ZONE $4, $5) AS period,
			input_tokens + output_tokens + cache_creation_tokens + cache_read_tokens AS tokens,
			actual_cost AS user_cost, total_cost AS standard_cost,
			COALESCE(account_stats_cost, total_cost) * COALESCE(account_rate_multiplier, 1) AS account_cost
		FROM usage_logs WHERE account_id = ANY($1) AND created_at >= $2 AND created_at < $3
	)
	SELECT CASE WHEN GROUPING(account_id) = 0 THEN 'account'
		WHEN GROUPING(period) = 0 THEN 'period' ELSE 'summary' END,
		COALESCE(account_id, 0), COALESCE(period, ''), COUNT(*), COALESCE(SUM(tokens), 0),
		COALESCE(SUM(user_cost), 0), COALESCE(SUM(account_cost), 0),
		COALESCE(SUM(standard_cost), 0), COALESCE(SUM(user_cost - account_cost), 0)
	FROM usage GROUP BY GROUPING SETS ((account_id), (period), ()) ORDER BY 1, 2, 3`,
		pq.Array(ids), start, end, tz, safeDateFormat(granularity))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kind, period string
		var id int64
		var totals usagestats.APIAccountTotals
		if err := rows.Scan(&kind, &id, &period, &totals.Requests, &totals.Tokens,
			&totals.UserCost, &totals.AccountCost, &totals.StandardCost, &totals.Margin); err != nil {
			return nil, err
		}
		switch kind {
		case "summary":
			result.Summary = totals
		case "account":
			result.Accounts[accountIndexes[id]].APIAccountTotals = totals
		case "period":
			result.Periods = append(result.Periods, usagestats.APIAccountPeriod{Period: period, APIAccountTotals: totals})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
