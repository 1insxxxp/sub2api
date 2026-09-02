//go:build unit

package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"

	coreent "entgo.io/ent"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newCheckinServiceTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:checkin_service?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

type checkinSettingRepoStub struct {
	values map[string]string
}

type sequencedCheckinSettingRepo struct {
	*checkinSettingRepoStub
	mu        sync.Mutex
	snapshots []map[string]string
	reads     int
}

type barrierCheckinSettingRepo struct {
	*checkinSettingRepoStub
	mu               sync.Mutex
	firstReadStarted chan struct{}
	releaseFirstRead chan struct{}
	firstRead        bool
}

func newBarrierCheckinSettingRepo(values map[string]string) *barrierCheckinSettingRepo {
	if _, ok := values[SettingKeyCheckinMinDailyUsageCount]; !ok {
		values[SettingKeyCheckinMinDailyUsageCount] = "0"
	}
	return &barrierCheckinSettingRepo{
		checkinSettingRepoStub: &checkinSettingRepoStub{values: values},
		firstReadStarted:       make(chan struct{}),
		releaseFirstRead:       make(chan struct{}),
	}
}

func (r *barrierCheckinSettingRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	firstRead := !r.firstRead
	if firstRead {
		r.firstRead = true
		close(r.firstReadStarted)
	}
	r.mu.Unlock()
	if firstRead {
		select {
		case <-r.releaseFirstRead:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return out, nil
}

func (r *barrierCheckinSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *sequencedCheckinSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	index := r.reads
	if index >= len(r.snapshots) {
		index = len(r.snapshots) - 1
	}
	r.reads++
	out := make(map[string]string, len(keys))
	if index < 0 {
		return out, nil
	}
	for _, key := range keys {
		if value, ok := r.snapshots[index][key]; ok {
			out[key] = value
		} else if key == SettingKeyCheckinMinDailyUsageCount {
			out[key] = "0"
		}
	}
	return out, nil
}

func newCheckinSettingRepoStub() *checkinSettingRepoStub {
	return &checkinSettingRepoStub{values: map[string]string{SettingKeyCheckinMinDailyUsageCount: "0"}}
}

func (s *checkinSettingRepoStub) Get(_ context.Context, key string) (*Setting, error) {
	value, ok := s.values[key]
	if !ok {
		return nil, ErrSettingNotFound
	}
	return &Setting{Key: key, Value: value}, nil
}

func (s *checkinSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	setting, err := s.Get(ctx, key)
	if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (s *checkinSettingRepoStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *checkinSettingRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (s *checkinSettingRepoStub) SetMultiple(_ context.Context, settings map[string]string) error {
	for key, value := range settings {
		s.values[key] = value
	}
	return nil
}

func (s *checkinSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

func (s *checkinSettingRepoStub) Delete(_ context.Context, key string) error {
	delete(s.values, key)
	return nil
}

func setCheckinConfig(t *testing.T, repo *checkinSettingRepoStub, cfg CheckinConfig) {
	t.Helper()
	repo.values[SettingKeyCheckinEnabled] = strconv.FormatBool(cfg.Enabled)
	repo.values[SettingKeyCheckinMinTotalUsageUSD] = strconv.FormatFloat(cfg.MinTotalUsageUSD, 'f', -1, 64)
	repo.values[SettingKeyCheckinMinTotalRechargeUSD] = strconv.FormatFloat(cfg.MinTotalRechargeUSD, 'f', -1, 64)
	repo.values[SettingKeyCheckinMinDailyUsageCount] = strconv.Itoa(cfg.MinDailyUsageCount)
}

func createCheckinTestUser(t *testing.T, ctx context.Context, client *dbent.Client, email string, balance float64) *dbent.User {
	t.Helper()
	createdUser, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetStatus(StatusActive).
		SetBalance(balance).
		Save(ctx)
	require.NoError(t, err)
	return createdUser
}

func createCheckinUsage(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, totalCost, actualCost float64) {
	t.Helper()
	suffix := strconv.FormatInt(userID, 10)
	group, err := client.Group.Create().
		SetName("checkin-test-group-" + suffix).
		Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetUserID(userID).
		SetKey("sk-checkin-test-" + suffix).
		SetName("test key").
		SetGroupID(group.ID).
		Save(ctx)
	require.NoError(t, err)
	account, err := client.Account.Create().
		SetName("checkin-test-account-" + suffix).
		SetPlatform("anthropic").
		SetType("api_key").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetUserID(userID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID("req-checkin-test-" + suffix).
		SetModel("claude-test").
		SetTotalCost(totalCost).
		SetActualCost(actualCost).
		Save(ctx)
	require.NoError(t, err)
}

func createCheckinUsageAt(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, requestID string, actualCost float64, createdAt time.Time) {
	t.Helper()
	group, err := client.Group.Create().SetName("checkin-usage-" + requestID).Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetUserID(userID).
		SetKey("sk-checkin-" + requestID).
		SetName("check-in usage").
		SetGroupID(group.ID).
		Save(ctx)
	require.NoError(t, err)
	account, err := client.Account.Create().
		SetName("checkin-account-" + requestID).
		SetPlatform("anthropic").
		SetType("api_key").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetUserID(userID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID(requestID).
		SetModel("claude-test").
		SetTotalCost(actualCost).
		SetActualCost(actualCost).
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)
}

func createCheckinUsageWithThresholdExempt(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, requestID string, actualCost, thresholdExemptCost float64, createdAt *time.Time) {
	t.Helper()
	group, err := client.Group.Create().SetName("checkin-gift-" + requestID).Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetUserID(userID).
		SetKey("sk-checkin-gift-" + requestID).
		SetName("check-in gift usage").
		SetGroupID(group.ID).
		Save(ctx)
	require.NoError(t, err)
	account, err := client.Account.Create().
		SetName("checkin-gift-account-" + requestID).
		SetPlatform("anthropic").
		SetType("api_key").
		Save(ctx)
	require.NoError(t, err)
	create := client.UsageLog.Create().
		SetUserID(userID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID(requestID).
		SetModel("claude-test").
		SetTotalCost(actualCost).
		SetActualCost(actualCost).
		SetThresholdExemptCost(thresholdExemptCost)
	if createdAt != nil {
		create.SetCreatedAt(*createdAt)
	}
	_, err = create.Save(ctx)
	require.NoError(t, err)
}

func configureCheckinCampaignAwardBaseline(t *testing.T, ctx context.Context, svc *CheckinService) {
	t.Helper()
	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled:             true,
		MinTotalUsageUSD:    5,
		MinTotalRechargeUSD: 20,
		Tiers:               []CheckinRewardTier{{Amount: 1, Probability: 100}},
		StreakEnabled:       true,
		StreakRules: []CheckinStreakRule{
			{Day: 7, LotteryAttempts: 4},
			{Day: 14, LotteryAttempts: 8},
		},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         8,
		TotalRewardCap:         10,
	})
	require.NoError(t, err)
}

func createPreviousCheckinDays(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, firstDay time.Time, count int) {
	t.Helper()
	for index := 0; index < count; index++ {
		day := firstDay.AddDate(0, 0, index)
		_, err := client.UserCheckin.Create().
			SetUserID(userID).
			SetCheckinDate(day.Format("2006-01-02")).
			SetStreakDay(index + 1).
			SetBaseRewardAmount(1).
			SetTotalRewardAmount(1).
			SetRewardAmount(1).
			SetBalanceBefore(float64(index)).
			SetBalanceAfter(float64(index + 1)).
			SetCreatedAt(day).
			Save(ctx)
		require.NoError(t, err)
	}
}

func TestCheckinStatusShowsActiveRewardCampaign(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "campaign-status@example.com", 10)
	repo := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	configureCheckinCampaignAwardBaseline(t, ctx, svc)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	campaign := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "暑期签到加码", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-17", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)
	createCheckinUsageAt(t, ctx, client, user.ID, "campaign-status-usage", 50, time.Date(2026, 8, 14, 12, 0, 0, 0, beijing))

	status, err := svc.GetStatus(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, status.CheckedIn)
	require.NotNil(t, status.RewardCampaignID)
	require.Equal(t, campaign.ID, *status.RewardCampaignID)
	require.Equal(t, "暑期签到加码", status.RewardCampaignName)
	require.Equal(t, 4.0, status.EstimatedUsageRebate, "campaigns must not change the baseline usage rebate")
	require.Equal(t, 5.0, status.MinTotalUsageUSD, "campaigns must not change baseline eligibility")
}

func TestCheckinStatusUsesAtomicBaselineCampaignSnapshot(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "atomic-status-snapshot@example.com", 10)
	repo := newBarrierCheckinSettingRepo(map[string]string{
		SettingKeyCheckinEnabled: "true",
		SettingKeyCheckinRewardConfig: `{
			"tiers":[{"amount":1,"probability":100}],
			"usage_rebate_enabled":true,
			"usage_rebate_rate_percent":10,
			"usage_rebate_cap":1,
			"total_reward_cap":2
		}`,
	})
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	draft := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "原子状态快照", domain.CheckinRewardCampaignStatusDraft,
		"2026-08-15", "2026-08-15", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)

	statusResult := make(chan *CheckinStatus, 1)
	statusErr := make(chan error, 1)
	go func() {
		status, err := svc.GetStatus(ctx, user.ID)
		statusResult <- status
		statusErr <- err
	}()
	<-repo.firstReadStarted

	writerDone := make(chan error, 1)
	go func() {
		_, err := svc.UpdateConfig(ctx, CheckinConfig{
			Enabled:                true,
			Tiers:                  []CheckinRewardTier{{Amount: 1, Probability: 100}},
			UsageRebateEnabled:     true,
			UsageRebateRatePercent: 10,
			UsageRebateCap:         1,
			TotalRewardCap:         5,
		})
		if err == nil {
			_, err = svc.EnableRewardCampaign(ctx, draft.ID, 99)
		}
		writerDone <- err
	}()
	var writerErr error
	select {
	case writerErr = <-writerDone:
		close(repo.releaseFirstRead)
	case <-time.After(50 * time.Millisecond):
		close(repo.releaseFirstRead)
		writerErr = <-writerDone
	}
	require.NoError(t, writerErr)

	require.NoError(t, <-statusErr)
	status := <-statusResult
	require.NotNil(t, status)
	require.Nil(t, status.RewardCampaignID, "status must expose a complete old or new snapshot, never old baseline plus new campaign")
}

func TestCheckinPreflightUsesAtomicBaselineCampaignSnapshot(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "atomic-preflight-snapshot@example.com", 10)
	repo := newBarrierCheckinSettingRepo(map[string]string{
		SettingKeyCheckinEnabled: "true",
		SettingKeyCheckinRewardConfig: `{
			"tiers":[{"amount":1,"probability":100}],
			"usage_rebate_enabled":true,
			"usage_rebate_rate_percent":10,
			"usage_rebate_cap":1,
			"total_reward_cap":2
		}`,
	})
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	svc.rewardRoll = func() float64 { return 0 }
	draft := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "原子预检快照", domain.CheckinRewardCampaignStatusDraft,
		"2026-08-15", "2026-08-15", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)

	checkinResult := make(chan *CheckinResult, 1)
	checkinErr := make(chan error, 1)
	go func() {
		result, err := svc.Checkin(ctx, user.ID)
		checkinResult <- result
		checkinErr <- err
	}()
	<-repo.firstReadStarted

	writerDone := make(chan error, 1)
	go func() {
		_, err := svc.UpdateConfig(ctx, CheckinConfig{
			Enabled:                true,
			Tiers:                  []CheckinRewardTier{{Amount: 1, Probability: 100}},
			UsageRebateEnabled:     true,
			UsageRebateRatePercent: 10,
			UsageRebateCap:         1,
			TotalRewardCap:         5,
		})
		if err == nil {
			_, err = svc.EnableRewardCampaign(ctx, draft.ID, 99)
		}
		writerDone <- err
	}()
	var writerErr error
	select {
	case writerErr = <-writerDone:
		close(repo.releaseFirstRead)
	case <-time.After(50 * time.Millisecond):
		close(repo.releaseFirstRead)
		writerErr = <-writerDone
	}
	require.NoError(t, writerErr)

	require.NoError(t, <-checkinErr)
	result := <-checkinResult
	require.NotNil(t, result)
	switch result.BaseRewardAmount {
	case 1:
		require.Nil(t, result.RewardCampaignID)
	case 5:
		require.NotNil(t, result.RewardCampaignID)
	default:
		t.Fatalf("check-in used a mixed preflight snapshot: base reward = %v", result.BaseRewardAmount)
	}
}

