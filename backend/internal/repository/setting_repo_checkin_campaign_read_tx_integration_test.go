//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

type blockingCheckinAwardRepository struct {
	service.SettingRepository
	reader service.CheckinCampaignConfigReadTransactionRepository
	writer service.CheckinCampaignConfigTransactionRepository

	mu           sync.Mutex
	block        bool
	readEntered  chan struct{}
	writeEntered chan struct{}
	releaseRead  chan struct{}
	readOnce     sync.Once
	writeOnce    sync.Once
}

type duplicateCheckinBarrier struct {
	mu      sync.Mutex
	arrived int
	ready   chan struct{}
}

func newDuplicateCheckinBarrier() *duplicateCheckinBarrier {
	return &duplicateCheckinBarrier{ready: make(chan struct{})}
}

func (b *duplicateCheckinBarrier) wait(ctx context.Context) error {
	b.mu.Lock()
	b.arrived++
	if b.arrived == 2 {
		close(b.ready)
	}
	b.mu.Unlock()
	select {
	case <-b.ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type duplicateCheckinReadRepository struct {
	service.SettingRepository
	reader         service.CheckinCampaignConfigReadTransactionRepository
	barrier        *duplicateCheckinBarrier
	callbackErrors chan error
}

func (r *duplicateCheckinReadRepository) WithCheckinCampaignConfigReadTx(
	ctx context.Context,
	fn func(*dbent.Client, service.SettingRepository) error,
) error {
	return r.reader.WithCheckinCampaignConfigReadTx(ctx, func(client *dbent.Client, repo service.SettingRepository) error {
		var firstCheckinQuery sync.Once
		waitedForDuplicate := false
		client.UserCheckin.Intercept(dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
			return dbent.QuerierFunc(func(queryCtx context.Context, query dbent.Query) (dbent.Value, error) {
				value, err := next.Query(queryCtx, query)
				if err == nil {
					firstCheckinQuery.Do(func() {
						waitedForDuplicate = true
						err = r.barrier.wait(queryCtx)
					})
				}
				return value, err
			})
		}))
		err := fn(client, repo)
		if waitedForDuplicate {
			r.callbackErrors <- err
		}
		return err
	})
}

func (r *blockingCheckinAwardRepository) WithCheckinCampaignConfigReadTx(
	ctx context.Context,
	fn func(*dbent.Client, service.SettingRepository) error,
) error {
	return r.reader.WithCheckinCampaignConfigReadTx(ctx, func(client *dbent.Client, repo service.SettingRepository) error {
		r.mu.Lock()
		block := r.block
		readEntered := r.readEntered
		releaseRead := r.releaseRead
		r.mu.Unlock()
		if block {
			r.readOnce.Do(func() { close(readEntered) })
			select {
			case <-releaseRead:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return fn(client, repo)
	})
}

func (r *blockingCheckinAwardRepository) WithCheckinCampaignConfigTx(
	ctx context.Context,
	fn func(*dbent.Client, service.SettingRepository) error,
) error {
	return r.writer.WithCheckinCampaignConfigTx(ctx, func(client *dbent.Client, repo service.SettingRepository) error {
		r.mu.Lock()
		block := r.block
		writeEntered := r.writeEntered
		r.mu.Unlock()
		if block {
			r.writeOnce.Do(func() { close(writeEntered) })
		}
		return fn(client, repo)
	})
}

func (r *blockingCheckinAwardRepository) startBlocking() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.block = true
	r.readEntered = make(chan struct{})
	r.writeEntered = make(chan struct{})
	r.releaseRead = make(chan struct{})
	r.readOnce = sync.Once{}
	r.writeOnce = sync.Once{}
}

func (r *blockingCheckinAwardRepository) stopBlocking() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.block = false
}

