package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestAPIAccountSummaryMatchesAddressesOnceAndKeepsIdleAccounts(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := newUsageLogRepositoryWithSQL(nil, db)
	mock.ExpectQuery(`(?s)SELECT id, name,.*FROM accounts WHERE deleted_at IS NULL`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "base_url", "api_base_urls"}).
			AddRow(1, "primary", "https://API.example.com/v1/", `{"responses":"https://api.example.com/v1"}`).
			AddRow(2, "idle protocol", "https://other.example.com", `{"responses":"https://api.example.com/v1"}`).
			AddRow(3, "different path", "https://api.example.com/v10", `{}`).
			AddRow(4, "different host", "https://api.example.com.attacker.test/v1", `{}`))
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	mock.ExpectQuery(`(?s)COALESCE\(account_stats_cost, total_cost\) \* COALESCE\(account_rate_multiplier, 1\).*created_at >= \$2 AND created_at < \$3.*GROUPING SETS`).
		WithArgs("{1,2}", start, end, "UTC", "YYYY-MM-DD").
		WillReturnRows(sqlmock.NewRows([]string{"kind", "id", "period", "requests", "tokens", "user_cost", "account_cost", "standard_cost", "margin"}).
			AddRow("account", 1, "", 2, 30, 1.2, 0.3, 6, 0.9).
			AddRow("period", 0, "2026-11-01", 2, 30, 1.2, 0.3, 6, 0.9).
			AddRow("summary", 0, "", 2, 30, 1.2, 0.3, 6, 0.9))
	result, err := repo.GetAPIAccountSummary(context.Background(), "https://api.example.com/v1", start, end, "day", "UTC")
	require.NoError(t, err)
	require.Len(t, result.Accounts, 2)
	require.Equal(t, result.Summary, result.Accounts[0].APIAccountTotals)
	require.Zero(t, result.Accounts[1].Requests)
	require.Equal(t, result.Summary, result.Periods[0].APIAccountTotals)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAPIAccountSummaryNoMatchAndQueryFailure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		db, mock := newSQLMock(t)
		repo := newUsageLogRepositoryWithSQL(nil, db)
		expect := mock.ExpectQuery(`SELECT id, name,`)
		if failed {
			expect.WillReturnError(errors.New("database unavailable"))
		} else {
			expect.WillReturnRows(sqlmock.NewRows([]string{"id", "name", "base_url", "api_base_urls"}))
		}
		result, err := repo.GetAPIAccountSummary(context.Background(), "https://api.example.com", time.Now(), time.Now(), "day", "UTC")
		if failed {
			require.Error(t, err)
		} else {
			require.NoError(t, err)
			require.Empty(t, result.Accounts)
			require.Empty(t, result.Periods)
			require.Zero(t, result.Summary.UserCost)
		}
		require.NoError(t, mock.ExpectationsWereMet())
	}
}