func TestCheckinAwardsCampaignBaseButPreservesUsageAndStreakRewards(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "campaign-award@example.com", 10)
	repo := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	configureCheckinCampaignAwardBaseline(t, ctx, svc)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	svc.rewardRoll = func() float64 { return 0 }
	createPreviousCheckinDays(t, ctx, client, user.ID, time.Date(2026, 8, 9, 9, 0, 0, 0, beijing), 6)
	createCheckinUsageAt(t, ctx, client, user.ID, "campaign-award-usage", 50, time.Date(2026, 8, 14, 12, 0, 0, 0, beijing))
	campaign := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "七夕基础奖励", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-15", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)

	result, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 7, result.StreakDay)
	require.Equal(t, 5.0, result.BaseRewardAmount)
	require.Equal(t, 50.0, result.PreviousDayUsageAmount)
	require.Equal(t, 4.0, result.UsageRebateAmount)
	require.Zero(t, result.BonusRewardAmount)
	require.Equal(t, 4, result.LotteryAttemptsReward)
	require.Zero(t, result.RewardCapAdjustment)
	require.Equal(t, 9.0, result.TotalRewardAmount)
	require.Equal(t, 19.0, result.BalanceAfter)
	require.Equal(t, 4, client.LotteryAttemptWallet.Query().OnlyX(ctx).Balance)
	require.NotNil(t, result.RewardCampaignID)
	require.Equal(t, campaign.ID, *result.RewardCampaignID)
	require.Equal(t, "七夕基础奖励", result.RewardCampaignName)
	require.NotNil(t, result.NextStreakRule)
}

func TestCheckinAuthorityReadsBaselineAndCampaignFromOneSharedSnapshot(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "campaign-shared-snapshot@example.com", 10)
	oldSnapshot := map[string]string{
		SettingKeyCheckinEnabled:      "true",
		SettingKeyCheckinRewardConfig: `{"tiers":[{"amount":1,"probability":100,"sort_order":1}]}`,
	}
	newSnapshot := map[string]string{
		SettingKeyCheckinEnabled: "true",
		SettingKeyCheckinRewardConfig: `{
			"tiers":[{"amount":1,"probability":100,"sort_order":1}],
			"usage_rebate_enabled":true,
			"usage_rebate_rate_percent":10,
			"usage_rebate_cap":5,
			"total_reward_cap":10
		}`,
	}
	repo := &sequencedCheckinSettingRepo{
		checkinSettingRepoStub: newCheckinSettingRepoStub(),
		snapshots:              []map[string]string{oldSnapshot, newSnapshot},
	}
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	svc.rewardRoll = func() float64 { return 0 }
	createCheckinUsageAt(t, ctx, client, user.ID, "campaign-shared-snapshot-usage", 50, time.Date(2026, 8, 14, 12, 0, 0, 0, beijing))
	createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "共享快照", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-15", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)

	result, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 5.0, result.BaseRewardAmount)
	require.Equal(t, 5.0, result.UsageRebateAmount)
	require.Equal(t, 10.0, result.TotalRewardAmount)
	require.Equal(t, 2, repo.reads, "preflight and authoritative transaction must read separate snapshots")
}

func TestCheckinPersistsCampaignAuditSnapshot(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "campaign-audit@example.com", 10)
	repo := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	configureCheckinCampaignAwardBaseline(t, ctx, svc)
	createCheckinUsage(t, ctx, client, user.ID, 5, 5)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	svc.rewardRoll = func() float64 { return 0 }
	tiers := []domain.CheckinRewardTier{
		{Amount: 6, Probability: 25, SortOrder: 20},
		{Amount: 5, Probability: 75, SortOrder: 10},
	}
	normalizedTiers := []domain.CheckinRewardTier{
		{Amount: 5, Probability: 75, SortOrder: 1},
		{Amount: 6, Probability: 25, SortOrder: 2},
	}
	campaign := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "审计快照活动", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-15", tiers,
	)

	_, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	entity, err := client.UserCheckin.Query().Only(ctx)
	require.NoError(t, err)
	require.NotNil(t, entity.RewardCampaignID)
	require.Equal(t, campaign.ID, *entity.RewardCampaignID)
	require.Equal(t, "审计快照活动", entity.RewardCampaignName)
	require.Equal(t, normalizedTiers, entity.RewardCampaignTiersSnapshot)

	history, err := svc.ListHistoryForUser(ctx, user.ID, 7)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.NotNil(t, history[0].RewardCampaignID)
	require.Equal(t, campaign.ID, *history[0].RewardCampaignID)
	require.Equal(t, "审计快照活动", history[0].RewardCampaignName)
	require.Empty(t, history[0].RewardCampaignTiers)

	adminRecords, total, err := svc.ListRecords(ctx, 1, 10, CheckinListFilters{UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, history[0].RewardCampaignID, adminRecords[0].RewardCampaignID)
	require.Equal(t, normalizedTiers, adminRecords[0].RewardCampaignTiers)

	*history[0].RewardCampaignID = 999
	require.Equal(t, campaign.ID, *entity.RewardCampaignID, "DTO pointers must not alias entity storage")
}

