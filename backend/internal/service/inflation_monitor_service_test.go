package service

import (
	"context"
	"database/sql"
	"math"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInflationMonitorStatistics(t *testing.T) {
	a, b, c := 0.0, 20.0, 100.0
	samples := []InflationMonitorSample{{Rate: &a}, {Rate: &b}, {Rate: &c}, {Rate: &c, Error: "failed"}, {}}
	require.InDelta(t, 40, *InflationAggregate(samples, "mean"), .0001)
	require.Equal(t, 20.0, *InflationAggregate(samples, "median"))
	require.Equal(t, 10.0, *InflationAggregate(samples[:2], "median"))
	require.Nil(t, InflationAggregate(samples[3:], "mean"))
}
func TestInflationMonitorPlanValidation(t *testing.T) {
	base := InflationMonitorPlan{GroupID: 27, Enabled: true, IntervalMinutes: 60, AccountIDs: []int64{1}, Model: "gpt-test", Prompt: DefaultInflationPrompt, Encoding: "o200k_base", Aggregation: "mean", YellowThreshold: 10, RedThreshold: 30}
	require.NoError(t, validateInflationPlan(base))
	for _, mutate := range []func(*InflationMonitorPlan){
		func(p *InflationMonitorPlan) { p.IntervalMinutes = 0 },
		func(p *InflationMonitorPlan) { p.IntervalMinutes = 1441 },
		func(p *InflationMonitorPlan) { p.AccountIDs = nil },
		func(p *InflationMonitorPlan) { p.AccountIDs = []int64{1, 1} },
		func(p *InflationMonitorPlan) { p.Model = "" },
		func(p *InflationMonitorPlan) { p.Encoding = "other" },
		func(p *InflationMonitorPlan) { p.Aggregation = "last" },
		func(p *InflationMonitorPlan) { p.RedThreshold = p.YellowThreshold },
		func(p *InflationMonitorPlan) { p.YellowThreshold = math.NaN() },
	} {
		p := base
		mutate(&p)
		require.Error(t, validateInflationPlan(p))
	}
}
func TestInflationMonitorSaveRetains24RoundsAndFencesStaleRuns(t *testing.T) {
	for _, current := range []bool{true, false} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		s := NewInflationMonitorService(db, nil)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT revision").WithArgs(int64(27), "lease", "revision").WillReturnRows(sqlmock.NewRows([]string{"current"}).AddRow(current))
		if current {
			mock.ExpectExec("INSERT INTO inflation_monitor_results").WithArgs(int64(27), sqlmock.AnyArg(), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectExec("DELETE FROM inflation_monitor_results.*LIMIT 24").WithArgs(int64(27)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectExec("UPDATE inflation_monitor_plans").WithArgs(int64(27)).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()
		} else {
			mock.ExpectRollback()
		}
		require.NoError(t, s.saveResult(context.Background(), 27, "lease", "revision", InflationMonitorResult{MeasuredAt: time.Now()}))
		require.NoError(t, mock.ExpectationsWereMet())
		require.NoError(t, db.Close())
	}
}
func TestInflationMonitorUserHistoryRequiresOwnedKeyAndRedactsAccounts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := NewInflationMonitorService(db, nil)
	plan := InflationMonitorPlan{GroupID: 27, Aggregation: "median", AccountIDs: []int64{233}, YellowThreshold: 10, RedThreshold: 30}
	mock.ExpectQuery("SELECT p.group_id,p.config.*k.user_id=\\$2").WithArgs(int64(88), int64(2)).WillReturnError(sql.ErrNoRows)
	_, err = s.History(context.Background(), 2, 88)
	require.ErrorIs(t, err, sql.ErrNoRows)
	mock.ExpectQuery("SELECT p.group_id,p.config.*k.user_id=\\$2").WithArgs(int64(89), int64(2)).WillReturnRows(sqlmock.NewRows([]string{"group_id", "config"}).AddRow(27, mustInflationJSON(plan)))
	rate := 5.0
	saved := InflationMonitorResult{Samples: []InflationMonitorSample{{AccountID: 233, Rate: &rate}}}
	mock.ExpectQuery("SELECT result FROM.*LIMIT 24").WithArgs(int64(27)).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow(mustInflationJSON(saved)))
	result, err := s.History(context.Background(), 2, 89)
	require.NoError(t, err)
	require.Equal(t, "median", result.Aggregation)
	require.Zero(t, result.Results[0].Samples[0].AccountID)
	require.NotContains(t, mustInflationJSON(result), "account_id")
	require.NoError(t, mock.ExpectationsWereMet())
}
func TestInflationMonitorCapturesInputAndConfiguredEffort(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	p := &accountInflationProbe{encoding: "o200k_base", reasoningEffort: "none"}
	c.Set(accountInflationKey, p)
	captureInflationUsage(c, map[string]any{"usage": map[string]any{"input_tokens": float64(100), "output_tokens": float64(20)}})
	require.Equal(t, 100, *p.result().InputTokens)
	payload := map[string]any{"input": "hi", "reasoning": map[string]any{"effort": "high"}}
	applyInflationPrompt(c, payload)
	require.Equal(t, "none", payload["reasoning"].(map[string]any)["effort"])
	payload = map[string]any{"messages": []any{}, "stream": true}
	applyInflationPrompt(c, payload)
	require.Equal(t, "none", payload["reasoning_effort"])
	captureInflationUsage(c, map[string]any{"usageMetadata": map[string]any{"promptTokenCount": float64(90)}})
	require.Equal(t, 90, *p.result().InputTokens)
}
func TestInflationMonitorSummaryUsesLatestRoundAndCurrentStatistic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := NewInflationMonitorService(db, nil)
	a, b, c := 0.0, 20.0, 100.0
	p := InflationMonitorPlan{Aggregation: "median", YellowThreshold: 5, RedThreshold: 15}
	r := InflationMonitorResult{MeasuredAt: time.Now(), Samples: []InflationMonitorSample{{Rate: &a}, {Rate: &b}, {Rate: &c}}}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT p.group_id,p.config,r.result FROM inflation_monitor_plans")).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"group_id", "config", "result"}).AddRow(27, mustInflationJSON(p), mustInflationJSON(r)))
	items, err := s.Summaries(context.Background(), 2)
	require.NoError(t, err)
	require.Equal(t, 20.0, *items[0].Rate)
	require.Equal(t, 15.0, items[0].RedThreshold)
	require.NoError(t, mock.ExpectationsWereMet())
	require.False(t, strings.Contains(mustInflationJSON(items), "account_id"))
}

