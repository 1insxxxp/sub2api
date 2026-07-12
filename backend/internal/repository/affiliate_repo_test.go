package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestAffiliateUserOverviewSQLIncludesMaturedFrozenQuota(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")

	require.Contains(t, query, "ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0)")
	require.Contains(t, query, "frozen_until <= NOW()")
}

func TestAffiliateRecordQueriesUseLedgerAuditFields(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "JOIN payment_orders po ON po.id = ual.source_order_id")
	require.Contains(t, content, "ual.amount::double precision")
	require.Contains(t, content, "ual.balance_after::double precision")
	require.NotContains(t, content, "parseAffiliateRebateAmount")
	require.NotContains(t, content, `"current_balance": "u.balance"`)
}

func TestAffiliateQualificationReconcileSQLMatchesAuthoritativePaymentRules(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateQualificationReconcileSQL), " ")

	require.Contains(t, query, "FOR UPDATE")
	require.Contains(t, query, "FROM payment_orders po CROSS JOIN locked")
	require.Contains(t, query, "po.order_type IN ('balance', 'subscription')")
	require.Contains(t, query, "'COMPLETED', 'REFUND_REQUESTED', 'REFUNDING', 'REFUND_PENDING', 'REFUND_FAILED'")
	require.Contains(t, query, "WHEN po.status = 'PARTIALLY_REFUNDED' THEN GREATEST(po.amount - po.refund_amount, 0)")
	require.Contains(t, query, "WHEN po.status = 'REFUNDED' THEN 0")
	require.Contains(t, query, "GREATEST(COALESCE((SELECT SUM(net_amount) FROM authoritative_orders), 0), 0)")
	require.Contains(t, query, "WHEN totals.amount >= $2 AND locked.qualifying_payment_amount < $2")
	require.Contains(t, query, "WHEN totals.amount < $2 THEN NULL")
}

func TestAffiliateQualificationCountSQLUsesCurrentThreshold(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateQualificationCountSQL), " ")

	require.Contains(t, query, "inviter_id = $1")
	require.Contains(t, query, "qualifying_payment_amount >= $2")
	require.NotContains(t, query, "qualified_at IS NOT NULL")
	require.NotContains(t, query, "aff_count")
}

func TestAffiliateQualificationReconcileAllReturnsRowsIterationError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client, db: db}
	wantErr := errors.New("rows iteration failed")

	mock.ExpectQuery("SELECT user_id FROM user_affiliates").
		WithArgs(int64(0), 2).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).
			AddRow(int64(11)).
			AddRow(int64(12)).
			RowError(1, wantErr))

	err = repo.ReconcileAllAffiliateQualifications(context.Background(), 50, 2)
	require.ErrorIs(t, err, wantErr)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateQualificationReconcileAllRejectsOuterTransaction(t *testing.T) {
	repo := &affiliateRepository{}
	txCtx := dbent.NewTxContext(context.Background(), &dbent.Tx{})

	err := repo.ReconcileAllAffiliateQualifications(txCtx, 50, 10)

	require.Error(t, err)
	require.Contains(t, err.Error(), "transaction")
}

func TestAffiliateQualificationAdvisoryLockSkipsCallbackWhenBusy(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &affiliateRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_try_advisory_xact_lock").
		WithArgs(affiliateQualificationReconcileAdvisoryLockKey).
		WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	mock.ExpectRollback()
	called := false
	acquired, err := repo.TryWithAffiliateQualificationReconcileLock(context.Background(), func(context.Context) error {
		called = true
		return nil
	})

	require.NoError(t, err)
	require.False(t, acquired)
	require.False(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateQualificationAdvisoryLockCommitsAfterCallback(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	repo := &affiliateRepository{db: db}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_try_advisory_xact_lock").
		WithArgs(affiliateQualificationReconcileAdvisoryLockKey).
		WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(true))
	mock.ExpectCommit()
	called := false
	acquired, err := repo.TryWithAffiliateQualificationReconcileLock(context.Background(), func(callbackCtx context.Context) error {
		called = true
		require.Nil(t, dbent.TxFromContext(callbackCtx))
		return nil
	})

	require.NoError(t, err)
	require.True(t, acquired)
	require.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}