func TestCheckinRechecksCampaignInsideTransaction(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "campaign-recheck@example.com", 10)
	repo := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	configureCheckinCampaignAwardBaseline(t, ctx, svc)
	createCheckinUsage(t, ctx, client, user.ID, 5, 5)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	campaign := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "发奖前停用", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-15", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)
	firstResolveFinished := make(chan struct{})
	var firstResolve sync.Once
	client.CheckinRewardCampaign.Intercept(dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
		return dbent.QuerierFunc(func(queryCtx context.Context, query dbent.Query) (dbent.Value, error) {
			value, err := next.Query(queryCtx, query)
			if err == nil {
				firstResolve.Do(func() { close(firstResolveFinished) })
			}
			return value, err
		})
	}))
	disableResult := make(chan error, 1)
	go func() {
		<-firstResolveFinished
		_, disableErr := svc.DisableRewardCampaign(ctx, campaign.ID, 99)
		disableResult <- disableErr
	}()

	result, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.NoError(t, <-disableResult)
	require.Equal(t, 1.0, result.BaseRewardAmount, "transaction-time disabled campaign must fall back to baseline")
	require.Nil(t, result.RewardCampaignID)
	require.Empty(t, result.RewardCampaignName)
	entity, err := client.UserCheckin.Query().Only(ctx)
	require.NoError(t, err)
	require.Nil(t, entity.RewardCampaignID)
	require.Empty(t, entity.RewardCampaignName)
	require.Empty(t, entity.RewardCampaignTiersSnapshot)
}

func TestAlreadyCheckedInKeepsOriginalCampaignReward(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "campaign-history@example.com", 10)
	repo := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	configureCheckinCampaignAwardBaseline(t, ctx, svc)
	createCheckinUsage(t, ctx, client, user.ID, 5, 5)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 9, 0, 0, 0, beijing) }
	campaign := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "领取后停用", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-15", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)
	first, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	_, err = client.CheckinRewardCampaign.UpdateOneID(campaign.ID).
		SetStatus(domain.CheckinRewardCampaignStatusDisabled).
		Save(ctx)
	require.NoError(t, err)
	repo.values[SettingKeyCheckinEnabled] = "false"

	status, err := svc.GetStatus(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, status.CheckedIn)
	require.NotNil(t, status.RewardCampaignID)
	require.Equal(t, campaign.ID, *status.RewardCampaignID)
	require.Equal(t, "领取后停用", status.RewardCampaignName)
	require.Equal(t, first.BaseRewardAmount, status.BaseRewardAmount)
	require.Equal(t, first.TotalRewardAmount, status.TotalRewardAmount)

	repo.values[SettingKeyCheckinEnabled] = "true"
	second, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, second.AlreadyCheckedIn)
	require.NotNil(t, second.RewardCampaignID)
	require.Equal(t, campaign.ID, *second.RewardCampaignID)
	require.Equal(t, first.TotalRewardAmount, second.TotalRewardAmount)
}

func TestCheckinAfterCampaignEndUsesBaseline(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "campaign-ended@example.com", 10)
	repo := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	configureCheckinCampaignAwardBaseline(t, ctx, svc)
	createCheckinUsage(t, ctx, client, user.ID, 5, 5)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 16, 9, 0, 0, 0, beijing) }
	createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "已经结束", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-15", []domain.CheckinRewardTier{{Amount: 5, Probability: 100}},
	)

	status, err := svc.GetStatus(ctx, user.ID)
	require.NoError(t, err)
	require.Nil(t, status.RewardCampaignID)
	require.Empty(t, status.RewardCampaignName)
	result, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 1.0, result.BaseRewardAmount)
	require.Nil(t, result.RewardCampaignID)
	require.Empty(t, result.RewardCampaignName)
	require.Empty(t, result.RecentRecords[0].RewardCampaignTiers)
}

func TestCheckinRecordFromEntityClonesCampaignAuditFields(t *testing.T) {
	campaignID := int64(42)
	entity := &dbent.UserCheckin{
		ID:                          1,
		UserID:                      2,
		CheckinDate:                 "2026-08-15",
		RewardCampaignID:            &campaignID,
		RewardCampaignName:          "不可别名",
		RewardCampaignTiersSnapshot: []domain.CheckinRewardTier{{Amount: 5, Probability: 100, SortOrder: 1}},
	}

	record := checkinRecordFromEntity(entity)
	require.NotNil(t, record.RewardCampaignID)
	*record.RewardCampaignID = 99
	record.RewardCampaignTiers[0].Amount = 99
	require.Equal(t, int64(42), *entity.RewardCampaignID)
	require.Equal(t, 5.0, entity.RewardCampaignTiersSnapshot[0].Amount)

	baseline := checkinRecordFromEntity(&dbent.UserCheckin{ID: 2, UserID: 3, CheckinDate: "2026-08-16"})
	require.Nil(t, baseline.RewardCampaignID)
	require.Empty(t, baseline.RewardCampaignName)
	require.Empty(t, baseline.RewardCampaignTiers)
}

func TestCheckinRetryAfterPolicyChangeReturnsStoredAward(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "policy-changed-retry@example.com", 15)
	rewardedAt := time.Date(2026, 8, 15, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	_, err := client.UserCheckin.Create().
		SetUserID(user.ID).
		SetCheckinDate("2026-08-15").
		SetBaseRewardAmount(5).
		SetTotalRewardAmount(5).
		SetRewardAmount(5).
		SetBalanceBefore(10).
		SetBalanceAfter(15).
		SetCreatedAt(rewardedAt).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UserCheckinBlacklist.Create().SetUserID(user.ID).SetReason("changed later").Save(ctx)
	require.NoError(t, err)
	repo := newCheckinSettingRepoStub()
	repo.values[SettingKeyCheckinEnabled] = "false"
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(repo)
	svc.now = func() time.Time { return rewardedAt }

	result, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, result.AlreadyCheckedIn)
	require.Equal(t, 5.0, result.TotalRewardAmount)
	require.Equal(t, 15.0, result.BalanceAfter)
}

