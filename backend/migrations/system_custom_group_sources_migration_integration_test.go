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

const systemCustomGroupSourcesPostgresImage = "postgres:18.1-alpine3.23"

func TestSystemCustomGroupSourcesMigrationPostgres(t *testing.T) {
	db := startSystemCustomGroupSourcesPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
CREATE TABLE groups (
    id BIGINT PRIMARY KEY
);

CREATE TABLE system_custom_group_models (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL,
    source_group_id BIGINT NOT NULL
);

INSERT INTO groups (id) VALUES
    (1), (2), (3), (4), (5),
    (10), (11), (12), (13);

INSERT INTO system_custom_group_models (group_id, source_group_id) VALUES
    (1, 3),
    (1, 2),
    (1, 2),
    (1, 4);`)
	require.NoError(t, err)

	raw, err := FS.ReadFile("236_system_custom_group_sources.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)

	_, err = db.ExecContext(ctx, migrationSQL)
	require.NoError(t, err)

	requireSystemCustomGroupSources(t, ctx, db, 1, []systemCustomGroupSourceRow{
		{sourceGroupID: 3, priority: 0},
		{sourceGroupID: 2, priority: 1},
		{sourceGroupID: 4, priority: 2},
	})
	requireSystemCustomGroupRouteCount(t, ctx, db, 4)

	_, err = db.ExecContext(ctx, `
INSERT INTO system_custom_group_sources (group_id, source_group_id, priority)
VALUES (2, 2, 10)`)
	requireSystemCustomGroupSourcesPostgresCode(t, err, "23514")

	_, err = db.ExecContext(ctx, `
INSERT INTO system_custom_group_sources (group_id, source_group_id, priority)
VALUES (1, 5, -1)`)
	requireSystemCustomGroupSourcesPostgresCode(t, err, "23514")

	_, err = db.ExecContext(ctx, `
INSERT INTO system_custom_group_sources (group_id, source_group_id, priority)
VALUES (10, 11, 0)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DELETE FROM groups WHERE id = 10")
	require.NoError(t, err)
	requireSystemCustomGroupSources(t, ctx, db, 10, []systemCustomGroupSourceRow{})

	_, err = db.ExecContext(ctx, `
INSERT INTO system_custom_group_sources (group_id, source_group_id, priority)
VALUES (12, 13, 0)`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DELETE FROM groups WHERE id = 13")
	requireSystemCustomGroupSourcesPostgresCode(t, err, "23001")
	_, err = db.ExecContext(ctx, "DELETE FROM groups WHERE id = 12")
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, "DELETE FROM groups WHERE id = 13")
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, `
DELETE FROM system_custom_group_sources
WHERE group_id = 1 AND source_group_id = 2;

UPDATE system_custom_group_sources
SET priority = 9
WHERE group_id = 1 AND source_group_id = 3;

UPDATE system_custom_group_sources
SET priority = 0
WHERE group_id = 1 AND source_group_id = 4;

UPDATE system_custom_group_sources
SET priority = 2
WHERE group_id = 1 AND source_group_id = 3;

INSERT INTO system_custom_group_sources (group_id, source_group_id, priority)
VALUES (1, 5, 1);`)
	require.NoError(t, err)

	_, err = db.ExecContext(ctx, migrationSQL)
	require.NoError(t, err)
	requireSystemCustomGroupSources(t, ctx, db, 1, []systemCustomGroupSourceRow{
		{sourceGroupID: 4, priority: 0},
		{sourceGroupID: 5, priority: 1},
		{sourceGroupID: 3, priority: 2},
	})
	requireSystemCustomGroupRouteCount(t, ctx, db, 4)
}

type systemCustomGroupSourceRow struct {
	sourceGroupID int64
	priority      int
}

func requireSystemCustomGroupSources(
	t *testing.T,
	ctx context.Context,
	db *sql.DB,
	groupID int64,
	want []systemCustomGroupSourceRow,
) {
	t.Helper()

	rows, err := db.QueryContext(ctx, `
SELECT source_group_id, priority
FROM system_custom_group_sources
WHERE group_id = $1
ORDER BY priority`, groupID)
	require.NoError(t, err)
	defer rows.Close()

	got := make([]systemCustomGroupSourceRow, 0, len(want))
	for rows.Next() {
		var row systemCustomGroupSourceRow
		require.NoError(t, rows.Scan(&row.sourceGroupID, &row.priority))
		got = append(got, row)
	}
	require.NoError(t, rows.Err())
	require.Equal(t, want, got)
}

func requireSystemCustomGroupRouteCount(t *testing.T, ctx context.Context, db *sql.DB, want int) {
	t.Helper()

	var got int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM system_custom_group_models").Scan(&got))
	require.Equal(t, want, got)
}

func requireSystemCustomGroupSourcesPostgresCode(t *testing.T, err error, code pq.ErrorCode) {
	t.Helper()
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, code, pqErr.Code)
}

func startSystemCustomGroupSourcesPostgres(t *testing.T) *sql.DB {
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
		systemCustomGroupSourcesPostgresImage,
		tcpostgres.WithDatabase("sub2api_system_custom_group_sources_migration_test"),
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
