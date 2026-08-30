//go:build integration

package migrations

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const migration232PostgresImage = "postgres:18.1-alpine3.23"

func TestMigration232Postgres(t *testing.T) {
	db := startMigration232Postgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    balance NUMERIC(20,8) NOT NULL DEFAULT 0
);

CREATE TABLE redeem_codes (
    id BIGINT PRIMARY KEY
);

CREATE TABLE usage_logs (
    id BIGINT PRIMARY KEY
);

INSERT INTO users (id) VALUES (1);
INSERT INTO redeem_codes (id) VALUES (1);
INSERT INTO usage_logs (id) VALUES (1);`)
	require.NoError(t, err)

	content, err := FS.ReadFile("232_gift_redeem_balance_eligibility.sql")
	require.NoError(t, err)
	migrationSQL := string(content)

	for range 2 {
		_, err = db.ExecContext(ctx, migrationSQL)
		require.NoError(t, err)
	}

	requireMigration232Column(t, ctx, db, "users", "gift_balance", "numeric", 20, 8, "0")
	requireMigration232Column(t, ctx, db, "users", "frozen_gift_balance", "numeric", 20, 8, "0")
	requireMigration232Column(t, ctx, db, "redeem_codes", "threshold_exempt", "boolean", 0, 0, "false")
	requireMigration232Column(t, ctx, db, "usage_logs", "threshold_exempt_cost", "numeric", 20, 10, "0")

	var giftBalance, frozenGiftBalance, thresholdExemptCost string
	var thresholdExempt bool
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT gift_balance::text, frozen_gift_balance::text
FROM users
WHERE id = 1`).Scan(&giftBalance, &frozenGiftBalance))
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT threshold_exempt
FROM redeem_codes
WHERE id = 1`).Scan(&thresholdExempt))
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT threshold_exempt_cost::text
FROM usage_logs
WHERE id = 1`).Scan(&thresholdExemptCost))
	require.Equal(t, "0.00000000", giftBalance)
	require.Equal(t, "0.00000000", frozenGiftBalance)
	require.False(t, thresholdExempt)
	require.Equal(t, "0.0000000000", thresholdExemptCost)

	constraints := []struct {
		table string
		name  string
	}{
		{"users", "users_gift_balance_nonnegative"},
		{"users", "users_frozen_gift_balance_nonnegative"},
		{"users", "users_gift_balance_within_balance"},
		{"usage_logs", "usage_logs_threshold_exempt_cost_nonnegative"},
	}
	for _, constraint := range constraints {
		var validated bool
		require.NoError(t, db.QueryRowContext(ctx, `
SELECT convalidated
FROM pg_constraint
WHERE conname = $1
  AND conrelid = to_regclass($2)`, constraint.name, constraint.table).Scan(&validated))
		require.False(t, validated, "%s must remain NOT VALID during installation", constraint.name)
	}

	negativeUpdates := []string{
		"UPDATE users SET gift_balance = -0.00000001 WHERE id = 1",
		"UPDATE users SET frozen_gift_balance = -0.00000001 WHERE id = 1",
		"UPDATE usage_logs SET threshold_exempt_cost = -0.0000000001 WHERE id = 1",
	}
	for _, query := range negativeUpdates {
		_, err = db.ExecContext(ctx, query)
		requireMigration232CheckViolation(t, err)
	}
	_, err = db.ExecContext(ctx, "UPDATE users SET balance = 1, gift_balance = 2 WHERE id = 1")
	requireMigration232CheckViolation(t, err)
}

func requireMigration232Column(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	table string,
	column string,
	dataType string,
	precision int64,
	scale int64,
	defaultValue string,
) {
	t.Helper()

	var actualDataType, nullable, actualDefault string
	var actualPrecision, actualScale sql.NullInt64
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT data_type, numeric_precision, numeric_scale, is_nullable, column_default
FROM information_schema.columns
WHERE table_schema = current_schema()
  AND table_name = $1
  AND column_name = $2`, table, column).Scan(
		&actualDataType,
		&actualPrecision,
		&actualScale,
		&nullable,
		&actualDefault,
	))
	require.Equal(t, dataType, actualDataType)
	require.Equal(t, "NO", nullable)
	require.Equal(t, defaultValue, actualDefault)
	if precision == 0 {
		require.False(t, actualPrecision.Valid)
		require.False(t, actualScale.Valid)
		return
	}
	require.Equal(t, precision, actualPrecision.Int64)
	require.Equal(t, scale, actualScale.Int64)
}

func requireMigration232CheckViolation(t *testing.T, err error) {
	t.Helper()
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, pq.ErrorCode("23514"), pqErr.Code)
}

func startMigration232Postgres(t *testing.T) *sql.DB {
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
		migration232PostgresImage,
		tcpostgres.WithDatabase("sub2api_gift_balance_migration_test"),
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