func TestAlreadyCheckedInReflectsCurrentPolicyState(t *testing.T) {
	testCases := []struct {
		name              string
		configure         func(t *testing.T, ctx context.Context, client *dbent.Client, repo *checkinSettingRepoStub, userID int64)
		expectedEnabled   bool
		expectedEligible  bool
		expectedBlacklist bool
		expectedReason    string
	}{
		{
			name: "retry after disabled",
			configure: func(_ *testing.T, _ context.Context, _ *dbent.Client, repo *checkinSettingRepoStub, _ int64) {
				repo.values[SettingKeyCheckinEnabled] = "false"
			},
			expectedReason: CheckinIneligibleReasonDisabled,
		},
		{
			name: "retry after blacklist",
			configure: func(t *testing.T, ctx context.Context, client *dbent.Client, repo *checkinSettingRepoStub, userID int64) {
				repo.values[SettingKeyCheckinEnabled] = "true"
				_, err := client.UserCheckinBlacklist.Create().SetUserID(userID).SetReason("changed later").Save(ctx)
				require.NoError(t, err)
			},
			expectedBlacklist: true,
			expectedReason:    CheckinIneligibleReasonBlacklisted,
		},
		{
			name: "retry after threshold increase",
			configure: func(_ *testing.T, _ context.Context, _ *dbent.Client, repo *checkinSettingRepoStub, _ int64) {
				repo.values[SettingKeyCheckinEnabled] = "true"
				repo.values[SettingKeyCheckinMinTotalUsageUSD] = "100"
			},
			expectedEnabled: true,
			expectedReason:  CheckinIneligibleReasonInsufficientSpend,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			client := newCheckinServiceTestClient(t)
			ctx := context.Background()
			user := createCheckinTestUser(t, ctx, client, "policy-state-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"@example.com", 15)
			rewardedAt := time.Date(2026, 8, 15, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
			_, err := client.UserCheckin.Create().
				SetUserID(user.ID).
				SetCheckinDate("2026-08-15").
				SetBaseRewardAmount(5).
				SetTotalRewardAmount(5).
				SetRewardAmount(5).
				SetBalanceBefore(10).
				SetBalanceAfter(15).
				SetCreatedAt(rewardedAt).
				Save(ctx)
			require.NoError(t, err)
			repo := newCheckinSettingRepoStub()
			testCase.configure(t, ctx, client, repo, user.ID)
			svc := NewCheckinService(client, nil, nil)
			svc.SetSettingRepository(repo)
			svc.now = func() time.Time { return rewardedAt }

			status, err := svc.GetStatus(ctx, user.ID)
			require.NoError(t, err)
			result, err := svc.Checkin(ctx, user.ID)
			require.NoError(t, err)
			require.True(t, result.AlreadyCheckedIn)
			require.Equal(t, 5.0, result.TotalRewardAmount)
			require.Equal(t, status.Enabled, result.Enabled)
			require.Equal(t, status.Eligible, result.Eligible)
			require.Equal(t, status.Blacklisted, result.Blacklisted)
			require.Equal(t, status.IneligibleReason, result.IneligibleReason)
			require.Equal(t, testCase.expectedEnabled, result.Enabled)
			require.Equal(t, testCase.expectedEligible, result.Eligible)
			require.Equal(t, testCase.expectedBlacklist, result.Blacklisted)
			require.Equal(t, testCase.expectedReason, result.IneligibleReason)
		})
	}
}

func TestCheckinTransactionExistingRecordSkipsBalanceMutation(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "transaction-existing@example.com", 10)
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(newCheckinSettingRepoStub())
	rewardedAt := time.Date(2026, 8, 15, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	svc.now = func() time.Time { return rewardedAt }
	outerQueries := 0
	client.UserCheckin.Intercept(dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
		return dbent.QuerierFunc(func(queryCtx context.Context, query dbent.Query) (dbent.Value, error) {
			value, queryErr := next.Query(queryCtx, query)
			outerQueries++
			if queryErr == nil && outerQueries == 1 {
				_, createErr := client.UserCheckin.Create().
					SetUserID(user.ID).
					SetCheckinDate("2026-08-15").
					SetBaseRewardAmount(3).
					SetTotalRewardAmount(3).
					SetRewardAmount(3).
					SetBalanceBefore(10).
					SetBalanceAfter(13).
					SetCreatedAt(rewardedAt).
					Save(queryCtx)
				require.NoError(t, createErr)
			}
			return value, queryErr
		})
	}))
	userUpdates := 0
	client.User.Use(func(next dbent.Mutator) dbent.Mutator {
		return dbent.MutateFunc(func(mutationCtx context.Context, mutation dbent.Mutation) (dbent.Value, error) {
			if mutation.Op().Is(coreent.OpUpdateOne) {
				userUpdates++
			}
			return next.Mutate(mutationCtx, mutation)
		})
	})

	result, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, result.AlreadyCheckedIn)
	require.Equal(t, 3.0, result.TotalRewardAmount)
	require.Zero(t, userUpdates, "transaction must check idempotency before touching balance")
}

func TestCheckinHistorySnapshotProjectsDatesAndLimitsRecentPayload(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "history-projection@example.com", 10)
	start := time.Date(2026, 7, 27, 9, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	campaign, err := client.CheckinRewardCampaign.Create().
		SetName("large audit").
		SetStatus(domain.CheckinRewardCampaignStatusEnabled).
		SetStartDate(start).
		SetEndDate(start.AddDate(0, 0, 19)).
		SetRewardTiers([]domain.CheckinRewardTier{{Amount: 5, Probability: 100}}).
		Save(ctx)
	require.NoError(t, err)
	for index := 0; index < 20; index++ {
		_, err := client.UserCheckin.Create().
			SetUserID(user.ID).
			SetCheckinDate(start.AddDate(0, 0, index).Format("2006-01-02")).
			SetRewardCampaignID(campaign.ID).
			SetRewardCampaignName("large audit").
			SetRewardCampaignTiersSnapshot([]domain.CheckinRewardTier{{Amount: 5, Probability: 100}}).
			SetBaseRewardAmount(5).
			SetTotalRewardAmount(5).
			SetRewardAmount(5).
			Save(ctx)
		require.NoError(t, err)
	}
	type queryShape struct {
		fields []string
		limit  int
	}
	shapes := make([]queryShape, 0)
	client.UserCheckin.Intercept(dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
		return dbent.QuerierFunc(func(queryCtx context.Context, query dbent.Query) (dbent.Value, error) {
			if qc := coreent.QueryFromContext(queryCtx); qc != nil {
				shape := queryShape{fields: append([]string(nil), qc.Fields...)}
				if qc.Limit != nil {
					shape.limit = *qc.Limit
				}
				shapes = append(shapes, shape)
			}
			return next.Query(queryCtx, query)
		})
	}))
	svc := NewCheckinService(client, nil, nil)
	cfg := DefaultCheckinConfig()

	snapshot, err := svc.checkinHistorySnapshot(ctx, user.ID, "2026-08-15", cfg)
	require.NoError(t, err)
	require.Len(t, snapshot.RecentRecords, 7)
	for _, record := range snapshot.RecentRecords {
		require.Empty(t, record.RewardCampaignTiers, "user history must not expose campaign tier snapshots")
	}
	require.Contains(t, shapes, queryShape{fields: []string{"checkin_date"}, limit: checkinHistoryLookbackLimit(cfg)})
	require.Contains(t, shapes, queryShape{limit: 7})
}

func TestUserCheckinDateUniqueConstraintClassifier(t *testing.T) {
	require.True(t, isUserCheckinDateUniqueConstraintError(&pq.Error{
		Code:       "23505",
		Constraint: "user_checkins_user_id_date_uq",
	}))
	require.True(t, isUserCheckinDateUniqueConstraintError(errors.New(
		"constraint failed: UNIQUE constraint failed: user_checkins.user_id, user_checkins.checkin_date",
	)))
	require.False(t, isUserCheckinDateUniqueConstraintError(&pq.Error{
		Code:       "23505",
		Constraint: "users_email_key",
	}))
	require.False(t, isUserCheckinDateUniqueConstraintError(&pq.Error{
		Code:       "23503",
		Constraint: "user_checkins_reward_campaign_id_fkey",
	}))
	require.False(t, isUserCheckinDateUniqueConstraintError(errors.New("constraint failed: FOREIGN KEY constraint failed")))
	require.False(t, isUserCheckinDateUniqueConstraintError(errors.New(
		"constraint failed: UNIQUE constraint failed: user_checkins.user_id, user_checkins.checkin_date, user_checkins.id",
	)))

	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "sqlite-unique-classifier@example.com", 10)
	create := func() error {
		_, err := client.UserCheckin.Create().
			SetUserID(user.ID).
			SetCheckinDate("2026-08-15").
			SetRewardAmount(1).
			Save(ctx)
		return err
	}
	require.NoError(t, create())
	require.True(t, isUserCheckinDateUniqueConstraintError(create()))
}

func TestCheckinNonUniqueConstraintDoesNotBecomeNotFound(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "non-unique-constraint@example.com", 10)
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(newCheckinSettingRepoStub())
	client.UserCheckin.Use(func(next dbent.Mutator) dbent.Mutator {
		return dbent.MutateFunc(func(mutationCtx context.Context, mutation dbent.Mutation) (dbent.Value, error) {
			if mutation.Op().Is(coreent.OpCreate) {
				return nil, errors.New("constraint failed: FOREIGN KEY constraint failed")
			}
			return next.Mutate(mutationCtx, mutation)
		})
	})

	_, err := svc.Checkin(ctx, user.ID)
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrCheckinNotFound)
	require.ErrorContains(t, err, "create check-in record")
}

func TestCheckinRewardForRollUsesWeightedTiers(t *testing.T) {
	cfg := *DefaultCheckinConfig()
	require.Equal(t, 1.0, selectCheckinReward(cfg, 0))
	require.Equal(t, 1.0, selectCheckinReward(cfg, 0.3199))
	require.Equal(t, 2.0, selectCheckinReward(cfg, 0.32))
	require.Equal(t, 2.0, selectCheckinReward(cfg, 0.5699))
	require.Equal(t, 3.0, selectCheckinReward(cfg, 0.5701))
	require.Equal(t, 4.0, selectCheckinReward(cfg, 0.75))
	require.Equal(t, 4.5, selectCheckinReward(cfg, 0.85))
	require.Equal(t, 5.0, selectCheckinReward(cfg, 0.93))
	require.Equal(t, 10.0, selectCheckinReward(cfg, 0.98))
	require.Equal(t, 10.0, selectCheckinReward(cfg, 0.9999))
}

