//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const migration175PostgresImage = "postgres:18.1-alpine3.23"

type affiliateTierMigrations struct {
	storageSQL  string
	backfillSQL string
	indexSQL    string
}

func TestMigration175Postgres(t *testing.T) {
	db := startMigration175Postgres(t)
	migrations := readAffiliateTierMigrations(t)

	t.Run("177 recovers an invalid same-name index", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		conn, schema := newMigration175Schema(t, ctx, db, "invalid_index")
		seedMigration175AffiliatesAndOrders(t, ctx, conn)
		execTransactionalMigration(t, ctx, conn, migrations.storageSQL)

		_, err := conn.ExecContext(ctx, `
CREATE UNIQUE INDEX CONCURRENTLY idx_user_affiliates_inviter_qualifying_amount
    ON user_affiliates (inviter_id, qualifying_payment_amount)`)
		require.Error(t, err, "duplicate inviter IDs must leave a failed concurrent index")

		valid, _ := affiliateQualifiedIndexState(t, ctx, conn, schema)
		require.False(t, valid, "failed concurrent build must leave an INVALID index")

		execNonTransactionalMigration(t, ctx, conn, migrations.indexSQL)

		valid, predicate := affiliateQualifiedIndexState(t, ctx, conn, schema)
		require.True(t, valid, "177 must replace the INVALID index with a valid index")
		require.Empty(t, predicate)
	})

	t.Run("ordered migrations build schema backfill and replay", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		conn, schema := newMigration175Schema(t, ctx, db, "backfill")
		seedMigration175AffiliatesAndOrders(t, ctx, conn)
		applyAffiliateTierMigrations(t, ctx, conn, migrations)

		assertMigration175Schema(t, ctx, conn, schema)
		assertMigration175Qualification(t, ctx, conn, 2, "50.00000000", timestamp(2))
		assertMigration175Qualification(t, ctx, conn, 3, "50.00000000", timestamp(3))
		assertMigration175Unqualified(t, ctx, conn, 4)
		assertMigration175Unqualified(t, ctx, conn, 5)
		assertMigration175Unqualified(t, ctx, conn, 6)
		assertMigration175Unqualified(t, ctx, conn, 8)
		assertMigration175Qualification(t, ctx, conn, 9, "51.00000000", timestamp(9))
		assertMigration175Qualification(t, ctx, conn, 10, "52.00000000", timestamp(10))
		assertMigration175Qualification(t, ctx, conn, 11, "53.00000000", timestamp(11))
		assertMigration175Qualification(t, ctx, conn, 12, "54.00000000", timestamp(12))
		assertMigration175Unqualified(t, ctx, conn, 13)
		assertReconcileRequired(t, ctx, conn)

		// A relationship without an inviter is outside the backfill. Give it a
		// sentinel value after the first run so replay proves it remains untouched.
		unboundQualifiedAt := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)
		_, err := conn.ExecContext(ctx, `
UPDATE user_affiliates
SET qualifying_payment_amount = 777, qualified_at = $1
WHERE user_id = 7`, unboundQualifiedAt)
		require.NoError(t, err)
		_, err = conn.ExecContext(ctx, `
UPDATE settings SET value = 'false'
WHERE key = 'affiliate_tier_reconcile_required'`)
		require.NoError(t, err)

		applyAffiliateTierMigrations(t, ctx, conn, migrations)
		assertMigration175Qualification(t, ctx, conn, 2, "50.00000000", timestamp(2))
		assertMigration175Qualification(t, ctx, conn, 3, "50.00000000", timestamp(3))
		assertMigration175Qualification(t, ctx, conn, 7, "777.00000000", unboundQualifiedAt)
		assertReconcileRequired(t, ctx, conn)
	})

	settingCases := []struct {
		name     string
		initial  *string
		expected string
	}{
		{name: "missing inserts eight", expected: "8"},
		{name: "numeric ten updates to eight", initial: stringPointer("10.0"), expected: "8"},
		{name: "administrator value is preserved", initial: stringPointer("12.5"), expected: "12.5"},
	}
	for _, tc := range settingCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			conn, _ := newMigration175Schema(t, ctx, db, strings.ReplaceAll(tc.name, " ", "_"))
			if tc.initial != nil {
				_, err := conn.ExecContext(ctx, `
INSERT INTO settings (key, value) VALUES ('affiliate_rebate_rate', $1)`, *tc.initial)
				require.NoError(t, err)
			}

			applyAffiliateTierMigrations(t, ctx, conn, migrations)
			applyAffiliateTierMigrations(t, ctx, conn, migrations)

			var value string
			require.NoError(t, conn.QueryRowContext(ctx, `
SELECT value FROM settings WHERE key = 'affiliate_rebate_rate'`).Scan(&value))
			require.Equal(t, tc.expected, value)
			assertReconcileRequired(t, ctx, conn)
		})
	}
}

