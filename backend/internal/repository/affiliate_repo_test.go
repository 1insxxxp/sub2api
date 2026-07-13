package repository

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestAffiliateUserOverviewSQLIncludesMaturedFrozenQuota(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")

	require.Contains(t, query, "ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0)")
	require.Contains(t, query, "frozen_until <= NOW()")
}

func TestAffiliateReadQueriesProjectDynamicQualificationFields(t *testing.T) {
	inviteesQuery := strings.Join(strings.Fields(affiliateInviteesSQL), " ")
	require.Contains(t, inviteesQuery, "ua.qualifying_payment_amount::double precision")
	require.Contains(t, inviteesQuery, "(ua.qualifying_payment_amount >= $3) AS qualified")
	require.Contains(t, inviteesQuery, "CASE WHEN ua.qualifying_payment_amount >= $3 THEN COALESCE(ua.qualified_at, ua.created_at) ELSE NULL END AS qualified_at")

	recordsQuery := strings.Join(strings.Fields(affiliateInviteRecordsSQL), " ")
	require.Contains(t, recordsQuery, "ua.qualifying_payment_amount::double precision")
	require.Contains(t, recordsQuery, "(ua.qualifying_payment_amount >= %[1]s) AS qualified")
	require.Contains(t, recordsQuery, "CASE WHEN ua.qualifying_payment_amount >= %[1]s THEN COALESCE(ua.qualified_at, ua.created_at) ELSE NULL END AS qualified_at")

	overviewQuery := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")
	require.Contains(t, overviewQuery, "qualifying_payment_amount >= $2")
}

func TestAffiliateBatchReconcileSQLKeepsQualifiedAndQualifiedAtConsistent(t *testing.T) {
	for _, query := range []string{affiliateInviterQualificationReconcileSQL, affiliateInviteesQualificationReconcileSQL} {
		normalized := strings.Join(strings.Fields(query), " ")
		require.Contains(t, normalized, "PARTITION BY user_id")
		require.Contains(t, normalized, "WHEN totals.amount >= $2 THEN COALESCE(totals.qualified_at, NOW())")
		require.Contains(t, normalized, "ELSE NULL")
		require.Contains(t, normalized, "RETURNING ua.user_id")
	}
}

func TestAffiliateInviteRecordsSQLProjectsInviterTierInputs(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateInviteRecordsSQL), " ")
	require.Contains(t, query, "page_inviter_ids AS")
	require.Contains(t, query, "JOIN page_inviter_ids")
	require.Contains(t, query, "record_count AS")
	require.Contains(t, query, "COUNT(*)::bigint AS total_count FROM matched_records")
	require.Contains(t, query, "LEFT JOIN page_records ON TRUE")
	require.Contains(t, query, "invited_count")
	require.Contains(t, query, "qualified_invitee_count")
	require.Contains(t, query, "aff_rebate_rate_percent")
}

func TestAffiliateInviteRecordsUseOneSnapshotForCountAndPage(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	method := string(source)
	start := strings.Index(method, "func (r *affiliateRepository) ListAffiliateInviteRecordsWithQualification")
	require.NotEqual(t, -1, start)
	end := strings.Index(method[start:], "func (r *affiliateRepository) ListAffiliateRebateRecords")
	require.NotEqual(t, -1, end)
	method = method[start : start+end]

	require.NotContains(t, method, "queryAffiliateRecordCount")
	require.Equal(t, 1, strings.Count(method, "QueryContext"))
}

func TestAffiliateInviteRecordsReturnsTotalForEmptyOutOfRangePage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client, db: db}

	mock.ExpectQuery("(?s)WITH matched_records AS MATERIALIZED.*record_count").
		WithArgs(service.AffiliateQualificationAmountDefault, 20, 1980).
		WillReturnRows(sqlmock.NewRows([]string{
			"has_record", "inviter_id", "inviter_email", "inviter_username",
			"invitee_id", "invitee_email", "invitee_username", "aff_code",
			"total_rebate", "qualifying_payment_amount", "qualified", "qualified_at",
			"invited_count", "qualified_invitee_count", "aff_rebate_rate_percent",
			"created_at", "total_count",
		}).AddRow(false, int64(0), "", "", int64(0), "", "", "", 0.0, 0.0, false, nil, 0, 0, nil, time.Now(), int64(7)))

	items, total, err := repo.ListAffiliateInviteRecordsWithQualification(context.Background(), service.AffiliateRecordFilter{
		Page:     100,
		PageSize: 20,
	}, service.AffiliateQualificationAmountDefault)

	require.NoError(t, err)
	require.Empty(t, items)
	require.Equal(t, int64(7), total)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateInviteRecordOrderingUsesUniqueTieBreaker(t *testing.T) {
	orderBy := buildAffiliateRecordOrderBy(service.AffiliateRecordFilter{SortBy: "created_at", SortDesc: true}, map[string]string{
		"created_at": "ua.created_at",
	}, "ua.created_at", "ua.user_id")

	require.Equal(t, "ORDER BY ua.created_at DESC NULLS LAST, ua.user_id DESC", orderBy)
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
	require.Contains(t, query, "WHEN totals.amount >= $2 THEN COALESCE(totals.qualified_at, NOW())")
	require.Contains(t, query, "ELSE NULL")
}

