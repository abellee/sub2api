package repository

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBatchDuplicateAccountsRepositoryCommitsOrRollsBackTogether(t *testing.T) {
	for _, failSecond := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "rollback"}[failSecond], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			repo := newAccountRepositoryWithSQL(client, nil, nil)
			accounts := []*service.Account{
				{Name: "source-1", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Credentials: map[string]any{"api_key": "secret"}},
				{Name: "source-2", Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Credentials: map[string]any{"api_key": "secret"}},
			}
			mock.ExpectBegin()
			for i := range accounts {
				mock.ExpectQuery(`INSERT INTO "accounts"`).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(42 + i)))
				outbox := mock.ExpectExec(`INSERT INTO scheduler_outbox`)
				if failSecond && i == 1 {
					outbox.WillReturnError(errors.New("outbox failed"))
				} else {
					outbox.WillReturnResult(sqlmock.NewResult(1, 1))
				}
			}
			if failSecond {
				mock.ExpectRollback()
			} else {
				mock.ExpectCommit()
			}
			err = repo.CreateManyWithAccountGroups(context.Background(), accounts)
			if failSecond {
				require.ErrorContains(t, err, "outbox failed")
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