func readAffiliateTierMigrations(t *testing.T) affiliateTierMigrations {
	t.Helper()
	read := func(name string) string {
		content, err := FS.ReadFile(name)
		require.NoError(t, err)
		return string(content)
	}
	return affiliateTierMigrations{
		storageSQL:  read("175_add_affiliate_promotion_tiers.sql"),
		backfillSQL: read("176_backfill_affiliate_promotion_tiers.sql"),
		indexSQL:    read("177_add_affiliate_qualified_lookup_index_notx.sql"),
	}
}

func applyAffiliateTierMigrations(
	t *testing.T,
	ctx context.Context,
	conn *sql.Conn,
	migrations affiliateTierMigrations,
) {
	t.Helper()
	execTransactionalMigration(t, ctx, conn, migrations.storageSQL)
	execTransactionalMigration(t, ctx, conn, migrations.backfillSQL)

	// The runner executes *_notx.sql directly because PostgreSQL prohibits
	// CREATE INDEX CONCURRENTLY inside a transaction block.
	execNonTransactionalMigration(t, ctx, conn, migrations.indexSQL)
}

func execTransactionalMigration(t *testing.T, ctx context.Context, conn *sql.Conn, migrationSQL string) {
	t.Helper()
	tx, err := conn.BeginTx(ctx, nil)
	require.NoError(t, err)
	if _, err = tx.ExecContext(ctx, migrationSQL); err != nil {
		_ = tx.Rollback()
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
}

func execNonTransactionalMigration(t *testing.T, ctx context.Context, conn *sql.Conn, migrationSQL string) {
	t.Helper()
	for _, statement := range strings.Split(migrationSQL, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		_, err := conn.ExecContext(ctx, statement)
		require.NoError(t, err)
	}
}

func startMigration175Postgres(t *testing.T) *sql.DB {
	t.Helper()
	dockerCtx, cancelDocker := context.WithTimeout(context.Background(), 5*time.Second)
	dockerErr := exec.CommandContext(dockerCtx, "docker", "info").Run()
	cancelDocker()
	if dockerErr != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Docker is required for migration integration tests in CI: %v", dockerErr)
		}
		t.Skip("Docker is unavailable; skipping PostgreSQL migration integration test")
	}

	startCtx, cancelStart := context.WithTimeout(context.Background(), 60*time.Second)
	container, err := tcpostgres.Run(
		startCtx,
		migration175PostgresImage,
		tcpostgres.WithDatabase("sub2api_migration_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	cancelStart()
	require.NoError(t, err)
	t.Cleanup(func() {
		terminateCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		_ = container.Terminate(terminateCtx)
	})

	dsnCtx, cancelDSN := context.WithTimeout(context.Background(), 5*time.Second)
	dsn, err := container.ConnectionString(dsnCtx, "sslmode=disable", "TimeZone=UTC")
	cancelDSN()
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelPing()
	require.NoError(t, db.PingContext(pingCtx))
	return db
}

func newMigration175Schema(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	suffix string,
) (*sql.Conn, string) {
	t.Helper()

	schema := "migration_175_" + suffix
	conn, err := db.Conn(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _ = db.ExecContext(dropCtx, "DROP SCHEMA IF EXISTS "+schema+" CASCADE")
	})
	t.Cleanup(func() { _ = conn.Close() })

	_, err = conn.ExecContext(ctx, fmt.Sprintf(`
CREATE SCHEMA %s;
SET search_path TO %s;

CREATE TABLE users (
    id BIGINT PRIMARY KEY
);

CREATE TABLE user_affiliates (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    aff_code VARCHAR(32) NOT NULL UNIQUE,
    inviter_id BIGINT NULL REFERENCES users(id) ON DELETE SET NULL,
    aff_count INTEGER NOT NULL DEFAULT 0,
    aff_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    aff_history_quota DECIMAL(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE payment_orders (
    id BIGINT PRIMARY KEY,
    user_id BIGINT NOT NULL,
    amount DECIMAL(20,2) NOT NULL,
    order_type VARCHAR(20) NOT NULL,
    status VARCHAR(30) NOT NULL,
    refund_amount DECIMAL(20,2) NOT NULL DEFAULT 0,
    paid_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE settings (
    id BIGSERIAL PRIMARY KEY,
    key VARCHAR(100) NOT NULL UNIQUE,
    value TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);`, schema, schema))
	require.NoError(t, err)
	return conn, schema
}

