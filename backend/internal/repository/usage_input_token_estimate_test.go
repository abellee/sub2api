package repository

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUsageInputTokenEstimateInsertAndRead(t *testing.T) {
	for _, estimate := range []*service.InputTokenEstimate{nil, {Tokens: 0, Encoding: "o200k_base"}, {Tokens: 1234, Encoding: "o200k_base", Partial: true}} {
		t.Run(stringEstimateName(estimate), func(t *testing.T) {
			log := &service.UsageLog{UserID: 2, APIKeyID: 2, AccountID: 7, Model: "gpt-6-sol", RequestID: "tokenizer", InputTokenEstimate: estimate}
			prepared := prepareUsageLogInsert(log)
			columns := strings.Split(usageLogSelectColumns, ", ")
			index := -1
			for i, column := range columns {
				if column == "input_token_estimate" {
					index = i - 1
				}
			}
			require.GreaterOrEqual(t, index, 0)
			if estimate == nil {
				require.Nil(t, prepared.args[index])
			} else {
				payload, err := json.Marshal(estimate)
				require.NoError(t, err)
				storedJSON, ok := prepared.args[index].(string)
				require.True(t, ok)
				require.JSONEq(t, string(payload), storedJSON)
			}

			db, mock := newSQLMock(t)
			values := append([]driver.Value{int64(42)}, anySliceToDriverValues(prepared.args)...)
			mock.ExpectQuery("SELECT").WillReturnRows(sqlmock.NewRows(columns).AddRow(values...))
			rows, err := db.QueryContext(context.Background(), "SELECT usage")
			require.NoError(t, err)
			defer func() { _ = rows.Close() }()
			require.True(t, rows.Next())
			stored, err := scanUsageLog(rows)
			require.NoError(t, err)
			require.Equal(t, estimate, stored.InputTokenEstimate)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func stringEstimateName(estimate *service.InputTokenEstimate) string {
	if estimate == nil {
		return "absent"
	}
	if estimate.Tokens == 0 {
		return "zero"
	}
	return "partial"
}
