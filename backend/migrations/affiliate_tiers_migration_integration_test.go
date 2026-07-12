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

func TestMigration175Postgres(t *testing.T) {
	ctx := context.Background()
	db := startMigration175Postgres(t, ctx)

	content, err := FS.ReadFile("175_add_affiliate_promotion_tiers.sql")
	require.NoError(t, err)
	migrationSQL := string(content)

	t.Run("schema backfill and replay", func(t *testing.T) {
		tx, schema := newMigration175Schema(t, ctx, db, "backfill")
		seedMigration175AffiliatesAndOrders(t, ctx, tx)

		_, err := tx.ExecContext(ctx, migrationSQL)
		require.NoError(t, err)

		assertMigration175Schema(t, ctx, tx, schema)
		assertMigration175Qualification(t, ctx, tx, 2, "50.00000000", time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC))
		assertMigration175Qualification(t, ctx, tx, 3, "50.00000000", time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC))
		assertMigration175Unqualified(t, ctx, tx, 4)
		assertMigration175Unqualified(t, ctx, tx, 5)
		assertMigration175Unqualified(t, ctx, tx, 6)
		assertMigration175Unqualified(t, ctx, tx, 8)

		// A relationship without an inviter is outside the backfill. Give it a
		// sentinel value after the first run so replay proves it remains untouched.
		unboundQualifiedAt := time.Date(2025, 12, 31, 12, 0, 0, 0, time.UTC)
		_, err = tx.ExecContext(ctx, `
UPDATE user_affiliates
SET qualifying_payment_amount = 777, qualified_at = $1
WHERE user_id = 7`, unboundQualifiedAt)
		require.NoError(t, err)

		_, err = tx.ExecContext(ctx, migrationSQL)
		require.NoError(t, err, "migration must be replayable")

		assertMigration175Qualification(t, ctx, tx, 2, "50.00000000", time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC))
		assertMigration175Qualification(t, ctx, tx, 3, "50.00000000", time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC))
		assertMigration175Qualification(t, ctx, tx, 7, "777.00000000", unboundQualifiedAt)
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
			tx, _ := newMigration175Schema(t, ctx, db, strings.ReplaceAll(tc.name, " ", "_"))
			if tc.initial != nil {
				_, err := tx.ExecContext(ctx, `
INSERT INTO settings (key, value) VALUES ('affiliate_rebate_rate', $1)`, *tc.initial)
				require.NoError(t, err)
			}

			_, err := tx.ExecContext(ctx, migrationSQL)
			require.NoError(t, err)
			_, err = tx.ExecContext(ctx, migrationSQL)
			require.NoError(t, err, "settings migration must be replayable")

			var value string
			require.NoError(t, tx.QueryRowContext(ctx, `
SELECT value FROM settings WHERE key = 'affiliate_rebate_rate'`).Scan(&value))
			require.Equal(t, tc.expected, value)
		})
	}
}