func seedMigration175AffiliatesAndOrders(t *testing.T, ctx context.Context, conn *sql.Conn) {
	t.Helper()

	_, err := conn.ExecContext(ctx, `
INSERT INTO users (id)
SELECT generate_series(1, 13);

INSERT INTO user_affiliates (user_id, aff_code, inviter_id) VALUES
    (1,  'AFF1',  NULL),
    (2,  'AFF2',  1),
    (3,  'AFF3',  1),
    (4,  'AFF4',  1),
    (5,  'AFF5',  1),
    (6,  'AFF6',  1),
    (7,  'AFF7',  NULL),
    (8,  'AFF8',  1),
    (9,  'AFF9',  1),
    (10, 'AFF10', 1),
    (11, 'AFF11', 1),
    (12, 'AFF12', 1),
    (13, 'AFF13', 1);

INSERT INTO payment_orders
    (id, user_id, amount, order_type, status, refund_amount, completed_at, created_at)
VALUES
    (1,  2,  20,  'balance',      'COMPLETED',          0,  '2026-01-01 12:00:00+00', '2026-01-01 12:00:00+00'),
    (2,  2,  30,  'subscription', 'COMPLETED',          0,  '2026-01-02 12:00:00+00', '2026-01-02 12:00:00+00'),
    (3,  2,  500, 'voucher',      'COMPLETED',          0,  '2026-01-02 13:00:00+00', '2026-01-02 13:00:00+00'),
    (4,  3,  80,  'balance',      'PARTIALLY_REFUNDED', 30, '2026-01-03 12:00:00+00', '2026-01-03 12:00:00+00'),
    (5,  4,  100, 'subscription', 'REFUNDED',           100,'2026-01-04 12:00:00+00', '2026-01-04 12:00:00+00'),
    (6,  5,  100, 'balance',      'PENDING',            0,  '2026-01-05 12:00:00+00', '2026-01-05 12:00:00+00'),
    (7,  6,  100, 'gift',         'COMPLETED',          0,  '2026-01-06 12:00:00+00', '2026-01-06 12:00:00+00'),
    (8,  7,  100, 'balance',      'COMPLETED',          0,  '2026-01-07 12:00:00+00', '2026-01-07 12:00:00+00'),
    (9,  8,  40,  'balance',      'PARTIALLY_REFUNDED', 60, '2026-01-08 12:00:00+00', '2026-01-08 12:00:00+00'),
    (10, 9,  51,  'balance',      'REFUND_REQUESTED',   11, '2026-01-09 12:00:00+00', '2026-01-09 12:00:00+00'),
    (11, 10, 52,  'subscription', 'REFUNDING',          12, '2026-01-10 12:00:00+00', '2026-01-10 12:00:00+00'),
    (12, 11, 53,  'balance',      'REFUND_PENDING',     13, '2026-01-11 12:00:00+00', '2026-01-11 12:00:00+00'),
    (13, 12, 54,  'subscription', 'REFUND_FAILED',      14, '2026-01-12 12:00:00+00', '2026-01-12 12:00:00+00'),
    (14, 13, 100, 'balance',      'FAILED',             0,  '2026-01-13 12:00:00+00', '2026-01-13 12:00:00+00');`)
	require.NoError(t, err)
}

