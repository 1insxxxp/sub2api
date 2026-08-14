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

const checkinRewardCampaignPostgresImage = "postgres:18.1-alpine3.23"

func TestCheckinRewardCampaignMigrationPostgres(t *testing.T) {
	db := startCheckinRewardCampaignPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
CREATE TABLE user_checkins (
    id BIGSERIAL PRIMARY KEY
)`)
	require.NoError(t, err)

	raw, err := FS.ReadFile("222_checkin_reward_campaigns.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)

	for range 2 {
		_, err = db.ExecContext(ctx, migrationSQL)
		require.NoError(t, err)
	}

	insertCampaign := func(name, status, startDate, endDate string) error {
		_, insertErr := db.ExecContext(ctx, `
INSERT INTO checkin_reward_campaigns (name, status, start_date, end_date, reward_tiers)
VALUES ($1, $2, $3, $4, '[{"amount": 2, "probability": 100, "sort_order": 1}]'::jsonb)`,
			name, status, startDate, endDate)
		return insertErr
	}

	require.NoError(t, insertCampaign("enabled-one", "enabled", "2026-08-01", "2026-08-03"))
	require.NoError(t, insertCampaign("enabled-two", "enabled", "2026-08-04", "2026-08-06"))

	err = insertCampaign("enabled-overlap", "enabled", "2026-08-03", "2026-08-05")
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, pq.ErrorCode("23P01"), pqErr.Code)

	require.NoError(t, insertCampaign("draft-one", "draft", "2026-08-02", "2026-08-05"))
	require.NoError(t, insertCampaign("draft-two", "draft", "2026-08-02", "2026-08-05"))
}

func startCheckinRewardCampaignPostgres(t *testing.T) *sql.DB {
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
		checkinRewardCampaignPostgresImage,
		tcpostgres.WithDatabase("sub2api_checkin_campaign_migration_test"),
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