func TestCheckinPostgresSharedReadProducesCompleteOldOrNewPolicy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	baseRepo := NewSettingRepository(integrationEntClient)
	reader := baseRepo.(service.CheckinCampaignConfigReadTransactionRepository)
	writer := baseRepo.(service.CheckinCampaignConfigTransactionRepository)
	blockingRepo := &blockingCheckinAwardRepository{
		SettingRepository: baseRepo,
		reader:            reader,
		writer:            writer,
	}
	svc := service.NewCheckinService(integrationEntClient, nil, nil)
	svc.SetSettingRepository(blockingRepo)
	beijing := time.FixedZone("CST", 8*60*60)
	checkinDate := time.Now().In(beijing).Format("2006-01-02")

	oldSettings, err := baseRepo.GetMultiple(ctx, []string{
		service.SettingKeyCheckinEnabled,
		service.SettingKeyCheckinMinTotalUsageUSD,
		service.SettingKeyCheckinMinTotalRechargeUSD,
		service.SettingKeyCheckinRewardConfig,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, key := range []string{
			service.SettingKeyCheckinEnabled,
			service.SettingKeyCheckinMinTotalUsageUSD,
			service.SettingKeyCheckinMinTotalRechargeUSD,
			service.SettingKeyCheckinRewardConfig,
		} {
			if value, ok := oldSettings[key]; ok {
				_ = baseRepo.Set(cleanupCtx, key, value)
			} else {
				_ = baseRepo.Delete(cleanupCtx, key)
			}
		}
	})

	_, err = svc.UpdateConfig(ctx, service.CheckinConfig{
		Enabled:       true,
		Tiers:         []service.CheckinRewardTier{{Amount: 1, Probability: 100}},
		StreakEnabled: true,
		StreakRules:   []service.CheckinStreakRule{{Day: 1, BonusAmount: 1}},
	})
	require.NoError(t, err)
	campaign, err := svc.CreateRewardCampaign(ctx, service.CreateCheckinRewardCampaignInput{
		Name:        "postgres shared award snapshot",
		StartDate:   checkinDate,
		EndDate:     checkinDate,
		RewardTiers: []service.CheckinRewardTier{{Amount: 5, Probability: 100}},
		AdminID:     9001,
	})
	require.NoError(t, err)
	_, err = svc.EnableRewardCampaign(ctx, campaign.ID, 9001)
	require.NoError(t, err)

	first := createPostgresCheckinIntegrationUser(t, ctx, "old")
	second := createPostgresCheckinIntegrationUser(t, ctx, "new")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM redeem_codes WHERE used_by = ANY($1)", pq.Array([]int64{first.ID, second.ID}))
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM user_checkins WHERE user_id = ANY($1)", pq.Array([]int64{first.ID, second.ID}))
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM users WHERE id = ANY($1)", pq.Array([]int64{first.ID, second.ID}))
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM checkin_reward_campaigns WHERE id = $1", campaign.ID)
	})
	blockingRepo.startBlocking()
	checkinResult := make(chan *service.CheckinResult, 1)
	checkinErr := make(chan error, 1)
	go func() {
		result, checkinError := svc.Checkin(ctx, first.ID)
		checkinResult <- result
		checkinErr <- checkinError
	}()
	select {
	case <-blockingRepo.readEntered:
	case <-ctx.Done():
		t.Fatal("check-in did not acquire PostgreSQL shared campaign/config lock")
	}

	updateResult := make(chan error, 1)
	go func() {
		_, updateErr := svc.UpdateConfig(ctx, service.CheckinConfig{
			Enabled:       true,
			Tiers:         []service.CheckinRewardTier{{Amount: 1, Probability: 100}},
			StreakEnabled: true,
			StreakRules:   []service.CheckinStreakRule{{Day: 1, BonusAmount: 3}},
		})
		updateResult <- updateErr
	}()
	select {
	case <-blockingRepo.writeEntered:
		t.Fatal("exclusive config writer entered while check-in held a shared snapshot lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(blockingRepo.releaseRead)
	require.NoError(t, <-checkinErr)
	oldResult := <-checkinResult
	require.NotNil(t, oldResult)
	require.Equal(t, 5.0, oldResult.BaseRewardAmount)
	require.Equal(t, 1.0, oldResult.BonusRewardAmount)
	require.Equal(t, 6.0, oldResult.TotalRewardAmount)
	require.NoError(t, <-updateResult)
	blockingRepo.stopBlocking()

	newResult, err := svc.Checkin(ctx, second.ID)
	require.NoError(t, err)
	require.Equal(t, 5.0, newResult.BaseRewardAmount)
	require.Equal(t, 3.0, newResult.BonusRewardAmount)
	require.Equal(t, 8.0, newResult.TotalRewardAmount)

	_, err = svc.DisableRewardCampaign(ctx, campaign.ID, 9001)
	require.NoError(t, err)
	nextCampaign, err := svc.CreateRewardCampaign(ctx, service.CreateCheckinRewardCampaignInput{
		Name:        "postgres shared campaign enable",
		StartDate:   checkinDate,
		EndDate:     checkinDate,
		RewardTiers: []service.CheckinRewardTier{{Amount: 5, Probability: 100}},
		AdminID:     9001,
	})
	require.NoError(t, err)
	third := createPostgresCheckinIntegrationUser(t, ctx, "before-enable")
	fourth := createPostgresCheckinIntegrationUser(t, ctx, "after-enable")
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		ids := []int64{third.ID, fourth.ID}
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM redeem_codes WHERE used_by = ANY($1)", pq.Array(ids))
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM user_checkins WHERE user_id = ANY($1)", pq.Array(ids))
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM users WHERE id = ANY($1)", pq.Array(ids))
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM checkin_reward_campaigns WHERE id = $1", nextCampaign.ID)
	})

	blockingRepo.startBlocking()
	beforeEnableResult := make(chan *service.CheckinResult, 1)
	beforeEnableErr := make(chan error, 1)
	go func() {
		result, checkinError := svc.Checkin(ctx, third.ID)
		beforeEnableResult <- result
		beforeEnableErr <- checkinError
	}()
	select {
	case <-blockingRepo.readEntered:
	case <-ctx.Done():
		t.Fatal("check-in did not acquire shared lock before campaign enable")
	}
	enableResult := make(chan error, 1)
	go func() {
		_, enableErr := svc.EnableRewardCampaign(ctx, nextCampaign.ID, 9001)
		enableResult <- enableErr
	}()
	select {
	case <-blockingRepo.writeEntered:
		t.Fatal("campaign enable entered while check-in held a shared snapshot lock")
	case <-time.After(100 * time.Millisecond):
	}
	close(blockingRepo.releaseRead)
	require.NoError(t, <-beforeEnableErr)
	oldCampaignResult := <-beforeEnableResult
	require.Equal(t, 1.0, oldCampaignResult.BaseRewardAmount)
	require.Equal(t, 3.0, oldCampaignResult.BonusRewardAmount)
	require.Equal(t, 4.0, oldCampaignResult.TotalRewardAmount)
	require.NoError(t, <-enableResult)
	blockingRepo.stopBlocking()

	afterEnableResult, err := svc.Checkin(ctx, fourth.ID)
	require.NoError(t, err)
	require.Equal(t, 5.0, afterEnableResult.BaseRewardAmount)
	require.Equal(t, 3.0, afterEnableResult.BonusRewardAmount)
	require.Equal(t, 8.0, afterEnableResult.TotalRewardAmount)
}

