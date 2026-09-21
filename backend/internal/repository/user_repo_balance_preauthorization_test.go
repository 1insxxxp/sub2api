//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRepositoryListRecentBalanceCosts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	mock.ExpectQuery(`(?s)SELECT actual_cost.*FROM usage_logs.*billing_type = \$2.*COALESCE\(NULLIF\(requested_model, ''\), model\).*ORDER BY created_at DESC, id DESC.*LIMIT \$4`).
		WithArgs(int64(7), service.BillingTypeBalance, "gpt-5.6", 100).
		WillReturnRows(sqlmock.NewRows([]string{"actual_cost"}).AddRow(2.5).AddRow(1.25))

	repo := newUserRepositoryWithSQL(nil, db)
	costs, err := repo.ListRecentBalanceCosts(context.Background(), 7, "gpt-5.6", 100)
	require.NoError(t, err)
	require.Equal(t, []float64{2.5, 1.25}, costs)
	require.NoError(t, mock.ExpectationsWereMet())
}
