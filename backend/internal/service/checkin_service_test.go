//go:build unit

package service

import (
	"context"
	"database/sql"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
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