func TestCheckinServiceCheckinAwardsBalanceForBeijingDate(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()

	createdUser, err := client.User.Create().
		SetEmail("checkin-user@example.com").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetStatus(StatusActive).
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(newCheckinSettingRepoStub())
	svc.now = func() time.Time {
		return time.Date(2026, 6, 4, 16, 30, 0, 0, time.UTC)
	}
	svc.rewardRoll = func() float64 { return 0.5 }

	result, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.False(t, result.AlreadyCheckedIn)
	require.True(t, result.CheckedIn)
	require.Equal(t, "2026-06-05", result.CheckinDate)
	require.Equal(t, 2.0, result.RewardAmount)
	require.Equal(t, 10.0, result.BalanceBefore)
	require.Equal(t, 12.0, result.BalanceAfter)
	require.Equal(t, 1, result.StreakDay)
	require.Equal(t, 2.0, result.BaseRewardAmount)
	require.Equal(t, 0.0, result.BonusRewardAmount)

	updatedUser, err := client.User.Get(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 12.0, updatedUser.Balance)

	records, err := client.UserCheckin.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "2026-06-05", records[0].CheckinDate)
	require.Equal(t, 2.0, records[0].RewardAmount)

	historyCount, err := client.RedeemCode.Query().
		Where(
			redeemcode.UsedByEQ(createdUser.ID),
			redeemcode.TypeEQ(AdjustmentTypeCheckinReward),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, historyCount)
}

func TestCheckinServiceSecondCheckinSameBeijingDateDoesNotAwardAgain(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()

	createdUser, err := client.User.Create().
		SetEmail("checkin-once@example.com").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetStatus(StatusActive).
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(newCheckinSettingRepoStub())
	svc.now = func() time.Time {
		return time.Date(2026, 6, 5, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	}
	svc.rewardRoll = func() float64 { return 0 }

	first, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 1.0, first.RewardAmount)

	svc.rewardRoll = func() float64 { return 0.99 }
	second, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.True(t, second.AlreadyCheckedIn)
	require.Equal(t, 1.0, second.RewardAmount)
	require.Equal(t, 11.0, second.BalanceAfter)

	updatedUser, err := client.User.Get(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 11.0, updatedUser.Balance)

	checkinCount, err := client.UserCheckin.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, checkinCount)

	historyCount, err := client.RedeemCode.Query().
		Where(redeemcode.TypeEQ(AdjustmentTypeCheckinReward)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, historyCount)
}

func TestCheckinServiceBlacklistBlocksStatusAndCheckin(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()

	createdUser, err := client.User.Create().
		SetEmail("blocked@example.com").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetStatus(StatusActive).
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserCheckinBlacklist.Create().
		SetUserID(createdUser.ID).
		SetReason("manual block").
		SetCreatedBy(1).
		Save(ctx)
	require.NoError(t, err)

	svc := NewCheckinService(client, nil, nil)
	status, err := svc.GetStatus(ctx, createdUser.ID)
	require.NoError(t, err)
	require.True(t, status.Blacklisted)
	require.False(t, status.Enabled)

	_, err = svc.Checkin(ctx, createdUser.ID)
	require.ErrorIs(t, err, ErrCheckinBlacklisted)
}

func TestCheckinServiceGlobalDisabledBlocksStatusAndCheckin(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "disabled@example.com", 10)
	settings := newCheckinSettingRepoStub()
	setCheckinConfig(t, settings, CheckinConfig{Enabled: false})

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)

	status, err := svc.GetStatus(ctx, createdUser.ID)
	require.NoError(t, err)
	require.False(t, status.Enabled)
	require.False(t, status.Eligible)
	require.Equal(t, CheckinIneligibleReasonDisabled, status.IneligibleReason)

	_, err = svc.Checkin(ctx, createdUser.ID)
	require.ErrorIs(t, err, ErrCheckinDisabled)
}

func TestCheckinServiceMinimumSpendBlocksLowUsageUser(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "low-spend@example.com", 10)
	settings := newCheckinSettingRepoStub()
	setCheckinConfig(t, settings, CheckinConfig{Enabled: true, MinTotalUsageUSD: 5})
	createCheckinUsage(t, ctx, client, createdUser.ID, 2.5, 2.5)

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)

	status, err := svc.GetStatus(ctx, createdUser.ID)
	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.False(t, status.Eligible)
	require.Equal(t, CheckinIneligibleReasonInsufficientSpend, status.IneligibleReason)
	require.Equal(t, 5.0, status.MinTotalUsageUSD)
	require.Equal(t, 2.5, status.TotalUsageUSD)

	_, err = svc.Checkin(ctx, createdUser.ID)
	require.ErrorIs(t, err, ErrCheckinInsufficientSpend)

	updatedUser, err := client.User.Get(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 10.0, updatedUser.Balance)

	checkinCount, err := client.UserCheckin.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 0, checkinCount)
}

func TestCheckinServiceMinimumSpendAllowsEligibleUser(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "enough-spend@example.com", 10)
	settings := newCheckinSettingRepoStub()
	setCheckinConfig(t, settings, CheckinConfig{Enabled: true, MinTotalUsageUSD: 5})
	createCheckinUsage(t, ctx, client, createdUser.ID, 7.5, 7.5)

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	svc.rewardRoll = func() float64 { return 0 }

	status, err := svc.GetStatus(ctx, createdUser.ID)
	require.NoError(t, err)
	require.True(t, status.Enabled)
	require.True(t, status.Eligible)
	require.Empty(t, status.IneligibleReason)
	require.Equal(t, 5.0, status.MinTotalUsageUSD)
	require.Equal(t, 7.5, status.TotalUsageUSD)

	result, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.False(t, result.AlreadyCheckedIn)
	require.Equal(t, 1.0, result.RewardAmount)
	require.Equal(t, 11.0, result.BalanceAfter)

	usageRows, err := client.UsageLog.Query().
		Where(usagelog.UserIDEQ(createdUser.ID)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, usageRows)
}

func TestCheckinServiceMinimumSpendUsesActualCost(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "actual-spend@example.com", 10)
	settings := newCheckinSettingRepoStub()
	setCheckinConfig(t, settings, CheckinConfig{Enabled: true, MinTotalUsageUSD: 5})
	createCheckinUsage(t, ctx, client, createdUser.ID, 8, 4)

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)

	status, err := svc.GetStatus(ctx, createdUser.ID)
	require.NoError(t, err)
	require.False(t, status.Eligible)
	require.Equal(t, 4.0, status.TotalUsageUSD)
}

func TestTotalUsageUSDWithClient_ThresholdExemptCost(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "gift-usage-total@example.com", 10)
	createCheckinUsageWithThresholdExempt(t, ctx, client, createdUser.ID, "gift-total-full", 10, 10, nil)
	createCheckinUsageWithThresholdExempt(t, ctx, client, createdUser.ID, "gift-total-mixed", 12, 5, nil)
	createCheckinUsageWithThresholdExempt(t, ctx, client, createdUser.ID, "gift-total-clamped", 2, 5, nil)

	svc := NewCheckinService(client, nil, nil)
	total, err := svc.totalUsageUSDWithClient(ctx, client, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 7.0, total)
}

func TestCheckinServiceMinimumSpendGiftUsageEligibility(t *testing.T) {
	t.Run("fully gift funded usage is ineligible", func(t *testing.T) {
		client := newCheckinServiceTestClient(t)
		ctx := context.Background()
		createdUser := createCheckinTestUser(t, ctx, client, "gift-usage-full@example.com", 10)
		settings := newCheckinSettingRepoStub()
		setCheckinConfig(t, settings, CheckinConfig{Enabled: true, MinTotalUsageUSD: 5})
		createCheckinUsageWithThresholdExempt(t, ctx, client, createdUser.ID, "gift-eligibility-full", 8, 8, nil)

		svc := NewCheckinService(client, nil, nil)
		svc.SetSettingRepository(settings)
		status, err := svc.GetStatus(ctx, createdUser.ID)
		require.NoError(t, err)
		require.False(t, status.Eligible)
		require.Equal(t, CheckinIneligibleReasonInsufficientSpend, status.IneligibleReason)
		require.Zero(t, status.TotalUsageUSD)
		_, err = svc.Checkin(ctx, createdUser.ID)
		require.ErrorIs(t, err, ErrCheckinInsufficientSpend)
	})

	t.Run("mixed usage counts only ordinary funded portion", func(t *testing.T) {
		client := newCheckinServiceTestClient(t)
		ctx := context.Background()
		createdUser := createCheckinTestUser(t, ctx, client, "gift-usage-mixed@example.com", 10)
		settings := newCheckinSettingRepoStub()
		setCheckinConfig(t, settings, CheckinConfig{Enabled: true, MinTotalUsageUSD: 5})
		createCheckinUsageWithThresholdExempt(t, ctx, client, createdUser.ID, "gift-eligibility-mixed", 8, 4, nil)

		svc := NewCheckinService(client, nil, nil)
		svc.SetSettingRepository(settings)
		status, err := svc.GetStatus(ctx, createdUser.ID)
		require.NoError(t, err)
		require.False(t, status.Eligible)
		require.Equal(t, 4.0, status.TotalUsageUSD)

		createCheckinUsageWithThresholdExempt(t, ctx, client, createdUser.ID, "gift-eligibility-ordinary", 1, 0, nil)
		status, err = svc.GetStatus(ctx, createdUser.ID)
		require.NoError(t, err)
		require.True(t, status.Eligible)
		require.Equal(t, 5.0, status.TotalUsageUSD)
		svc.rewardRoll = func() float64 { return 0 }
		_, err = svc.Checkin(ctx, createdUser.ID)
		require.NoError(t, err)
	})
}

