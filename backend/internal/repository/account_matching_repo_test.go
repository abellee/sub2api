package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAccountMatchingRepositorySaveAndConflict(t *testing.T) {
	for _, conflict := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		repo := NewAccountMatchingRepository(db)
		rule := &service.AccountMatchingRule{UserID: 2, GroupIDs: []int64{10, 11}, AccountIDs: []int64{20, 21}}
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT email FROM users").WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"email"}).AddRow("user2@example.test"))
		mock.ExpectQuery("SELECT.*count").WithArgs("{10,11}", "{20,21}").WillReturnRows(sqlmock.NewRows([]string{"valid"}).AddRow(true))
		mock.ExpectQuery("SELECT EXISTS").WithArgs(int64(2), int64(0), "{10,11}").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(conflict))
		if conflict {
			mock.ExpectRollback()
			require.ErrorIs(t, repo.Save(context.Background(), rule), service.ErrAccountMatchingConflict)
		} else {
			now := time.Now()
			mock.ExpectQuery("INSERT INTO account_matching_rules").WithArgs(int64(2), "{10,11}", "{20,21}").WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(1, now, now))
			mock.ExpectCommit()
			require.NoError(t, repo.Save(context.Background(), rule))
			require.Equal(t, int64(1), rule.ID)
		}
		require.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	}
}

func TestAccountMatchingRepositoryLookupDistinguishesNoRuleAndFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := NewAccountMatchingRepository(db)
	mock.ExpectQuery("SELECT account_ids").WithArgs(int64(2), int64(10)).WillReturnRows(sqlmock.NewRows([]string{"account_ids"}).AddRow("{20,21}"))
	ids, err := repo.AllowedAccounts(context.Background(), 2, 10)
	require.NoError(t, err)
	require.Equal(t, []int64{20, 21}, ids)
	mock.ExpectQuery("SELECT account_ids").WithArgs(int64(2), int64(11)).WillReturnError(sql.ErrNoRows)
	ids, err = repo.AllowedAccounts(context.Background(), 2, 11)
	require.NoError(t, err)
	require.Nil(t, ids)
	mock.ExpectQuery("SELECT account_ids").WithArgs(int64(2), int64(10)).WillReturnError(sql.ErrConnDone)
	_, err = repo.AllowedAccounts(context.Background(), 2, 10)
	require.ErrorIs(t, err, sql.ErrConnDone)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAccountMatchingRepositoryRejectsInvalidSelections(t *testing.T) {
	for _, ids := range [][]int64{nil, {}, {0}, {-1}, {1, 1}} {
		require.False(t, validMatchingIDs(ids))
	}
	require.True(t, validMatchingIDs([]int64{1, 2}))
}
