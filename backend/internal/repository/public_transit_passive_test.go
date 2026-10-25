package repository

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestPublicTransitPassiveAggregatesSkipsUnassignedPlatform(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Date(2026, 10, 25, 16, 0, 0, 0, time.UTC)
	since := now.Add(-24 * time.Hour)
	rows := sqlmock.NewRows([]string{
		"bucket_start", "platform", "group_id", "group_name", "model",
		"request_count", "success_count", "error_count", "latency_sum", "latency_samples",
		"ttft_sum", "ttft_samples", "input_tokens", "cache_creation_tokens", "cache_read_tokens", "last_request_at",
	}).
		AddRow(now, "openai", 1, "GPT", "gpt-5", 3, 2, 1, 600, 3, 90, 2, 100, 10, 20, now).
		AddRow(now, nil, nil, "", "unknown", 1, 0, 1, 10, 1, 0, 0, 0, 0, 0, now).
		AddRow(now, "   ", nil, "", "unknown", 1, 0, 1, 10, 1, 0, 0, 0, 0, 0, nil).
		AddRow(now, "gemini", 2, "Vertex", "gemini-2.5-pro", 2, 2, 0, 800, 2, 100, 2, 200, 0, 30, now)
	mock.ExpectQuery("WITH successes AS").WithArgs(since, now, int64(3600)).WillReturnRows(rows).RowsWillBeClosed()

	repo := &opsRepository{db: db}
	got, err := repo.GetPublicTransitPassiveAggregates(context.Background(), since, now, 3600)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "openai", got[0].Platform)
	require.Equal(t, int64(3), got[0].RequestCount)
	require.Equal(t, int64(1), got[0].ErrorCount)
	require.Equal(t, "gemini", got[1].Platform)
	require.Equal(t, "Vertex", got[1].GroupName)
	require.Equal(t, "gemini-2.5-pro", got[1].Model)
	require.Equal(t, int64(2), got[1].SuccessCount)
	require.NoError(t, mock.ExpectationsWereMet())
}
