//go:build integration

package service

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
)

const checkinRewardCampaignServicePostgresImage = "postgres:18.1-alpine3.23"

func TestResolveEffectiveCheckinConfigPostgresUTCSessionDateBoundaries(t *testing.T) {
	db := startCheckinRewardCampaignServicePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var sessionTimezone string
	require.NoError(t, db.QueryRowContext(ctx, "SHOW TimeZone").Scan(&sessionTimezone))
	require.Equal(t, "UTC", sessionTimezone)

	_, err := db.ExecContext(ctx, `
CREATE TABLE checkin_reward_campaigns (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    status VARCHAR(20) NOT NULL,
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    reward_tiers JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_by BIGINT NULL,
    updated_by BIGINT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)`)
	require.NoError(t, err)

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	svc := NewCheckinService(client, nil, nil)
	_, err = client.CheckinRewardCampaign.Create().
		SetName("UTC 会话边界").
		SetStatus(domain.CheckinRewardCampaignStatusEnabled).
		SetStartDate(time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)).
		SetEndDate(time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC)).
		SetRewardTiers([]domain.CheckinRewardTier{{Amount: 2, Probability: 100}}).
		Save(ctx)
	require.NoError(t, err)

	baseline := &CheckinConfig{
		Enabled: true,
		Tiers:   []CheckinRewardTier{{Amount: 1, Probability: 100}},
	}
	for _, date := range []string{"2026-08-09", "2026-08-10", "2026-08-12", "2026-08-13"} {
		t.Run(date, func(t *testing.T) {
			effective, resolveErr := svc.resolveEffectiveCheckinConfig(ctx, client, date, baseline)
			require.NoError(t, resolveErr)
			if date == "2026-08-10" || date == "2026-08-12" {
				require.NotNil(t, effective.Campaign)
				require.Equal(t, "2026-08-10", effective.Campaign.StartDate)
				require.Equal(t, "2026-08-12", effective.Campaign.EndDate)
				require.Equal(t, CheckinRewardCampaignLifecycleActive, effective.Campaign.LifecycleStatus)
				require.Equal(t, 2.0, effective.Config.Tiers[0].Amount)
				return
			}
			require.Nil(t, effective.Campaign)
			require.Equal(t, 1.0, effective.Config.Tiers[0].Amount)
		})
	}
}

func startCheckinRewardCampaignServicePostgres(t *testing.T) *sql.DB {
	t.Helper()
	dockerCtx, cancelDocker := context.WithTimeout(context.Background(), 5*time.Second)
	dockerErr := exec.CommandContext(dockerCtx, "docker", "info").Run()
	cancelDocker()
	if dockerErr != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("Docker is required for check-in campaign integration tests in CI: %v", dockerErr)
		}
		t.Skip("Docker is unavailable; skipping PostgreSQL check-in campaign integration test")
	}

	startCtx, cancelStart := context.WithTimeout(context.Background(), 60*time.Second)
	container, err := tcpostgres.Run(
		startCtx,
		checkinRewardCampaignServicePostgresImage,
		tcpostgres.WithDatabase("sub2api_checkin_campaign_service_test"),
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