func TestAffiliateQualificationCountSQLUsesCurrentThreshold(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateQualificationCountSQL), " ")

	require.Contains(t, query, "inviter_id = $1")
	require.Contains(t, query, "qualifying_payment_amount >= $2")
	require.NotContains(t, query, "qualified_at IS NOT NULL")
	require.NotContains(t, query, "aff_count")
}

func TestAffiliateQualificationMarkReconcileRequiredIsAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client, db: db}

	mock.ExpectBegin()
	mock.ExpectExec("(?s)INSERT INTO settings .* DO NOTHING").
		WithArgs(service.SettingKeyAffiliateTierReconcileGeneration).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("(?s)INSERT INTO settings .* DO NOTHING").
		WithArgs(service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT key, value FROM settings WHERE key IN \\(\\$1, \\$2\\) ORDER BY key FOR UPDATE").
		WithArgs(service.SettingKeyAffiliateTierReconcileGeneration, service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).
			AddRow(service.SettingKeyAffiliateTierReconcileGeneration, "6").
			AddRow(service.SettingKeyAffiliateTierReconcileRequired, "true"))
	mock.ExpectExec("UPDATE settings SET value = \\$2, updated_at = NOW\\(\\) WHERE key = \\$1").
		WithArgs(service.SettingKeyAffiliateTierReconcileGeneration, "7").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE settings SET value = 'true', updated_at = NOW\\(\\) WHERE key = \\$1").
		WithArgs(service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	token, err := repo.MarkReconcileRequired(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(7), token.Generation)
	require.True(t, token.WasPendingBefore)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateQualificationClearReconcileRequiredUsesGenerationCAS(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client, db: db}

	mock.ExpectBegin()
	mock.ExpectExec("(?s)INSERT INTO settings .* DO NOTHING").
		WithArgs(service.SettingKeyAffiliateTierReconcileGeneration).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("(?s)INSERT INTO settings .* DO NOTHING").
		WithArgs(service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT key, value FROM settings WHERE key IN \\(\\$1, \\$2\\) ORDER BY key FOR UPDATE").
		WithArgs(service.SettingKeyAffiliateTierReconcileGeneration, service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).
			AddRow(service.SettingKeyAffiliateTierReconcileGeneration, "7").
			AddRow(service.SettingKeyAffiliateTierReconcileRequired, "true"))
	mock.ExpectExec("UPDATE settings SET value = 'false', updated_at = NOW\\(\\) WHERE key = \\$1").
		WithArgs(service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	cleared, err := repo.ClearReconcileRequiredIfGeneration(context.Background(), 7)

	require.NoError(t, err)
	require.True(t, cleared)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestAffiliateQualificationClearReconcileRequiredKeepsNewerGeneration(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })
	repo := &affiliateRepository{client: client, db: db}

	mock.ExpectBegin()
	mock.ExpectExec("(?s)INSERT INTO settings .* DO NOTHING").
		WithArgs(service.SettingKeyAffiliateTierReconcileGeneration).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec("(?s)INSERT INTO settings .* DO NOTHING").
		WithArgs(service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT key, value FROM settings WHERE key IN \\(\\$1, \\$2\\) ORDER BY key FOR UPDATE").
		WithArgs(service.SettingKeyAffiliateTierReconcileGeneration, service.SettingKeyAffiliateTierReconcileRequired).
		WillReturnRows(sqlmock.NewRows([]string{"key", "value"}).
			AddRow(service.SettingKeyAffiliateTierReconcileGeneration, "8").
			AddRow(service.SettingKeyAffiliateTierReconcileRequired, "true"))
	mock.ExpectCommit()

	cleared, err := repo.ClearReconcileRequiredIfGeneration(context.Background(), 7)

	require.NoError(t, err)
	require.False(t, cleared)
	require.NoError(t, mock.ExpectationsWereMet())
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