func assertMigration175Schema(t *testing.T, ctx context.Context, conn *sql.Conn, schema string) {
	t.Helper()

	var dataType, nullable, columnDefault string
	var precision, scale int
	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT data_type, is_nullable, numeric_precision, numeric_scale, column_default
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = 'user_affiliates'
  AND column_name = 'qualifying_payment_amount'`, schema).
		Scan(&dataType, &nullable, &precision, &scale, &columnDefault))
	require.Equal(t, "numeric", dataType)
	require.Equal(t, "NO", nullable)
	require.Equal(t, 20, precision)
	require.Equal(t, 8, scale)
	require.Contains(t, columnDefault, "0")

	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT data_type, is_nullable
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = 'user_affiliates'
  AND column_name = 'qualified_at'`, schema).Scan(&dataType, &nullable))
	require.Equal(t, "timestamp with time zone", dataType)
	require.Equal(t, "YES", nullable)

	var indexDefinition string
	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT indexdef
FROM pg_indexes
WHERE schemaname = $1 AND tablename = 'user_affiliates'
  AND indexname = 'idx_user_affiliates_inviter_qualifying_amount'`, schema).Scan(&indexDefinition))
	require.Contains(t, indexDefinition, "(inviter_id, qualifying_payment_amount)")
	require.NotContains(t, indexDefinition, "WHERE")
}

func affiliateQualifiedIndexState(
	t *testing.T,
	ctx context.Context,
	conn *sql.Conn,
	schema string,
) (bool, string) {
	t.Helper()

	var valid bool
	var predicate sql.NullString
	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT i.indisvalid, pg_get_expr(i.indpred, i.indrelid)
FROM pg_index i
JOIN pg_class idx ON idx.oid = i.indexrelid
JOIN pg_namespace ns ON ns.oid = idx.relnamespace
WHERE ns.nspname = $1
  AND idx.relname = 'idx_user_affiliates_inviter_qualifying_amount'`, schema).Scan(&valid, &predicate))
	return valid, predicate.String
}

func assertMigration175Qualification(
	t *testing.T,
	ctx context.Context,
	conn *sql.Conn,
	userID int64,
	expectedAmount string,
	expectedQualifiedAt time.Time,
) {
	t.Helper()

	var amount string
	var qualifiedAt sql.NullTime
	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT qualifying_payment_amount::text, qualified_at
FROM user_affiliates WHERE user_id = $1`, userID).Scan(&amount, &qualifiedAt))
	require.Equal(t, expectedAmount, amount)
	require.True(t, qualifiedAt.Valid)
	require.True(t, expectedQualifiedAt.Equal(qualifiedAt.Time),
		"qualified_at: expected %s, got %s", expectedQualifiedAt, qualifiedAt.Time)
}

func assertMigration175Unqualified(t *testing.T, ctx context.Context, conn *sql.Conn, userID int64) {
	t.Helper()

	var amount string
	var qualifiedAt sql.NullTime
	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT qualifying_payment_amount::text, qualified_at
FROM user_affiliates WHERE user_id = $1`, userID).Scan(&amount, &qualifiedAt))
	require.Equal(t, "0.00000000", amount)
	require.False(t, qualifiedAt.Valid)
}

func assertReconcileRequired(t *testing.T, ctx context.Context, conn *sql.Conn) {
	t.Helper()
	var value string
	require.NoError(t, conn.QueryRowContext(ctx, `
SELECT value FROM settings WHERE key = 'affiliate_tier_reconcile_required'`).Scan(&value))
	require.Equal(t, "true", value)
}

func timestamp(day int) time.Time {
	return time.Date(2026, 1, day, 12, 0, 0, 0, time.UTC)
}

func stringPointer(value string) *string {
	return &value
}