func startMigration175Postgres(t *testing.T, ctx context.Context) *sql.DB {
	t.Helper()
	if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Docker is required for migration integration tests in CI: %v", err)
		}
		t.Skip("Docker is unavailable; skipping PostgreSQL migration integration test")
	}

	container, err := tcpostgres.Run(
		ctx,
		migration175PostgresImage,
		tcpostgres.WithDatabase("sub2api_migration_test"),
		tcpostgres.WithUsername("postgres"),
		tcpostgres.WithPassword("postgres"),
		tcpostgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable", "TimeZone=UTC")
	require.NoError(t, err)
	db, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	require.NoError(t, db.PingContext(ctx))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func newMigration175Schema(t *testing.T, ctx context.Context, db *sql.DB, suffix string) (*sql.Tx, string) {
	t.Helper()

	schema := "migration_175_" + suffix
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })

	_, err = tx.ExecContext(ctx, fmt.Sprintf(`
CREATE SCHEMA %s;
SET LOCAL search_path TO %s;

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
	return tx, schema
}

func seedMigration175AffiliatesAndOrders(t *testing.T, ctx context.Context, tx *sql.Tx) {
	t.Helper()

	_, err := tx.ExecContext(ctx, `
INSERT INTO users (id) VALUES (1), (2), (3), (4), (5), (6), (7), (8);
INSERT INTO user_affiliates (user_id, aff_code, inviter_id) VALUES
    (1, 'AFF1', NULL),
    (2, 'AFF2', 1),
    (3, 'AFF3', 1),
    (4, 'AFF4', 1),
    (5, 'AFF5', 1),
    (6, 'AFF6', 1),
    (7, 'AFF7', NULL),
    (8, 'AFF8', 1);

INSERT INTO payment_orders
    (id, user_id, amount, order_type, status, refund_amount, completed_at, created_at)
VALUES
    (1, 2, 20,  'balance',      'COMPLETED',          0,  '2026-01-01 12:00:00+00', '2026-01-01 12:00:00+00'),
    (2, 2, 30,  'subscription', 'COMPLETED',          0,  '2026-01-02 12:00:00+00', '2026-01-02 12:00:00+00'),
    (3, 2, 500, 'voucher',      'COMPLETED',          0,  '2026-01-02 13:00:00+00', '2026-01-02 13:00:00+00'),
    (4, 3, 80,  'balance',      'PARTIALLY_REFUNDED', 30, '2026-01-03 12:00:00+00', '2026-01-03 12:00:00+00'),
    (5, 4, 100, 'subscription', 'REFUNDED',           100,'2026-01-04 12:00:00+00', '2026-01-04 12:00:00+00'),
    (6, 5, 100, 'balance',      'PENDING',            0,  '2026-01-05 12:00:00+00', '2026-01-05 12:00:00+00'),
    (7, 6, 100, 'gift',         'COMPLETED',          0,  '2026-01-06 12:00:00+00', '2026-01-06 12:00:00+00'),
    (8, 7, 100, 'balance',      'COMPLETED',          0,  '2026-01-07 12:00:00+00', '2026-01-07 12:00:00+00'),
    (9, 8, 40,  'balance',      'PARTIALLY_REFUNDED', 60, '2026-01-08 12:00:00+00', '2026-01-08 12:00:00+00');`)
	require.NoError(t, err)
}

func assertMigration175Schema(t *testing.T, ctx context.Context, tx *sql.Tx, schema string) {
	t.Helper()

	var dataType, nullable, columnDefault string
	var precision, scale int
	require.NoError(t, tx.QueryRowContext(ctx, `
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

	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT data_type, is_nullable
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = 'user_affiliates'
  AND column_name = 'qualified_at'`, schema).Scan(&dataType, &nullable))
	require.Equal(t, "timestamp with time zone", dataType)
	require.Equal(t, "YES", nullable)

	var indexDefinition string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT indexdef
FROM pg_indexes
WHERE schemaname = $1 AND tablename = 'user_affiliates'
  AND indexname = 'idx_user_affiliates_inviter_qualified'`, schema).Scan(&indexDefinition))
	require.Contains(t, indexDefinition, "(inviter_id)")
	require.Contains(t, indexDefinition, "qualified_at IS NOT NULL")
}

func assertMigration175Qualification(
	t *testing.T,
	ctx context.Context,
	tx *sql.Tx,
	userID int64,
	expectedAmount string,
	expectedQualifiedAt time.Time,
) {
	t.Helper()

	var amount string
	var qualifiedAt sql.NullTime
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT qualifying_payment_amount::text, qualified_at
FROM user_affiliates WHERE user_id = $1`, userID).Scan(&amount, &qualifiedAt))
	require.Equal(t, expectedAmount, amount)
	require.True(t, qualifiedAt.Valid)
	require.True(t, expectedQualifiedAt.Equal(qualifiedAt.Time),
		"qualified_at: expected %s, got %s", expectedQualifiedAt, qualifiedAt.Time)
}

func assertMigration175Unqualified(t *testing.T, ctx context.Context, tx *sql.Tx, userID int64) {
	t.Helper()

	var amount string
	var qualifiedAt sql.NullTime
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT qualifying_payment_amount::text, qualified_at
FROM user_affiliates WHERE user_id = $1`, userID).Scan(&amount, &qualifiedAt))
	require.Equal(t, "0.00000000", amount)
	require.False(t, qualifiedAt.Valid)
}

func stringPointer(value string) *string {
	return &value
}
