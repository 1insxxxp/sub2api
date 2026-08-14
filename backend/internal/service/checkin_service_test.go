//go:build unit

package service

import (
	"context"
	"database/sql"
	"math"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"

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

func newCheckinSettingRepoStub() *checkinSettingRepoStub {
	return &checkinSettingRepoStub{values: map[string]string{}}
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

func configureCheckinCampaignAwardBaseline(t *testing.T, ctx context.Context, svc *CheckinService) {
	t.Helper()
	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled:             true,
		MinTotalUsageUSD:    5,
		MinTotalRechargeUSD: 20,
		Tiers:               []CheckinRewardTier{{Amount: 1, Probability: 100}},
		StreakEnabled:       true,
		StreakRules: []CheckinStreakRule{
			{Day: 7, BonusAmount: 4},
			{Day: 14, BonusAmount: 8},
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
	require.Equal(t, 4.0, result.BonusRewardAmount, "streak remains a fixed direct-balance reward")
	require.Zero(t, result.RewardCapAdjustment)
	require.Equal(t, 13.0, result.TotalRewardAmount)
	require.Equal(t, 23.0, result.BalanceAfter)
	require.NotNil(t, result.RewardCampaignID)
	require.Equal(t, campaign.ID, *result.RewardCampaignID)
	require.Equal(t, "七夕基础奖励", result.RewardCampaignName)
	require.NotNil(t, result.NextStreakRule)
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
	require.Equal(t, normalizedTiers, history[0].RewardCampaignTiers)

	adminRecords, total, err := svc.ListRecords(ctx, 1, 10, CheckinListFilters{UserID: user.ID})
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, history[0].RewardCampaignID, adminRecords[0].RewardCampaignID)
	require.Equal(t, normalizedTiers, adminRecords[0].RewardCampaignTiers)

	*history[0].RewardCampaignID = 999
	history[0].RewardCampaignTiers[0].Amount = 999
	require.Equal(t, campaign.ID, *entity.RewardCampaignID, "DTO pointers must not alias entity storage")
	require.Equal(t, 5.0, entity.RewardCampaignTiersSnapshot[0].Amount, "DTO slices must not alias entity storage")
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
	queryCount := 0
	client.CheckinRewardCampaign.Intercept(dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
		return dbent.QuerierFunc(func(queryCtx context.Context, query dbent.Query) (dbent.Value, error) {
			value, err := next.Query(queryCtx, query)
			queryCount++
			if err == nil && queryCount == 1 {
				_, updateErr := client.CheckinRewardCampaign.UpdateOneID(campaign.ID).
					SetStatus(domain.CheckinRewardCampaignStatusDisabled).
					Save(queryCtx)
				require.NoError(t, updateErr)
			}
			return value, err
		})
	}))

	result, err := svc.Checkin(ctx, user.ID)
	require.NoError(t, err)
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

func TestCheckinConfig_NormalizesUsageRebateWithFixedStreak(t *testing.T) {
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
	require.Equal(t, 10.0, normalized.StreakRules[0].BonusAmount)
	require.Zero(t, normalized.StreakRules[0].BonusRatePercent)
}

func TestCheckinConfig_LegacyFixedStreakRemainsValid(t *testing.T) {
	cfg := CheckinConfig{
		Enabled:       true,
		Tiers:         []CheckinRewardTier{{Amount: 1, Probability: 100}},
		StreakEnabled: true,
		StreakRules:   []CheckinStreakRule{{Day: 7, BonusAmount: 4}},
	}

	normalized, err := normalizeCheckinConfig(cfg)
	require.NoError(t, err)
	require.False(t, normalized.UsageRebateEnabled)
	require.Equal(t, 4.0, normalized.StreakRules[0].BonusAmount)
	require.Zero(t, normalized.StreakRules[0].BonusRatePercent)
}

func TestCheckinConfig_UsageRebatePreservesFixedStreakBonus(t *testing.T) {
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
	require.Equal(t, 4.0, normalized.StreakRules[0].BonusAmount)
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
		"streak_rules":[{"day":7,"bonus_amount":10,"bonus_rate_percent":0}],
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
	require.Equal(t, 4.0, loaded.StreakRules[0].BonusAmount)
	require.Zero(t, loaded.StreakRules[0].BonusRatePercent)
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
		{name: "fixed streak is added after total cap", base: 3, usage: 100, streakBonus: 4, wantRebate: 7, wantStreak: 4, wantTotal: 14, wantCapAdjustment: 1},
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

func TestCalculateUsageLinkedCheckinReward_AddsFixedStreakAfterCap(t *testing.T) {
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
	require.Equal(t, 4.0, got.StreakBonus)
	require.Equal(t, 14.0, got.TotalReward)
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

	svc := NewCheckinService(client, nil, nil)
	usage, err := svc.previousBeijingDayUsageUSDWithClient(ctx, client, createdUser.ID, "2026-08-02")
	require.NoError(t, err)
	require.Equal(t, 4.0, usage)
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

func TestCheckinServiceCustomRewardConfigAndStreakBonus(t *testing.T) {
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
			{Day: 3, BonusAmount: 7},
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
	require.Equal(t, 7.0, third.BonusRewardAmount)
	require.Equal(t, 8.0, third.RewardAmount)
	require.Equal(t, 20.0, third.BalanceAfter)
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
			{Day: 120, BonusAmount: 50},
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
	require.Equal(t, 50.0, result.BonusRewardAmount)
	require.Equal(t, 51.0, result.RewardAmount)
}
