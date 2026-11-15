//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAPIAccountSummaryAggregatesRecordedMoneyAndDateBoundaries(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)
	u := mustCreateUser(t, client, &service.User{Email: "api-summary@test.invalid"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: u.ID, Key: "sk-api-summary", Name: "summary"})
	baseURL := "https://summary.test.invalid/v1"
	account := mustCreateAccount(t, client, &service.Account{Name: "summary", Credentials: map[string]any{"base_url": baseURL + "/"}})
	idle := mustCreateAccount(t, client, &service.Account{Name: "idle", Status: service.StatusDisabled, Credentials: map[string]any{"base_url": baseURL}})
	deleted := mustCreateAccount(t, client, &service.Account{Name: "deleted", Credentials: map[string]any{"base_url": baseURL}})
	require.NoError(t, client.Account.UpdateOneID(deleted.ID).SetDeletedAt(time.Now()).Exec(ctx))
	loc, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	end := start.AddDate(0, 0, 2)
	multiplier, customCost := 0.2, 3.0
	logs := []*service.UsageLog{
		// Custom stats pricing is multiplied by the recorded account rate: 3 * 0.2.
		{AccountID: account.ID, CreatedAt: start, TotalCost: 10, ActualCost: 2, AccountStatsCost: &customCost, AccountRateMultiplier: &multiplier},
		// Historical rows with no multiplier or custom price fall back to total_cost.
		{AccountID: account.ID, CreatedAt: start.AddDate(0, 0, 1), TotalCost: 1, ActualCost: 0.1},
		// Unbilled usage can still have a cost; it must not disappear from the totals.
		{AccountID: account.ID, CreatedAt: start.Add(time.Hour), TotalCost: 2},
		{AccountID: account.ID, CreatedAt: start.Add(-time.Second), TotalCost: 100, ActualCost: 100},
		{AccountID: account.ID, CreatedAt: end, TotalCost: 100, ActualCost: 100},
		{AccountID: deleted.ID, CreatedAt: start, TotalCost: 100, ActualCost: 100},
	}
	for _, log := range logs {
		log.UserID, log.APIKeyID, log.Model, log.InputTokens = u.ID, key.ID, "summary-model", 10
		_, err := repo.Create(ctx, log)
		require.NoError(t, err)
	}
	result, err := repo.GetAPIAccountSummary(ctx, baseURL, start, end, "day", "Asia/Shanghai")
	require.NoError(t, err)
	require.Len(t, result.Accounts, 2)
	require.Equal(t, idle.ID, result.Accounts[1].AccountID)
	require.Zero(t, result.Accounts[1].Requests)
	require.Equal(t, int64(3), result.Summary.Requests)
	require.Equal(t, int64(30), result.Summary.Tokens)
	require.InDelta(t, 2.1, result.Summary.UserCost, 1e-8)
	require.InDelta(t, 3.6, result.Summary.AccountCost, 1e-8)
	require.InDelta(t, -1.5, result.Summary.Margin, 1e-8)
	require.InDelta(t, 13, result.Summary.StandardCost, 1e-8)
	require.Len(t, result.Periods, 2)
	require.Equal(t, "2026-11-01", result.Periods[0].Period)
	require.Equal(t, int64(2), result.Periods[0].Requests)
	for granularity, period := range map[string]string{"hour": "2026-11-01 00:00", "week": "2026-44", "month": "2026-11"} {
		grouped, err := repo.GetAPIAccountSummary(ctx, baseURL, start, end, granularity, "Asia/Shanghai")
		require.NoError(t, err)
		require.Equal(t, result.Summary, grouped.Summary)
		require.Equal(t, period, grouped.Periods[0].Period)
	}
}
