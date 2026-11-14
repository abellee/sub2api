package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestScheduledRecoveryEnableAndSchedulingNotificationAreAtomic(t *testing.T) {
	for _, state := range []string{"success", "plan changed", "probe changed", "copy changed", "notification failed"} {
		t.Run(state, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			repo := &scheduledTestPlanRepository{db: db}
			version := time.Now()
			plan := &service.ScheduledTestPlan{ID: 7, UpdatedAt: version}
			probe := &service.Account{ID: 42, UpdatedAt: version}
			copies := []*service.Account{{ID: 43, UpdatedAt: version}, {ID: 44, UpdatedAt: version}}
			mock.ExpectBegin()
			rows := sqlmock.NewRows([]string{"id"})
			if state != "plan changed" {
				rows.AddRow(7)
			}
			mock.ExpectQuery(`(?s)SELECT id FROM scheduled_test_plans.*enabled = true AND recover_scheduling = true AND updated_at = \$2.*FOR UPDATE`).WithArgs(int64(7), version).WillReturnRows(rows)
			if state != "plan changed" {
				rows := sqlmock.NewRows([]string{"id"})
				if state != "probe changed" {
					rows.AddRow(42)
				}
				mock.ExpectQuery(`(?s)SELECT id FROM accounts WHERE id = \$1 AND updated_at = \$2 AND deleted_at IS NULL.*FOR UPDATE`).WithArgs(int64(42), version).WillReturnRows(rows)
			}
			if state != "plan changed" && state != "probe changed" {
				for _, copy := range copies {
					affected := int64(1)
					if state == "copy changed" && copy.ID == 44 {
						affected = 0
					}
					mock.ExpectExec(`(?s)UPDATE accounts SET schedulable = true.*WHERE id = \$1 AND updated_at = \$2 AND deleted_at IS NULL.*schedulable = false AND status = 'active'.*expires_at > NOW\(\)`).WithArgs(copy.ID, version).WillReturnResult(sqlmock.NewResult(0, affected))
					if affected == 0 {
						continue
					}
					expectation := mock.ExpectExec(`INSERT INTO scheduler_outbox`).WithArgs(service.SchedulerOutboxEventAccountChanged, copy.ID, nil, nil, sqlmock.AnyArg())
					if state == "notification failed" && copy.ID == 44 {
						expectation.WillReturnError(errors.New("outbox unavailable"))
					} else {
						expectation.WillReturnResult(sqlmock.NewResult(1, 1))
					}
				}
			}
			expected := 0
			if state == "success" || state == "copy changed" {
				mock.ExpectCommit()
				expected = 2
				if state == "copy changed" {
					expected = 1
				}
			} else {
				mock.ExpectRollback()
			}
			opened, err := repo.EnableScheduling(context.Background(), plan, probe, copies)
			if state == "notification failed" {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, expected, opened)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestScheduledRecoveryListsOnlyExactKeyCandidates(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &scheduledTestPlanRepository{db: db}
	account := &service.Account{ID: 42, Type: service.AccountTypeAPIKey, Credentials: map[string]any{"api_key": "test-key"}}
	mock.ExpectQuery(`(?s)SELECT id FROM accounts.*deleted_at IS NULL AND type = 'apikey' AND credentials ->> 'api_key' = \$1.*ORDER BY id`).WithArgs("test-key").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(42).AddRow(43))
	ids, err := repo.ListRecoveryAccountIDs(context.Background(), account)
	require.NoError(t, err)
	require.Equal(t, []int64{42, 43}, ids)
	for _, account := range []*service.Account{{ID: 42, Type: service.AccountTypeOAuth}, {ID: 42, Type: service.AccountTypeAPIKey}} {
		ids, err := repo.ListRecoveryAccountIDs(context.Background(), account)
		require.NoError(t, err)
		require.Equal(t, []int64{42}, ids)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledTestClaimAndAdvanceRespectConcurrentChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &scheduledTestPlanRepository{db: db}
	now := time.Now()
	lease := now.Add(6 * time.Minute)
	for _, affected := range []int64{1, 0} {
		mock.ExpectExec(`(?s)UPDATE scheduled_test_plans SET next_run_at = \$3.*WHERE id = \$1 AND enabled = true AND next_run_at <= \$2`).WithArgs(int64(7), now, lease).WillReturnResult(sqlmock.NewResult(0, affected))
		claimed, err := repo.ClaimDue(context.Background(), 7, now, lease)
		require.NoError(t, err)
		require.Equal(t, affected == 1, claimed)
	}
	mock.ExpectExec(`UPDATE scheduled_test_plans.*WHERE id = \$1 AND updated_at = \$4`).WithArgs(int64(7), now, lease, now).WillReturnResult(sqlmock.NewResult(0, 0))
	require.NoError(t, repo.UpdateAfterRun(context.Background(), 7, now, lease, now))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestScheduledTestPlanPersistsRecoveryOptIn(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &scheduledTestPlanRepository{db: db}
	now := time.Now()
	columns := []string{"id", "account_id", "model_id", "cron_expression", "enabled", "max_results", "auto_recover", "recover_scheduling", "recovery_probe_interval_seconds", "last_run_at", "next_run_at", "created_at", "updated_at"}
	newRows := func(enabled bool) *sqlmock.Rows {
		return sqlmock.NewRows(columns).AddRow(7, 42, "probe-model", "*/5 * * * *", true, 100, false, enabled, 17, nil, now, now, now)
	}
	plan := &service.ScheduledTestPlan{ID: 7, AccountID: 42, ModelID: "probe-model", CronExpression: "*/5 * * * *", Enabled: true, MaxResults: 100, RecoverScheduling: true, RecoveryProbeIntervalSeconds: 17, NextRunAt: &now}
	mock.ExpectQuery(`(?s)INSERT INTO scheduled_test_plans.*recover_scheduling.*RETURNING`).WithArgs(int64(42), "probe-model", "*/5 * * * *", true, 100, false, true, 17, now).WillReturnRows(newRows(true))
	created, err := repo.Create(context.Background(), plan)
	require.NoError(t, err)
	require.True(t, created.RecoverScheduling)
	require.Equal(t, 17, created.RecoveryProbeIntervalSeconds)
	plan.RecoverScheduling = false
	mock.ExpectQuery(`(?s)UPDATE scheduled_test_plans.*recover_scheduling = \$7.*RETURNING`).WithArgs(int64(7), "probe-model", "*/5 * * * *", true, 100, false, false, 17, now).WillReturnRows(newRows(false))
	updated, err := repo.Update(context.Background(), plan)
	require.NoError(t, err)
	require.False(t, updated.RecoverScheduling)
	require.Equal(t, 17, updated.RecoveryProbeIntervalSeconds)
	require.NoError(t, mock.ExpectationsWereMet())
}
