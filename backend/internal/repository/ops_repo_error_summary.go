package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/querycache"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Live preset windows share a 30-second snapshot. Explicit historical windows
// remain exact; the overview reports the actual boundaries used by its query.
func cachedOpsFilter(filter *service.OpsDashboardFilter) service.OpsDashboardFilter {
	f := *filter
	f.StartTime, f.EndTime = f.StartTime.UTC(), f.EndTime.UTC()
	if f.GroupID != nil {
		id := *f.GroupID
		f.GroupID = &id
	}
	if delta := time.Since(f.EndTime); delta >= -5*time.Second && delta <= 5*time.Second {
		duration := f.EndTime.Sub(f.StartTime)
		for _, preset := range []time.Duration{5 * time.Minute, 15 * time.Minute, 30 * time.Minute, time.Hour, 6 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour} {
			if d := duration - preset; d > -time.Second && d < time.Second {
				f.EndTime = f.EndTime.Truncate(30 * time.Second)
				f.StartTime = f.EndTime.Add(-preset)
				break
			}
		}
	}
	return f
}

func opsStatisticsKey(filter *service.OpsDashboardFilter) string {
	data, _ := json.Marshal(filter)
	return string(data)
}

type opsErrorSummaryRow struct {
	Status                                   int
	Owner                                    string
	Total, SLA, BusinessLimited, ProviderSLA int64
}

// Both overview and distribution consume the same scan/classification of errors.
func (r *opsRepository) queryErrorSummary(ctx context.Context, filter *service.OpsDashboardFilter, start, end time.Time) ([]opsErrorSummaryRow, error) {
	f := *filter
	f.StartTime, f.EndTime = start.UTC(), end.UTC()
	return querycache.Load(ctx, &r.statisticsCache, "errors:"+opsStatisticsKey(&f), 30*time.Second, func(ctx context.Context) ([]opsErrorSummaryRow, error) {
		where, args, _ := buildErrorWhere(&f, f.StartTime, f.EndTime, 1)
		rows, err := r.db.QueryContext(ctx, `
SELECT COALESCE(upstream_status_code,status_code,0), COALESCE(error_owner,''),
 COUNT(*) FILTER (WHERE COALESCE(status_code,0) >= 400),
 COUNT(*) FILTER (WHERE COALESCE(status_code,0) >= 400 AND NOT is_business_limited AND is_sla_sample),
 COUNT(*) FILTER (WHERE COALESCE(status_code,0) >= 400 AND is_business_limited),
 COUNT(*) FILTER (WHERE NOT is_business_limited AND is_sla_sample)
FROM ops_error_log_metrics
`+where+` GROUP BY 1,2`, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		result := make([]opsErrorSummaryRow, 0)
		for rows.Next() {
			var row opsErrorSummaryRow
			if err := rows.Scan(&row.Status, &row.Owner, &row.Total, &row.SLA, &row.BusinessLimited, &row.ProviderSLA); err != nil {
				return nil, err
			}
			result = append(result, row)
		}
		return result, rows.Err()
	})
}