func TestInflationMonitorStopCancelsRunningProbes(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	s := NewInflationMonitorService(db, nil)
	plan := InflationMonitorPlan{GroupID: 27, Enabled: true, IntervalMinutes: 1, AccountIDs: []int64{233}}
	mock.ExpectQuery("SELECT config FROM inflation_monitor_plans").WillReturnRows(sqlmock.NewRows([]string{"config"}).AddRow(mustInflationJSON(plan)))
	mock.ExpectQuery("UPDATE inflation_monitor_plans SET lease_token").WillReturnRows(sqlmock.NewRows([]string{"revision"}).AddRow("r"))
	mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
	mock.ExpectExec("UPDATE inflation_monitor_plans SET lease_until=NULL,lease_token=NULL WHERE group_id").WithArgs(int64(27), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	entered := make(chan struct{})
	s.probe = func(ctx context.Context, id int64, p InflationMonitorPlan) InflationMonitorSample {
		close(entered)
		<-ctx.Done()
		return InflationMonitorSample{Error: "canceled"}
	}
	s.Start()
	// Use the same lifecycle context as the scheduler while triggering a due scan now.
	ctx, cancel := context.WithCancel(context.Background())
	originalCancel := s.cancel
	s.cancel = func() { cancel(); originalCancel() }
	s.tick(ctx)
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("probe did not start")
	}
	stopped := make(chan struct{})
	go func() { s.Stop(); close(stopped) }()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("probe was not canceled")
	}
	require.NoError(t, mock.ExpectationsWereMet())
}
