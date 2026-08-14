//go:build integration

package service

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/checkinrewardcampaign"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/lib/pq"
)

const checkinRewardCampaignServicePostgresImage = "postgres:18.1-alpine3.23"

type postgresCheckinCampaignTestRepository struct {
	client       *dbent.Client
	serialize    bool
	sawExclusion atomic.Bool
}

func (r *postgresCheckinCampaignTestRepository) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *postgresCheckinCampaignTestRepository) GetValue(context.Context, string) (string, error) {
	return "", ErrSettingNotFound
}
func (r *postgresCheckinCampaignTestRepository) Set(context.Context, string, string) error {
	return nil
}
func (r *postgresCheckinCampaignTestRepository) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *postgresCheckinCampaignTestRepository) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (r *postgresCheckinCampaignTestRepository) GetAll(context.Context) (map[string]string, error) {
	return map[string]string{}, nil
}
func (r *postgresCheckinCampaignTestRepository) Delete(context.Context, string) error { return nil }

func (r *postgresCheckinCampaignTestRepository) WithCheckinCampaignConfigTx(
	ctx context.Context,
	fn func(client *dbent.Client, repo SettingRepository) error,
) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if r.serialize {
		var rows entsql.Rows
		if err := tx.Client().Driver().Query(ctx, "SELECT pg_advisory_xact_lock($1)", []any{int64(0x43484b43414d50)}, &rows); err != nil {
			return err
		}
		_ = rows.Close()
	}
	if err := fn(tx.Client(), r); err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) && pqErr != nil && pqErr.Code == "23P01" {
			r.sawExclusion.Store(true)
		}
		return err
	}
	return tx.Commit()
}

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

func TestCheckinRewardCampaignEnableConcurrentPostgresOverlap(t *testing.T) {
	db := startCheckinRewardCampaignServicePostgres(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, `
CREATE EXTENSION IF NOT EXISTS btree_gist;
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
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT checkin_reward_campaigns_enabled_dates_excl
        EXCLUDE USING gist (daterange(start_date, end_date, '[]') WITH &&)
        WHERE (status = 'enabled')
);
CREATE FUNCTION slow_checkin_campaign_enable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status <> 'enabled' AND NEW.status = 'enabled' THEN
        PERFORM pg_sleep(0.25);
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER slow_checkin_campaign_enable_trigger
    BEFORE UPDATE OF status ON checkin_reward_campaigns
    FOR EACH ROW EXECUTE FUNCTION slow_checkin_campaign_enable();`)
	require.NoError(t, err)

	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	for _, tt := range []struct {
		name             string
		serialize        bool
		startDate        time.Time
		wantSQLExclusion bool
	}{
		{name: "shared advisory protocol", serialize: true, startDate: time.Date(2199, 8, 10, 0, 0, 0, 0, time.UTC)},
		{name: "database exclusion fallback", serialize: false, startDate: time.Date(2199, 9, 10, 0, 0, 0, 0, time.UTC), wantSQLExclusion: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			endDate := tt.startDate.AddDate(0, 0, 2)
			drafts := make([]*dbent.CheckinRewardCampaign, 0, 2)
			for index := 0; index < 2; index++ {
				draft, createErr := client.CheckinRewardCampaign.Create().
					SetName(tt.name + "-" + strconv.Itoa(index+1)).
					SetStatus(domain.CheckinRewardCampaignStatusDraft).
					SetStartDate(tt.startDate).
					SetEndDate(endDate).
					SetRewardTiers([]domain.CheckinRewardTier{{Amount: 2, Probability: 100}}).
					Save(ctx)
				require.NoError(t, createErr)
				drafts = append(drafts, draft)
			}

			repo := &postgresCheckinCampaignTestRepository{client: client, serialize: tt.serialize}
			svc := NewCheckinService(client, nil, nil)
			svc.SetSettingRepository(repo)
			start := make(chan struct{})
			type enableResult struct {
				campaign *CheckinRewardCampaign
				err      error
			}
			results := make(chan enableResult, 2)
			var wg sync.WaitGroup
			for _, draft := range drafts {
				wg.Add(1)
				go func(id int64) {
					defer wg.Done()
					<-start
					enabled, enableErr := svc.EnableRewardCampaign(ctx, id, 77)
					results <- enableResult{campaign: enabled, err: enableErr}
				}(draft.ID)
			}
			close(start)
			wg.Wait()
			close(results)

			successCount := 0
			overlapCount := 0
			for result := range results {
				if result.err == nil {
					successCount++
					require.NotNil(t, result.campaign)
					continue
				}
				overlapCount++
				require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_OVERLAP", infraerrors.Reason(result.err))
				metadata := infraerrors.FromError(result.err).Metadata
				require.NotEmpty(t, metadata["conflict_campaign_id"])
				require.NotEmpty(t, metadata["conflict_campaign_name"])
				require.NotEmpty(t, metadata["conflict_start_date"])
				require.NotEmpty(t, metadata["conflict_end_date"])
			}
			require.Equal(t, 1, successCount)
			require.Equal(t, 1, overlapCount)
			enabledCount, countErr := client.CheckinRewardCampaign.Query().Where(
				checkinrewardcampaign.IDIn(drafts[0].ID, drafts[1].ID),
				checkinrewardcampaign.StatusEQ(domain.CheckinRewardCampaignStatusEnabled),
			).Count(ctx)
			require.NoError(t, countErr)
			require.Equal(t, 1, enabledCount)
			require.Equal(t, tt.wantSQLExclusion, repo.sawExclusion.Load())
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