func TestCheckinServiceRechargeEligibility(t *testing.T) {
	tests := []struct {
		name           string
		minUsage       float64
		minRecharge    float64
		totalUsage     float64
		totalRecharged float64
		eligible       bool
	}{
		{name: "usage route met", minUsage: 5, minRecharge: 10, totalUsage: 5, eligible: true},
		{name: "recharge route met", minUsage: 5, minRecharge: 10, totalRecharged: 10, eligible: true},
		{name: "neither route met", minUsage: 5, minRecharge: 10, totalUsage: 4, totalRecharged: 9, eligible: false},
		{name: "recharge only enabled", minRecharge: 10, totalUsage: 100, totalRecharged: 9, eligible: false},
		{name: "usage only remains compatible", minUsage: 5, totalUsage: 5, eligible: true},
		{name: "both disabled is unrestricted", eligible: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newCheckinServiceTestClient(t)
			ctx := context.Background()
			createdUser := createCheckinTestUser(t, ctx, client, "eligibility-"+strconv.FormatInt(time.Now().UnixNano(), 10)+"@example.com", 10)
			if tt.totalRecharged > 0 {
				_, err := client.User.UpdateOneID(createdUser.ID).
					SetTotalRecharged(tt.totalRecharged).
					Save(ctx)
				require.NoError(t, err)
			}
			if tt.totalUsage > 0 {
				createCheckinUsage(t, ctx, client, createdUser.ID, tt.totalUsage, tt.totalUsage)
			}

			settings := newCheckinSettingRepoStub()
			setCheckinConfig(t, settings, CheckinConfig{
				Enabled:             true,
				MinTotalUsageUSD:    tt.minUsage,
				MinTotalRechargeUSD: tt.minRecharge,
			})
			svc := NewCheckinService(client, nil, nil)
			svc.SetSettingRepository(settings)
			svc.rewardRoll = func() float64 { return 0 }

			status, err := svc.GetStatus(ctx, createdUser.ID)
			require.NoError(t, err)
			require.Equal(t, tt.eligible, status.Eligible)
			require.Equal(t, tt.minUsage, status.MinTotalUsageUSD)
			require.Equal(t, tt.minRecharge, status.MinTotalRechargeUSD)
			require.Equal(t, tt.totalUsage, status.TotalUsageUSD)
			require.Equal(t, tt.totalRecharged, status.TotalRechargeUSD)

			result, err := svc.Checkin(ctx, createdUser.ID)
			if tt.eligible {
				require.NoError(t, err)
				require.NotNil(t, result)
				return
			}
			require.ErrorIs(t, err, ErrCheckinInsufficientEligibility)
			require.Equal(t, CheckinIneligibleReasonInsufficientEligibility, status.IneligibleReason)
		})
	}
}

func TestCheckinServiceRejectsNegativeRechargeThreshold(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)

	_, err := svc.UpdateConfig(context.Background(), CheckinConfig{
		Enabled:             true,
		MinTotalRechargeUSD: -1,
		Tiers:               DefaultCheckinConfig().Tiers,
	})
	require.Error(t, err)
}

func TestCheckinServicePersistsRechargeThreshold(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	defaults := DefaultCheckinConfig()

	updated, err := svc.UpdateConfig(context.Background(), CheckinConfig{
		Enabled:             true,
		MinTotalUsageUSD:    5,
		MinTotalRechargeUSD: 20,
		Tiers:               defaults.Tiers,
		StreakEnabled:       defaults.StreakEnabled,
		StreakRules:         defaults.StreakRules,
	})
	require.NoError(t, err)
	require.Equal(t, 20.0, updated.MinTotalRechargeUSD)
	require.Equal(t, "20", settings.values[SettingKeyCheckinMinTotalRechargeUSD])

	loaded, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, 5.0, loaded.MinTotalUsageUSD)
	require.Equal(t, 20.0, loaded.MinTotalRechargeUSD)
}

func TestCheckinConfig_NormalizesUsageRebateWithLegacyStreakRule(t *testing.T) {
	cfg := CheckinConfig{
		Enabled:                true,
		Tiers:                  []CheckinRewardTier{{Amount: 0.3, Probability: 100}},
		StreakEnabled:          true,
		StreakRules:            []CheckinStreakRule{{Day: 7, BonusAmount: 10}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         8,
		TotalRewardCap:         10,
	}

	normalized, err := normalizeCheckinConfig(cfg)
	require.NoError(t, err)
	require.True(t, normalized.UsageRebateEnabled)
	require.Equal(t, 8.0, normalized.UsageRebateRatePercent)
	require.Equal(t, 8.0, normalized.UsageRebateCap)
	require.Equal(t, 10.0, normalized.TotalRewardCap)
	require.Equal(t, 1, normalized.StreakRules[0].LotteryAttempts)
	require.Zero(t, normalized.StreakRules[0].BonusAmount)
	require.Zero(t, normalized.StreakRules[0].BonusRatePercent)
}

func TestCheckinConfig_LegacyFixedStreakBecomesOneLotteryAttempt(t *testing.T) {
	cfg := CheckinConfig{
		Enabled:       true,
		Tiers:         []CheckinRewardTier{{Amount: 1, Probability: 100}},
		StreakEnabled: true,
		StreakRules:   []CheckinStreakRule{{Day: 7, BonusAmount: 4}},
	}

	normalized, err := normalizeCheckinConfig(cfg)
	require.NoError(t, err)
	require.False(t, normalized.UsageRebateEnabled)
	require.Equal(t, 1, normalized.StreakRules[0].LotteryAttempts)
	require.Zero(t, normalized.StreakRules[0].BonusAmount)
	require.Zero(t, normalized.StreakRules[0].BonusRatePercent)
}

func TestCheckinConfig_UsageRebatePreservesStreakLotteryAttempts(t *testing.T) {
	cfg := CheckinConfig{
		Enabled:                true,
		Tiers:                  []CheckinRewardTier{{Amount: 1, Probability: 100}},
		StreakEnabled:          true,
		StreakRules:            []CheckinStreakRule{{Day: 7, BonusAmount: 4}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         8,
		TotalRewardCap:         10,
	}

	normalized, err := normalizeCheckinConfig(cfg)
	require.NoError(t, err)
	require.Equal(t, 1, normalized.StreakRules[0].LotteryAttempts)
	require.Zero(t, normalized.StreakRules[0].BonusAmount)
	require.Zero(t, normalized.StreakRules[0].BonusRatePercent)
}

func TestCheckinConfig_RejectsInvalidUsageRebateValues(t *testing.T) {
	valid := func() CheckinConfig {
		return CheckinConfig{
			Enabled:                true,
			Tiers:                  []CheckinRewardTier{{Amount: 0.3, Probability: 100}},
			UsageRebateEnabled:     true,
			UsageRebateRatePercent: 8,
			UsageRebateCap:         8,
			TotalRewardCap:         10,
		}
	}
	tests := []struct {
		name   string
		mutate func(*CheckinConfig)
	}{
		{"nan rate", func(cfg *CheckinConfig) { cfg.UsageRebateRatePercent = math.NaN() }},
		{"infinite cap", func(cfg *CheckinConfig) { cfg.UsageRebateCap = math.Inf(1) }},
		{"negative rate", func(cfg *CheckinConfig) { cfg.UsageRebateRatePercent = -1 }},
		{"rate over one hundred", func(cfg *CheckinConfig) { cfg.UsageRebateRatePercent = 101 }},
		{"zero rebate cap", func(cfg *CheckinConfig) { cfg.UsageRebateCap = 0 }},
		{"zero total cap", func(cfg *CheckinConfig) { cfg.TotalRewardCap = 0 }},
		{"total cap below minimum base", func(cfg *CheckinConfig) { cfg.TotalRewardCap = 0.2 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := valid()
			tt.mutate(&cfg)
			_, err := normalizeCheckinConfig(cfg)
			require.Error(t, err)
		})
	}
}

func TestCheckinServicePersistsUsageRebateConfig(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)

	updated, err := svc.UpdateConfig(context.Background(), CheckinConfig{
		Enabled:                true,
		Tiers:                  []CheckinRewardTier{{Amount: 0.3, Probability: 100}},
		StreakEnabled:          true,
		StreakRules:            []CheckinStreakRule{{Day: 7, BonusAmount: 10}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         8,
		TotalRewardCap:         10,
	})
	require.NoError(t, err)
	require.True(t, updated.UsageRebateEnabled)
	require.JSONEq(t, `{
		"tiers":[{"amount":0.3,"probability":100,"sort_order":1}],
		"streak_enabled":true,
		"streak_rules":[{"day":7,"lottery_attempts":1}],
		"usage_rebate_enabled":true,
		"usage_rebate_rate_percent":8,
		"usage_rebate_cap":8,
		"total_reward_cap":10
	}`, settings.values[SettingKeyCheckinRewardConfig])

	loaded, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, updated, loaded)
}

func TestCheckinServiceLoadsLegacyRewardConfigWithUsageRebateDisabled(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	settings := newCheckinSettingRepoStub()
	settings.values[SettingKeyCheckinRewardConfig] = `{
		"tiers":[{"amount":1,"probability":100,"sort_order":1}],
		"streak_enabled":true,
		"streak_rules":[{"day":7,"bonus_amount":4}]
	}`
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)

	loaded, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.False(t, loaded.UsageRebateEnabled)
	require.Zero(t, loaded.UsageRebateRatePercent)
	require.Zero(t, loaded.UsageRebateCap)
	require.Zero(t, loaded.TotalRewardCap)
	require.Equal(t, 1, loaded.StreakRules[0].LotteryAttempts)
	require.Zero(t, loaded.StreakRules[0].BonusAmount)
	require.Zero(t, loaded.StreakRules[0].BonusRatePercent)
}

