package service

import (
	"context"
	"net/url"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
)

type APIAccountSummaryFilter struct {
	BaseURL     string
	StartDate   string
	EndDate     string
	Timezone    string
	Granularity string
}

type apiAccountSummaryReader interface {
	GetAPIAccountSummary(context.Context, string, time.Time, time.Time, string, string) (*usagestats.APIAccountSummary, error)
}

// NormalizeAccountSummaryURL preserves path boundaries; /v1 and /v2 are different API addresses.
func NormalizeAccountSummaryURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err == nil && u != nil {
		u.Scheme = strings.ToLower(u.Scheme)
	}
	if len(raw) > 2048 || err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", infraerrors.BadRequest("INVALID_API_ADDRESS", "请输入完整的 HTTP 或 HTTPS API 地址，不含密钥、查询参数或片段")
	}
	u.Scheme = strings.ToLower(u.Scheme)
	u.Host = strings.ToLower(u.Host)
	return strings.TrimRight(u.String(), "/"), nil
}

func parseAPIAccountSummaryFilter(f APIAccountSummaryFilter) (APIAccountSummaryFilter, time.Time, time.Time, error) {
	baseURL, err := NormalizeAccountSummaryURL(f.BaseURL)
	if err != nil {
		return f, time.Time{}, time.Time{}, err
	}
	f.BaseURL = baseURL
	if f.Timezone == "" {
		f.Timezone = timezone.Name()
		if f.Timezone == "Local" {
			f.Timezone = "UTC"
		}
	}
	loc, err := time.LoadLocation(f.Timezone)
	if err != nil || f.Timezone == "Local" {
		return f, time.Time{}, time.Time{}, infraerrors.BadRequest("INVALID_TIMEZONE", "时区无效")
	}
	if f.Granularity == "" {
		f.Granularity = "day"
	}
	switch f.Granularity {
	case "hour", "day", "week", "month":
	default:
		return f, time.Time{}, time.Time{}, infraerrors.BadRequest("INVALID_GRANULARITY", "请选择按小时、天、周或月汇总")
	}
	start, startErr := time.ParseInLocation(time.DateOnly, f.StartDate, loc)
	end, endErr := time.ParseInLocation(time.DateOnly, f.EndDate, loc)
	if startErr != nil || endErr != nil || end.Before(start) || !end.Before(start.AddDate(0, 0, 366)) {
		return f, time.Time{}, time.Time{}, infraerrors.BadRequest("INVALID_DATE_RANGE", "请选择有效日期范围，一次最多查询 366 天")
	}
	// The selected end date is inclusive, including on daylight-saving transitions.
	return f, start, end.AddDate(0, 0, 1), nil
}

func (s *AccountUsageService) GetAPIAccountSummary(ctx context.Context, filter APIAccountSummaryFilter) (*usagestats.APIAccountSummary, error) {
	f, start, end, err := parseAPIAccountSummaryFilter(filter)
	if err != nil {
		return nil, err
	}
	reader, ok := s.usageLogRepo.(apiAccountSummaryReader)
	if !ok {
		return nil, infraerrors.New(503, "SUMMARY_UNAVAILABLE", "统计汇总暂不可用")
	}
	result, err := reader.GetAPIAccountSummary(ctx, f.BaseURL, start, end, f.Granularity, f.Timezone)
	if err != nil {
		return nil, err
	}
	result.BaseURL, result.StartDate, result.EndDate = f.BaseURL, f.StartDate, f.EndDate
	result.Timezone, result.Granularity = f.Timezone, f.Granularity
	return result, nil
}
