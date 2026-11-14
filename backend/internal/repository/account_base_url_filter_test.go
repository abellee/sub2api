package repository

import (
	"context"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestAccountBaseURLFilterUsesBoundLiteralInCountAndPage(t *testing.T) {
	for _, filter := range []string{" api.example.com ", "https://api.example.com/v1", "api_%' OR 1=1 --"} {
		t.Run(filter, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			repo := newAccountRepositoryWithSQL(client, nil, nil)
			predicate := `STRPOS\(LOWER\(COALESCE\("accounts"\."credentials" ->> 'base_url', ''\)\), LOWER\(\$2\)\) > 0`
			mock.ExpectQuery(predicate).WithArgs("openai", strings.TrimSpace(filter)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			mock.ExpectQuery(predicate + `.*LIMIT 20`).WithArgs("openai", strings.TrimSpace(filter)).WillReturnRows(sqlmock.NewRows([]string{"id"}))
			items, page, err := repo.ListWithFilters(context.Background(), pagination.PaginationParams{Page: 1, PageSize: 20}, "openai", "", "", "", 0, "", filter)
			require.NoError(t, err)
			require.Empty(t, items)
			require.Zero(t, page.Total)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