func TestCheckinConfigDefaultsToFiveDailyUsageRecordsAndOneAttemptPerMilestone(t *testing.T) {
	cfg := DefaultCheckinConfig()

	require.Equal(t, 5, cfg.MinDailyUsageCount)
	require.NotEmpty(t, cfg.StreakRules)
	for _, rule := range cfg.StreakRules {
		require.Equal(t, 1, rule.LotteryAttempts)
	}
}

func TestCheckinServiceLoadsLegacyStreakMoneyRulesAsOneLotteryAttempt(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	settings := newCheckinSettingRepoStub()
	settings.values[SettingKeyCheckinRewardConfig] = `{
		"tiers":[{"amount":1,"probability":100,"sort_order":1}],
		"streak_enabled":true,
		"streak_rules":[{"day":7,"bonus_amount":10},{"day":15,"bonus_amount":15}]
	}`
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)

	loaded, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, []CheckinStreakRule{
		{Day: 7, LotteryAttempts: 1},
		{Day: 15, LotteryAttempts: 1},
	}, loaded.StreakRules)
}

func TestCheckinServiceDailyUsageThresholdUsesBeijingCalendarDay(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "daily-usage-threshold@example.com", 0)
	beijing := time.FixedZone("CST", 8*60*60)
	svc := NewCheckinService(client, nil, nil)
	svc.now = func() time.Time { return time.Date(2026, 9, 2, 12, 0, 0, 0, beijing) }

	createCheckinUsageAt(t, ctx, client, user.ID, "previous-day", 1, time.Date(2026, 9, 1, 23, 59, 59, 0, beijing))
	for index := 0; index < 4; index++ {
		createCheckinUsageAt(
			t, ctx, client, user.ID, fmt.Sprintf("today-%d", index), 1,
			time.Date(2026, 9, 2, 8+index, 0, 0, 0, beijing),
		)
	}

	status, err := svc.GetStatus(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, status.Eligible)
	require.Equal(t, CheckinIneligibleReasonInsufficientDailyUsage, status.IneligibleReason)
	require.Equal(t, 4, status.TodayUsageCount)
	require.Equal(t, 5, status.MinDailyUsageCount)

	_, err = svc.Checkin(ctx, user.ID)
	require.ErrorIs(t, err, ErrCheckinInsufficientDailyUsage)

	createCheckinUsageAt(t, ctx, client, user.ID, "today-4", 1, time.Date(2026, 9, 2, 12, 30, 0, 0, beijing))
	status, err = svc.GetStatus(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, status.Eligible)
	require.Equal(t, 5, status.TodayUsageCount)
}

func TestCheckinServicePersistsDailyUsageThreshold(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	defaults := DefaultCheckinConfig()
	defaults.MinDailyUsageCount = 8

	updated, err := svc.UpdateConfig(context.Background(), *defaults)
	require.NoError(t, err)
	require.Equal(t, 8, updated.MinDailyUsageCount)
	require.Equal(t, "8", settings.values[SettingKeyCheckinMinDailyUsageCount])

	loaded, err := svc.GetConfig(context.Background())
	require.NoError(t, err)
	require.Equal(t, 8, loaded.MinDailyUsageCount)
}

func TestCalculateUsageLinkedCheckinReward(t *testing.T) {
	tests := []struct {
		name              string
		base              float64
		usage             float64
		streakBonus       float64
		wantRebate        float64
		wantStreak        float64
		wantTotal         float64
		wantCapAdjustment float64
	}{
		{name: "zero usage", base: 0.5, usage: 0, wantTotal: 0.5},
		{name: "normal usage", base: 0.8, usage: 50, wantRebate: 4, wantTotal: 4.8},
		{name: "rebate cap", base: 0.3, usage: 500, wantRebate: 8, wantTotal: 8.3, wantCapAdjustment: 32},
		{name: "streak attempts do not change balance cap", base: 3, usage: 100, streakBonus: 4, wantRebate: 7, wantStreak: 0, wantTotal: 10, wantCapAdjustment: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := CheckinConfig{
				Tiers:                  []CheckinRewardTier{{Amount: tt.base, Probability: 100}},
				UsageRebateEnabled:     true,
				UsageRebateRatePercent: 8,
				UsageRebateCap:         8,
				TotalRewardCap:         10,
				StreakEnabled:          tt.streakBonus > 0,
			}
			streakDay := 0
			if tt.streakBonus > 0 {
				streakDay = 7
				cfg.StreakRules = []CheckinStreakRule{{Day: streakDay, BonusAmount: tt.streakBonus}}
			}

			got := calculateUsageLinkedCheckinReward(cfg, tt.usage, tt.base, streakDay)
			require.Equal(t, tt.usage, got.PreviousDayUsage)
			require.Equal(t, tt.base, got.BaseReward)
			require.Equal(t, tt.wantRebate, got.UsageRebate)
			require.Equal(t, tt.wantStreak, got.StreakBonus)
			require.Equal(t, tt.wantTotal, got.TotalReward)
			require.Equal(t, tt.wantCapAdjustment, got.CapAdjustment)
		})
	}
}

func TestCalculateUsageLinkedCheckinReward_ExcludesStreakAttemptsFromBalance(t *testing.T) {
	cfg := CheckinConfig{
		Tiers:                  []CheckinRewardTier{{Amount: 3, Probability: 100}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         8,
		TotalRewardCap:         10,
		StreakEnabled:          true,
		StreakRules:            []CheckinStreakRule{{Day: 7, BonusAmount: 4}},
	}

	got := calculateUsageLinkedCheckinReward(cfg, 100, 3, 7)

	require.Equal(t, 3.0, got.BaseReward)
	require.Equal(t, 7.0, got.UsageRebate)
	require.Zero(t, got.StreakBonus)
	require.Equal(t, 10.0, got.TotalReward)
	require.Equal(t, 1.0, got.CapAdjustment)
}

func TestPreviousBeijingDayUsage(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "previous-day-usage@example.com", 10)
	group, err := client.Group.Create().SetName("previous-day-usage-group").Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetUserID(createdUser.ID).
		SetKey("sk-previous-day-usage").
		SetName("previous day usage").
		SetGroupID(group.ID).
		Save(ctx)
	require.NoError(t, err)
	account, err := client.Account.Create().
		SetName("previous-day-usage-account").
		SetPlatform("anthropic").
		SetType("api_key").
		Save(ctx)
	require.NoError(t, err)

	createAt := func(requestID string, amount float64, createdAt time.Time) {
		t.Helper()
		_, createErr := client.UsageLog.Create().
			SetUserID(createdUser.ID).
			SetAPIKeyID(apiKey.ID).
			SetAccountID(account.ID).
			SetRequestID(requestID).
			SetModel("claude-test").
			SetTotalCost(amount).
			SetActualCost(amount).
			SetCreatedAt(createdAt).
			Save(ctx)
		require.NoError(t, createErr)
	}
	beijing := time.FixedZone("CST", 8*60*60)
	createAt("before", 100, time.Date(2026, 7, 31, 23, 59, 59, 0, beijing))
	createAt("at-start", 1.25, time.Date(2026, 8, 1, 0, 0, 0, 0, beijing))
	createAt("at-end", 2.75, time.Date(2026, 8, 1, 23, 59, 59, 0, beijing))
	createAt("after", 200, time.Date(2026, 8, 2, 0, 0, 0, 0, beijing))
	fullyGiftFundedAt := time.Date(2026, 8, 1, 12, 0, 0, 0, beijing)
	createCheckinUsageWithThresholdExempt(t, ctx, client, createdUser.ID, "previous-day-gift-funded", 6, 6, &fullyGiftFundedAt)

	svc := NewCheckinService(client, nil, nil)
	usage, err := svc.previousBeijingDayUsageUSDWithClient(ctx, client, createdUser.ID, "2026-08-02")
	require.NoError(t, err)
	require.Equal(t, 10.0, usage, "previous-day rebate must use full actual cost, including gift-funded usage")
}

