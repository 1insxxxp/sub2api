package repository

import (
	"context"
	"math"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestDeductOrdinaryBalanceUsesAtomicConditionalUpdate(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectQuery(`UPDATE users\s+SET balance = balance - \$1, updated_at = NOW\(\)\s+WHERE id = \$2 AND deleted_at IS NULL\s+AND balance - gift_balance >= \$1\s+RETURNING balance`).
		WithArgs(6.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(14.0))

	require.NoError(t, repo.DeductOrdinaryBalance(context.Background(), 42, 6))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductOrdinaryBalanceDistinguishesInsufficientOrdinaryBalance(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectQuery(`UPDATE users[\s\S]+AND balance - gift_balance >= \$1[\s\S]+RETURNING balance`).
		WithArgs(6.0, int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}))
	mock.ExpectQuery(`SELECT balance, gift_balance FROM users WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "gift_balance"}).AddRow(20.0, 15.0))

	err := repo.DeductOrdinaryBalance(context.Background(), 42, 6)

	require.ErrorIs(t, err, service.ErrBalanceNegative)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductOrdinaryBalanceDistinguishesMissingUser(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectQuery(`UPDATE users[\s\S]+AND balance - gift_balance >= \$1[\s\S]+RETURNING balance`).
		WithArgs(1.0, int64(404)).
		WillReturnRows(sqlmock.NewRows([]string{"balance"}))
	mock.ExpectQuery(`SELECT balance, gift_balance FROM users WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs(int64(404)).
		WillReturnRows(sqlmock.NewRows([]string{"balance", "gift_balance"}))

	err := repo.DeductOrdinaryBalance(context.Background(), 404, 1)

	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDeductOrdinaryBalanceRejectsInvalidAmountsBeforeSQL(t *testing.T) {
	for _, amount := range []float64{0, -1, math.NaN(), math.Inf(1), 0.000000001, 1_000_000_000_000} {
		repo, mock := newRedeemAdjustmentRepoMock(t)

		err := repo.DeductOrdinaryBalance(context.Background(), 42, amount)

		require.ErrorIs(t, err, service.ErrBalanceNegative)
		require.NoError(t, mock.ExpectationsWereMet())
	}
}

func newRedeemAdjustmentRepoMock(t *testing.T) (*userRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	return newUserRepositoryWithSQL(client, db), mock
}

func TestApplyRedeemBalanceAdjustment_UsesAtomicFloor(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectExec(`UPDATE users SET balance = GREATEST\(balance \+ \$1, 0\), updated_at = NOW\(\) WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(-7.0, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.ApplyRedeemBalanceAdjustment(context.Background(), 42, -7))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyRedeemConcurrencyAdjustment_UsesAtomicFloor(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectExec(`UPDATE users SET concurrency = GREATEST\(concurrency \+ \$1, 0\), updated_at = NOW\(\) WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(-7, int64(42)).
		WillReturnResult(sqlmock.NewResult(0, 1))

	require.NoError(t, repo.ApplyRedeemConcurrencyAdjustment(context.Background(), 42, -7))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestApplyRedeemAdjustment_MissingUser(t *testing.T) {
	repo, mock := newRedeemAdjustmentRepoMock(t)
	mock.ExpectExec(`UPDATE users SET balance = GREATEST\(balance \+ \$1, 0\), updated_at = NOW\(\) WHERE id = \$2 AND deleted_at IS NULL`).
		WithArgs(-1.0, int64(404)).
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := repo.ApplyRedeemBalanceAdjustment(context.Background(), 404, -1)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