func TestSettingRepositoryPostgresSharedCheckinReadersDoNotSerialize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	repo := NewSettingRepository(integrationEntClient).(service.CheckinCampaignConfigReadTransactionRepository)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)

	for index := 0; index < 2; index++ {
		go func() {
			results <- repo.WithCheckinCampaignConfigReadTx(ctx, func(*dbent.Client, service.SettingRepository) error {
				entered <- struct{}{}
				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			})
		}()
	}
	for index := 0; index < 2; index++ {
		select {
		case <-entered:
		case <-time.After(time.Second):
			t.Fatal("PostgreSQL shared check-in readers serialized instead of entering together")
		}
	}
	close(release)
	require.NoError(t, <-results)
	require.NoError(t, <-results)
}

func TestCheckinPostgresConcurrentSameUserDateAwardsExactlyOnce(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	baseRepo := NewSettingRepository(integrationEntClient)
	settingsKeys := []string{
		service.SettingKeyCheckinEnabled,
		service.SettingKeyCheckinMinTotalUsageUSD,
		service.SettingKeyCheckinMinTotalRechargeUSD,
		service.SettingKeyCheckinRewardConfig,
	}
	oldSettings, err := baseRepo.GetMultiple(ctx, settingsKeys)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		for _, key := range settingsKeys {
			if value, ok := oldSettings[key]; ok {
				_ = baseRepo.Set(cleanupCtx, key, value)
			} else {
				_ = baseRepo.Delete(cleanupCtx, key)
			}
		}
	})

	svc := service.NewCheckinService(integrationEntClient, nil, nil)
	svc.SetSettingRepository(baseRepo)
	_, err = svc.UpdateConfig(ctx, service.CheckinConfig{
		Enabled: true,
		Tiers:   []service.CheckinRewardTier{{Amount: 1, Probability: 100}},
	})
	require.NoError(t, err)
	user := createPostgresCheckinIntegrationUser(t, ctx, "same-user-date")
	_, err = integrationEntClient.User.UpdateOneID(user.ID).SetBalance(10).Save(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM redeem_codes WHERE used_by = $1", user.ID)
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM user_checkins WHERE user_id = $1", user.ID)
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM users WHERE id = $1", user.ID)
	})

	barrier := newDuplicateCheckinBarrier()
	callbackErrors := make(chan error, 2)
	svc.SetSettingRepository(&duplicateCheckinReadRepository{
		SettingRepository: baseRepo,
		reader:            baseRepo.(service.CheckinCampaignConfigReadTransactionRepository),
		barrier:           barrier,
		callbackErrors:    callbackErrors,
	})
	type checkinOutcome struct {
		result *service.CheckinResult
		err    error
	}
	start := make(chan struct{})
	outcomes := make(chan checkinOutcome, 2)
	for index := 0; index < 2; index++ {
		go func() {
			<-start
			result, checkinErr := svc.Checkin(ctx, user.ID)
			outcomes <- checkinOutcome{result: result, err: checkinErr}
		}()
	}
	close(start)

	alreadyCheckedIn := 0
	newAwards := 0
	for index := 0; index < 2; index++ {
		outcome := <-outcomes
		require.NoError(t, outcome.err)
		require.NotNil(t, outcome.result)
		if outcome.result.AlreadyCheckedIn {
			alreadyCheckedIn++
		} else {
			newAwards++
		}
	}
	require.Equal(t, 1, newAwards)
	require.Equal(t, 1, alreadyCheckedIn)

	callbackErrorCount := 0
	for index := 0; index < 2; index++ {
		callbackErr := <-callbackErrors
		if callbackErr != nil {
			callbackErrorCount++
			require.Equal(t, "check-in already recorded", callbackErr.Error())
		}
	}
	require.Equal(t, 1, callbackErrorCount, "loser must roll back through the private duplicate sentinel")

	storedUser, err := integrationEntClient.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 11.0, storedUser.Balance)
	var checkinCount, historyCount int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM user_checkins WHERE user_id = $1", user.ID).Scan(&checkinCount))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM redeem_codes WHERE used_by = $1 AND type = $2", user.ID, service.AdjustmentTypeCheckinReward).Scan(&historyCount))
	require.Equal(t, 1, checkinCount)
	require.Equal(t, 1, historyCount)

	var (
		indexIsUnique   bool
		indexDefinition string
	)
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT indexes.indisunique, pg_get_indexdef(indexes.indexrelid)
		FROM pg_index AS indexes
		JOIN pg_class AS index_relation ON index_relation.oid = indexes.indexrelid
		WHERE indexes.indrelid = 'user_checkins'::regclass
		  AND index_relation.relname = 'user_checkins_user_id_date_uq'
	`).Scan(&indexIsUnique, &indexDefinition))
	require.True(t, indexIsUnique)
	require.Contains(t, indexDefinition, "(user_id, checkin_date)")
}

func createPostgresCheckinIntegrationUser(t *testing.T, ctx context.Context, suffix string) *dbent.User {
	t.Helper()
	email := fmt.Sprintf("checkin-shared-%s-%d@example.com", suffix, time.Now().UnixNano())
	user, err := integrationEntClient.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		SetBalance(0).
		Save(ctx)
	require.NoError(t, err)
	return user
}
