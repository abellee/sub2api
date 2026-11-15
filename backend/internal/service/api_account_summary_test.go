package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeAccountSummaryURL(t *testing.T) {
	for _, tc := range []struct{ raw, want string }{
		{" HTTPS://API.Example.com/v1/// ", "https://api.example.com/v1"},
		{"https://api.example.com", "https://api.example.com"},
		{"https://api.example.com/CaseSensitive", "https://api.example.com/CaseSensitive"},
		{"http://127.0.0.1:8080/v1/", "http://127.0.0.1:8080/v1"},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			got, err := NormalizeAccountSummaryURL(tc.raw)
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
	for _, raw := range []string{"", "api.example.com", "ftp://api.example.com", "https://key@api.example.com", "https://api.example.com?key=secret", "https://api.example.com#fragment", "https://api.example.com?"} {
		_, err := NormalizeAccountSummaryURL(raw)
		require.Error(t, err, raw)
	}
}

func TestParseAPIAccountSummaryFilter(t *testing.T) {
	f := APIAccountSummaryFilter{BaseURL: "https://api.example.com/v1/", StartDate: "2026-03-08", EndDate: "2026-03-08", Timezone: "America/New_York", Granularity: "hour"}
	got, start, end, err := parseAPIAccountSummaryFilter(f)
	require.NoError(t, err)
	require.Equal(t, "https://api.example.com/v1", got.BaseURL)
	require.Equal(t, time.Date(2026, 3, 8, 5, 0, 0, 0, time.UTC), start.UTC())
	require.Equal(t, 23*time.Hour, end.Sub(start))
	for _, tc := range []struct{ name, start, end, tz, granularity string }{
		{"reverse", "2026-03-09", "2026-03-08", "UTC", "day"},
		{"bad date", "2026-02-30", "2026-03-08", "UTC", "day"},
		{"too long", "2025-01-01", "2026-01-02", "UTC", "month"},
		{"bad timezone", "2026-03-08", "2026-03-08", "Invalid/Timezone", "day"},
		{"local timezone", "2026-03-08", "2026-03-08", "Local", "day"},
		{"bad dimension", "2026-03-08", "2026-03-08", "UTC", "injection"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, _, err := parseAPIAccountSummaryFilter(APIAccountSummaryFilter{BaseURL: f.BaseURL, StartDate: tc.start, EndDate: tc.end, Timezone: tc.tz, Granularity: tc.granularity})
			require.Error(t, err)
		})
	}
	f.StartDate, f.EndDate, f.Timezone = "2024-01-01", "2024-12-31", "UTC"
	_, _, _, err = parseAPIAccountSummaryFilter(f)
	require.NoError(t, err, "a full leap year fits the 366-day limit")
}