func TestCheckinService_Checkin_UsageRebate(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "usage-rebate@example.com", 10)
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled:                true,
		Tiers:                  []CheckinRewardTier{{Amount: 0.8, Probability: 100}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         8,
		TotalRewardCap:         10,
	})
	require.NoError(t, err)
	svc.rewardRoll = func() float64 { return 0 }
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 2, 9, 0, 0, 0, beijing) }
	createCheckinUsageAt(t, ctx, client, createdUser.ID, "usage-rebate-first", 50, time.Date(2026, 8, 1, 12, 0, 0, 0, beijing))

	status, err := svc.GetStatus(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 50.0, status.PreviousDayUsageAmount)
	require.Equal(t, 4.0, status.EstimatedUsageRebate)

	result, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.False(t, result.AlreadyCheckedIn)
	require.Equal(t, 50.0, result.PreviousDayUsageAmount)
	require.Equal(t, 0.8, result.BaseRewardAmount)
	require.Equal(t, 4.0, result.UsageRebateAmount)
	require.Zero(t, result.BonusRewardAmount)
	require.Equal(t, 4.8, result.TotalRewardAmount)
	require.Equal(t, 14.8, result.BalanceAfter)

	record, err := client.UserCheckin.Query().Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 50.0, record.PreviousDayUsageAmount)
	require.Equal(t, 4.0, record.UsageRebateAmount)
	require.Zero(t, record.RewardCapAdjustment)
}

func TestCheckinService_Checkin_IdempotentUsageSnapshot(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "usage-rebate-retry@example.com", 10)
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled:                true,
		Tiers:                  []CheckinRewardTier{{Amount: 0.5, Probability: 100}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         8,
		TotalRewardCap:         10,
	})
	require.NoError(t, err)
	svc.rewardRoll = func() float64 { return 0 }
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 8, 2, 9, 0, 0, 0, beijing) }
	createCheckinUsageAt(t, ctx, client, createdUser.ID, "usage-retry-first", 10, time.Date(2026, 8, 1, 10, 0, 0, 0, beijing))

	first, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 0.8, first.UsageRebateAmount)
	require.Equal(t, 1.3, first.TotalRewardAmount)

	createCheckinUsageAt(t, ctx, client, createdUser.ID, "usage-retry-late", 90, time.Date(2026, 8, 1, 11, 0, 0, 0, beijing))
	second, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.True(t, second.AlreadyCheckedIn)
	require.Equal(t, first.PreviousDayUsageAmount, second.PreviousDayUsageAmount)
	require.Equal(t, first.UsageRebateAmount, second.UsageRebateAmount)
	require.Equal(t, first.TotalRewardAmount, second.TotalRewardAmount)

	userAfter, err := client.User.Get(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 11.3, userAfter.Balance)
}

func TestCheckinServiceCustomRewardConfigAndStreakLotteryAttempts(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "streak@example.com", 10)
	settings := newCheckinSettingRepoStub()
	setCheckinConfig(t, settings, CheckinConfig{Enabled: true})

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled: true,
		Tiers: []CheckinRewardTier{
			{Amount: 1, Probability: 100, SortOrder: 1},
		},
		StreakEnabled: true,
		StreakRules: []CheckinStreakRule{
			{Day: 3, LotteryAttempts: 7},
		},
	})
	require.NoError(t, err)

	svc.now = func() time.Time {
		return time.Date(2026, 6, 1, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	}
	first, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 1, first.StreakDay)
	require.Equal(t, 1.0, first.RewardAmount)

	svc.now = func() time.Time {
		return time.Date(2026, 6, 2, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	}
	second, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 2, second.StreakDay)
	require.Equal(t, 1.0, second.RewardAmount)

	svc.now = func() time.Time {
		return time.Date(2026, 6, 3, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	}
	third, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 3, third.StreakDay)
	require.Equal(t, 1.0, third.BaseRewardAmount)
	require.Zero(t, third.BonusRewardAmount)
	require.Equal(t, 7, third.LotteryAttemptsReward)
	require.Equal(t, 1.0, third.RewardAmount)
	require.Equal(t, 13.0, third.BalanceAfter)
	require.Equal(t, 7, client.LotteryAttemptWallet.Query().OnlyX(ctx).Balance)
}

func TestCheckinServiceStreakMilestoneCreditsLotteryAttemptsWithoutBonusBalance(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "streak-lottery-attempts@example.com", 10)
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	beijing := time.FixedZone("CST", 8*60*60)
	svc.now = func() time.Time { return time.Date(2026, 9, 7, 10, 0, 0, 0, beijing) }
	svc.rewardRoll = func() float64 { return 0 }

	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled:            true,
		MinDailyUsageCount: 0,
		Tiers:              []CheckinRewardTier{{Amount: 2, Probability: 100}},
		StreakEnabled:      true,
		StreakRules:        []CheckinStreakRule{{Day: 7, LotteryAttempts: 3}},
	})
	require.NoError(t, err)
	createPreviousCheckinDays(t, ctx, client, user.ID, time.Date(2026, 9, 1, 9, 0, 0, 0, beijing), 6)

	first, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.False(t, first.AlreadyCheckedIn)
	require.Equal(t, 7, first.StreakDay)
	require.Equal(t, 2.0, first.TotalRewardAmount)
	require.Zero(t, first.BonusRewardAmount)
	require.Equal(t, 3, first.LotteryAttemptsReward)
	require.Equal(t, 12.0, first.BalanceAfter)

	walletBalance, err := lotteryAttemptBalance(ctx, client, user.ID)
	require.NoError(t, err)
	require.Equal(t, 3, walletBalance)
	require.Equal(t, 1, client.LotteryAttemptLedger.Query().CountX(ctx))

	second, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, second.AlreadyCheckedIn)
	require.Equal(t, 3, second.LotteryAttemptsReward)
	require.Equal(t, 3, client.LotteryAttemptWallet.Query().OnlyX(ctx).Balance)
	require.Equal(t, 1, client.LotteryAttemptLedger.Query().CountX(ctx))
}

func TestCheckinServiceLongStreakUsesFullHistory(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	createdUser := createCheckinTestUser(t, ctx, client, "long-streak@example.com", 10)
	settings := newCheckinSettingRepoStub()
	setCheckinConfig(t, settings, CheckinConfig{Enabled: true})

	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled: true,
		Tiers: []CheckinRewardTier{
			{Amount: 1, Probability: 100, SortOrder: 1},
		},
		StreakEnabled: true,
		StreakRules: []CheckinStreakRule{
			{Day: 120, LotteryAttempts: 50},
		},
	})
	require.NoError(t, err)

	loc := time.FixedZone("CST", 8*60*60)
	start := time.Date(2026, 1, 1, 9, 0, 0, 0, loc)
	for day := 0; day < 119; day++ {
		current := start.AddDate(0, 0, day)
		_, err := client.UserCheckin.Create().
			SetUserID(createdUser.ID).
			SetCheckinDate(current.Format("2006-01-02")).
			SetStreakDay(day + 1).
			SetBaseRewardAmount(1).
			SetBonusRewardAmount(0).
			SetTotalRewardAmount(1).
			SetRewardAmount(1).
			SetBalanceBefore(10 + float64(day)).
			SetBalanceAfter(11 + float64(day)).
			SetCreatedAt(current).
			Save(ctx)
		require.NoError(t, err)
	}

	svc.now = func() time.Time {
		return start.AddDate(0, 0, 119)
	}
	result, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 120, result.StreakDay)
	require.Equal(t, 120, result.LifetimeDays)
	require.Zero(t, result.BonusRewardAmount)
	require.Equal(t, 50, result.LotteryAttemptsReward)
	require.Equal(t, 1.0, result.RewardAmount)
}
