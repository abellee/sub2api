package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/latencytrace"
	"github.com/stretchr/testify/require"
)

func TestLatencyStoppedCollectionRejectsQueuedRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT stopped_at IS NULL,record_count .* FOR UPDATE`).WithArgs("collection").WillReturnRows(sqlmock.NewRows([]string{"active", "count"}).AddRow(false, 9))
	mock.ExpectRollback()
	repo := &opsRepository{db: db}
	require.NoError(t, repo.InsertLatencyRecord(context.Background(), &latencytrace.Record{CollectionID: "collection"}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLatencyInsertLocksAndCapsCollection(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT stopped_at IS NULL,record_count .* FOR UPDATE`).WithArgs("collection").WillReturnRows(sqlmock.NewRows([]string{"active", "count"}).AddRow(true, 9999))
	mock.ExpectExec(`INSERT INTO ops_latency_records`).WithArgs("collection", "request", "client", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE ops_latency_collections SET record_count=record_count\+1,[\s\S]*record_count\+1 >= 10000[\s\S]*'limit'`).WithArgs("collection").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	repo := &opsRepository{db: db}
	require.NoError(t, repo.InsertLatencyRecord(context.Background(), &latencytrace.Record{CollectionID: "collection", RequestID: "request", ClientRequestID: "client"}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLatencyCreateRejectsMissingScopeAndDuplicate(t *testing.T) {
	for _, valid := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing", true: "duplicate"}[valid], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() {
				mock.ExpectClose()
				require.NoError(t, db.Close())
			})
			mock.ExpectBegin()
			mock.ExpectExec(`SELECT pg_advisory_xact_lock`).WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(`SELECT EXISTS`).WithArgs(int64(2), int64(145), int64(748)).WillReturnRows(sqlmock.NewRows([]string{"valid"}).AddRow(valid))
			if valid {
				mock.ExpectQuery(`SELECT count`).WillReturnRows(sqlmock.NewRows([]string{"count", "duplicate"}).AddRow(1, true))
			}
			mock.ExpectRollback()
			repo := &opsRepository{db: db}
			err = repo.CreateLatencySession(context.Background(), &latencytrace.Session{Scope: latencytrace.Scope{UserID: 2, GroupID: 145, AccountID: 748}})
			if valid {
				require.ErrorIs(t, err, latencytrace.ErrActiveLimit)
			} else {
				require.ErrorIs(t, err, latencytrace.ErrInvalidScope)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestLatencyRequestSearchAcceptsClientPrefixAndCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() {
		mock.ExpectClose()
		require.NoError(t, db.Close())
	})
	mock.ExpectQuery(`SELECT id,detail FROM ops_latency_records`).WithArgs("collection", "client:abc", "abc", int64(50)).WillReturnRows(sqlmock.NewRows([]string{"id", "detail"}).AddRow(49, `{"collection_id":"collection","client_request_id":"abc","events":[],"attempts":[]}`))
	repo := &opsRepository{db: db}
	items, err := repo.ListLatencyRecords(context.Background(), "collection", "client:abc", 50)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(49), items[0].ID)
	require.NoError(t, mock.ExpectationsWereMet())
}
