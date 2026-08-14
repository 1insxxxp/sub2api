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

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

const checkinRewardCampaignPostgresImage = "postgres:18.1-alpine3.23"

func TestCheckinRewardCampaignMigrationPostgres(t *testing.T) {
	db := startCheckinRewardCampaignPostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
CREATE TABLE users (
    id BIGINT PRIMARY KEY
);

CREATE TABLE user_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date VARCHAR(10) NOT NULL,
    reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    balance_before DECIMAL(20,8) NOT NULL DEFAULT 0,
    balance_after DECIMAL(20,8) NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    streak_day INTEGER NOT NULL DEFAULT 1,
    base_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    bonus_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    total_reward_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    previous_day_usage_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    usage_rebate_amount DECIMAL(20,8) NOT NULL DEFAULT 0,
    reward_cap_adjustment DECIMAL(20,8) NOT NULL DEFAULT 0,
    UNIQUE (user_id, checkin_date)
);

INSERT INTO users (id) VALUES (1);
INSERT INTO user_checkins (user_id, checkin_date)
VALUES (1, '2026-07-31');`)
	require.NoError(t, err)

	raw, err := FS.ReadFile("222_checkin_reward_campaigns.sql")
	require.NoError(t, err)
	migrationSQL := string(raw)

	for range 2 {
		_, err = db.ExecContext(ctx, migrationSQL)
		require.NoError(t, err)
	}

	var legacyCampaignID sql.NullInt64
	var legacyCampaignName string
	var legacyTiers []byte
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT reward_campaign_id, reward_campaign_name, reward_campaign_tiers_snapshot
FROM user_checkins
WHERE user_id = 1 AND checkin_date = '2026-07-31'`).Scan(
		&legacyCampaignID,
		&legacyCampaignName,
		&legacyTiers,
	))
	require.False(t, legacyCampaignID.Valid)
	require.Empty(t, legacyCampaignName)
	require.JSONEq(t, "[]", string(legacyTiers))

	var campaignIndexDefinition string
	require.NoError(t, db.QueryRowContext(ctx, `
SELECT indexdef
FROM pg_indexes
WHERE schemaname = current_schema()
  AND tablename = 'user_checkins'
  AND indexname = 'user_checkins_reward_campaign_id_idx'`).Scan(&campaignIndexDefinition))
	require.Contains(t, campaignIndexDefinition, "(reward_campaign_id)")

	driver := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(driver))
	t.Cleanup(func() { _ = client.Close() })

	tiers := []domain.CheckinRewardTier{
		{Amount: 2.5, Probability: 75, SortOrder: 1},
		{Amount: 5, Probability: 25, SortOrder: 2},
	}
	entCampaign, err := client.CheckinRewardCampaign.Create().
		SetName("ent-round-trip").
		SetStatus(domain.CheckinRewardCampaignStatusDraft).
		SetStartDate(time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)).
		SetEndDate(time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)).
		SetRewardTiers(tiers).
		Save(ctx)
	require.NoError(t, err)

	loadedCampaign, err := client.CheckinRewardCampaign.Get(ctx, entCampaign.ID)
	require.NoError(t, err)
	require.Equal(t, tiers, loadedCampaign.RewardTiers)

	entCheckin, err := client.UserCheckin.Create().
		SetUserID(1).
		SetCheckinDate("2026-09-01").
		SetRewardCampaignID(entCampaign.ID).
		SetRewardCampaignName(entCampaign.Name).
		SetRewardCampaignTiersSnapshot(tiers).
		Save(ctx)
	require.NoError(t, err)

	loadedCheckin, err := client.UserCheckin.Get(ctx, entCheckin.ID)
	require.NoError(t, err)
	require.NotNil(t, loadedCheckin.RewardCampaignID)
	require.Equal(t, entCampaign.ID, *loadedCheckin.RewardCampaignID)
	require.Equal(t, entCampaign.Name, loadedCheckin.RewardCampaignName)
	require.Equal(t, tiers, loadedCheckin.RewardCampaignTiersSnapshot)

	err = client.CheckinRewardCampaign.DeleteOneID(entCampaign.ID).Exec(ctx)
	requirePostgresErrorCode(t, err, "23503")

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
	requirePostgresErrorCode(t, err, "23P01")

	require.NoError(t, insertCampaign("draft-one", "draft", "2026-08-02", "2026-08-05"))
	require.NoError(t, insertCampaign("draft-two", "draft", "2026-08-02", "2026-08-05"))

	_, err = db.ExecContext(ctx, `
UPDATE checkin_reward_campaigns
SET status = 'enabled'
WHERE name = 'draft-one'`)
	requirePostgresErrorCode(t, err, "23P01")
}

func requirePostgresErrorCode(t *testing.T, err error, code pq.ErrorCode) {
	t.Helper()
	require.Error(t, err)
	var pqErr *pq.Error
	require.True(t, errors.As(err, &pqErr))
	require.Equal(t, code, pqErr.Code)
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
