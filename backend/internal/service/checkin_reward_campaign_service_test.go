//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func checkinCampaignTestDate(t *testing.T, svc *CheckinService, date string) time.Time {
	t.Helper()
	parsed, err := time.ParseInLocation("2006-01-02", date, svc.beijingLocation)
	require.NoError(t, err)
	return parsed
}

func createCheckinRewardCampaignForResolverTest(
	t *testing.T,
	ctx context.Context,
	svc *CheckinService,
	client *dbent.Client,
	name, status, startDate, endDate string,
	tiers []domain.CheckinRewardTier,
) *dbent.CheckinRewardCampaign {
	t.Helper()
	campaign, err := client.CheckinRewardCampaign.Create().
		SetName(name).
		SetStatus(status).
		SetStartDate(checkinCampaignTestDate(t, svc, startDate)).
		SetEndDate(checkinCampaignTestDate(t, svc, endDate)).
		SetRewardTiers(tiers).
		Save(ctx)
	require.NoError(t, err)
	return campaign
}

func checkinCampaignBaselineConfig() *CheckinConfig {
	return &CheckinConfig{
		Enabled:             false,
		MinTotalUsageUSD:    12,
		MinTotalRechargeUSD: 34,
		Tiers: []CheckinRewardTier{
			{Amount: 1, Probability: 60, SortOrder: 1},
			{Amount: 5, Probability: 40, SortOrder: 2},
		},
		StreakEnabled: true,
		StreakRules: []CheckinStreakRule{
			{Day: 7, BonusAmount: 3},
			{Day: 14, BonusAmount: 8},
		},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 7.5,
		UsageRebateCap:         9,
		TotalRewardCap:         10,
	}
}

func TestResolveEffectiveCheckinConfigUsesBaselineWithoutCampaign(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	svc := NewCheckinService(client, nil, nil)
	baseline := checkinCampaignBaselineConfig()

	effective, err := svc.resolveEffectiveCheckinConfig(context.Background(), client, "2026-08-15", baseline)
	require.NoError(t, err)
	require.Nil(t, effective.Campaign)
	require.NotSame(t, baseline, effective.Config)
	require.Equal(t, 100.0, effective.Config.ProbabilityTotal)
	require.Equal(t, CheckinRewardPreview{MinReward: 1, MaxReward: 5, AverageReward: 2.6}, effective.Config.Preview)
	require.Equal(t, baseline.Enabled, effective.Config.Enabled)
	require.Equal(t, baseline.MinTotalUsageUSD, effective.Config.MinTotalUsageUSD)
	require.Equal(t, baseline.MinTotalRechargeUSD, effective.Config.MinTotalRechargeUSD)
	require.Equal(t, baseline.Tiers, effective.Config.Tiers)
	require.Equal(t, baseline.StreakEnabled, effective.Config.StreakEnabled)
	require.Equal(t, baseline.StreakRules, effective.Config.StreakRules)
	require.Equal(t, baseline.UsageRebateEnabled, effective.Config.UsageRebateEnabled)
	require.Equal(t, baseline.UsageRebateRatePercent, effective.Config.UsageRebateRatePercent)
	require.Equal(t, baseline.UsageRebateCap, effective.Config.UsageRebateCap)
	require.Equal(t, baseline.TotalRewardCap, effective.Config.TotalRewardCap)

	effective.Config.Tiers[0].Amount = 99
	effective.Config.StreakRules[0].BonusAmount = 99
	require.Equal(t, 1.0, baseline.Tiers[0].Amount)
	require.Equal(t, 3.0, baseline.StreakRules[0].BonusAmount)
}

func TestResolveEffectiveCheckinConfigReplacesOnlyTiers(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	svc := NewCheckinService(client, nil, nil)
	baseline := checkinCampaignBaselineConfig()
	campaignTiers := []domain.CheckinRewardTier{
		{Amount: 4, Probability: 25, SortOrder: 20},
		{Amount: 2, Probability: 75, SortOrder: 10},
	}
	created := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "暑期基础奖励", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-10", "2026-08-20", campaignTiers,
	)

	effective, err := svc.resolveEffectiveCheckinConfig(ctx, client, "2026-08-15", baseline)
	require.NoError(t, err)
	require.NotNil(t, effective.Campaign)
	require.Equal(t, created.ID, effective.Campaign.ID)
	require.Equal(t, "暑期基础奖励", effective.Campaign.Name)
	require.Equal(t, domain.CheckinRewardCampaignStatusEnabled, effective.Campaign.Status)
	require.Equal(t, "2026-08-10", effective.Campaign.StartDate)
	require.Equal(t, "2026-08-20", effective.Campaign.EndDate)

	wantTiers := []CheckinRewardTier{
		{Amount: 2, Probability: 75, SortOrder: 1},
		{Amount: 4, Probability: 25, SortOrder: 2},
	}
	require.Equal(t, wantTiers, effective.Config.Tiers)
	require.Equal(t, wantTiers, effective.Campaign.RewardTiers)
	require.Equal(t, 100.0, effective.Config.ProbabilityTotal)
	require.Equal(t, CheckinRewardPreview{MinReward: 2, MaxReward: 4, AverageReward: 2.5}, effective.Config.Preview)
	require.Equal(t, effective.Config.ProbabilityTotal, effective.Campaign.ProbabilityTotal)
	require.Equal(t, effective.Config.Preview, effective.Campaign.Preview)

	require.Equal(t, baseline.Enabled, effective.Config.Enabled)
	require.Equal(t, baseline.MinTotalUsageUSD, effective.Config.MinTotalUsageUSD)
	require.Equal(t, baseline.MinTotalRechargeUSD, effective.Config.MinTotalRechargeUSD)
	require.Equal(t, baseline.StreakEnabled, effective.Config.StreakEnabled)
	require.Equal(t, baseline.StreakRules, effective.Config.StreakRules)
	require.Equal(t, baseline.UsageRebateEnabled, effective.Config.UsageRebateEnabled)
	require.Equal(t, baseline.UsageRebateRatePercent, effective.Config.UsageRebateRatePercent)
	require.Equal(t, baseline.UsageRebateCap, effective.Config.UsageRebateCap)
	require.Equal(t, baseline.TotalRewardCap, effective.Config.TotalRewardCap)

	effective.Config.StreakRules[0].BonusAmount = 99
	require.Equal(t, 3.0, baseline.StreakRules[0].BonusAmount)
}

func TestResolveEffectiveCheckinConfigIncludesStartAndEndDates(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	svc := NewCheckinService(client, nil, nil)
	baseline := checkinCampaignBaselineConfig()
	createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "边界日期", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-10", "2026-08-12", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}},
	)

	for _, date := range []string{"2026-08-10", "2026-08-12"} {
		t.Run(date, func(t *testing.T) {
			effective, err := svc.resolveEffectiveCheckinConfig(ctx, client, date, baseline)
			require.NoError(t, err)
			require.NotNil(t, effective.Campaign)
			require.Equal(t, 2.0, effective.Config.Tiers[0].Amount)
		})
	}
}

func TestResolveEffectiveCheckinConfigIgnoresDraftAndDisabled(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	svc := NewCheckinService(client, nil, nil)
	baseline := checkinCampaignBaselineConfig()
	createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "草稿", domain.CheckinRewardCampaignStatusDraft,
		"2026-08-10", "2026-08-20", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}},
	)
	createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "停用", domain.CheckinRewardCampaignStatusDisabled,
		"2026-08-10", "2026-08-20", []domain.CheckinRewardTier{{Amount: 3, Probability: 100}},
	)

	effective, err := svc.resolveEffectiveCheckinConfig(ctx, client, "2026-08-15", baseline)
	require.NoError(t, err)
	require.Nil(t, effective.Campaign)
	require.Equal(t, baseline.Tiers, effective.Config.Tiers)
}

func TestResolveEffectiveCheckinConfigRejectsMultipleActiveRows(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	svc := NewCheckinService(client, nil, nil)
	baseline := checkinCampaignBaselineConfig()
	for index, amount := range []float64{2, 3} {
		createCheckinRewardCampaignForResolverTest(
			t, ctx, svc, client, "重叠-"+string(rune('A'+index)), domain.CheckinRewardCampaignStatusEnabled,
			"2026-08-10", "2026-08-20", []domain.CheckinRewardTier{{Amount: amount, Probability: 100}},
		)
	}

	_, err := svc.resolveEffectiveCheckinConfig(ctx, client, "2026-08-15", baseline)
	require.Error(t, err)
	require.Equal(t, http.StatusInternalServerError, infraerrors.Code(err))
	require.Equal(t, "CHECKIN_CAMPAIGN_DATA_INTEGRITY", infraerrors.Reason(err))
}

func TestResolveEffectiveCheckinConfigRejectsInvalidDate(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	svc := NewCheckinService(client, nil, nil)

	_, err := svc.resolveEffectiveCheckinConfig(context.Background(), client, "2026-8-15", checkinCampaignBaselineConfig())
	require.Error(t, err)
	require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
	require.Equal(t, "INVALID_CHECKIN_DATE", infraerrors.Reason(err))
}

func TestUpdateConfigRejectsConfigIncompatibleWithEnabledCampaign(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	settings := newCheckinSettingRepoStub()
	settings.values[SettingKeyCheckinEnabled] = "false"
	settings.values[SettingKeyCheckinMinTotalUsageUSD] = "99"
	settings.values[SettingKeyCheckinMinTotalRechargeUSD] = "88"
	settings.values[SettingKeyCheckinRewardConfig] = `{"tiers":[{"amount":1,"probability":100,"sort_order":1}]}`
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	svc.now = func() time.Time {
		return time.Date(2026, 8, 15, 12, 0, 0, 0, svc.beijingLocation)
	}
	created := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "高额活动", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-20", []domain.CheckinRewardTier{{Amount: 3, Probability: 100}},
	)
	before := make(map[string]string, len(settings.values))
	for key, value := range settings.values {
		before[key] = value
	}

	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled:                true,
		MinTotalUsageUSD:       12,
		MinTotalRechargeUSD:    34,
		Tiers:                  []CheckinRewardTier{{Amount: 1, Probability: 100}},
		StreakEnabled:          true,
		StreakRules:            []CheckinStreakRule{{Day: 7, BonusAmount: 3}},
		UsageRebateEnabled:     true,
		UsageRebateRatePercent: 8,
		UsageRebateCap:         5,
		TotalRewardCap:         2,
	})
	require.Error(t, err)
	require.Equal(t, http.StatusConflict, infraerrors.Code(err))
	require.Equal(t, "CHECKIN_CAMPAIGN_INCOMPATIBLE_WITH_CONFIG", infraerrors.Reason(err))
	appErr := infraerrors.FromError(err)
	require.Equal(t, map[string]string{
		"campaign_id":         strconv.FormatInt(created.ID, 10),
		"campaign_name":       "高额活动",
		"campaign_start_date": "2026-08-15",
		"campaign_end_date":   "2026-08-20",
	}, appErr.Metadata)
	require.Equal(t, before, settings.values, "incompatible updates must not persist any setting")
}

func TestNormalizeCheckinRewardTiersValidationAndInputImmutability(t *testing.T) {
	t.Run("stable sort and normalization do not mutate input", func(t *testing.T) {
		input := []CheckinRewardTier{
			{Amount: 2, Probability: 25, SortOrder: 4},
			{Amount: 1.23, Probability: 25, SortOrder: 2},
			{Amount: 3, Probability: 50, SortOrder: 2},
		}
		before := append([]CheckinRewardTier(nil), input...)

		normalized, err := normalizeCheckinRewardTiers(input)
		require.NoError(t, err)
		require.Equal(t, before, input)
		require.Equal(t, []CheckinRewardTier{
			{Amount: 1.23, Probability: 25, SortOrder: 1},
			{Amount: 3, Probability: 50, SortOrder: 2},
			{Amount: 2, Probability: 25, SortOrder: 3},
		}, normalized)
	})

	t.Run("duplicate cent amounts are rejected", func(t *testing.T) {
		_, err := normalizeCheckinRewardTiers([]CheckinRewardTier{
			{Amount: 1, Probability: 50},
			{Amount: 1.0, Probability: 50},
		})
		require.Equal(t, "CHECKIN_REWARD_CONFIG_DUPLICATE_AMOUNT", infraerrors.Reason(err))
	})

	t.Run("money precision over two decimals is rejected", func(t *testing.T) {
		_, err := normalizeCheckinRewardTiers([]CheckinRewardTier{{Amount: 1.234, Probability: 100}})
		require.Equal(t, "CHECKIN_REWARD_CONFIG_INVALID_AMOUNT", infraerrors.Reason(err))
	})

	t.Run("probability precision over two decimals is rejected", func(t *testing.T) {
		_, err := normalizeCheckinRewardTiers([]CheckinRewardTier{
			{Amount: 1, Probability: 33.333},
			{Amount: 2, Probability: 66.667},
		})
		require.Equal(t, "CHECKIN_REWARD_CONFIG_INVALID_PROBABILITY", infraerrors.Reason(err))
	})

	t.Run("probability must total exactly one hundred", func(t *testing.T) {
		_, err := normalizeCheckinRewardTiers([]CheckinRewardTier{{Amount: 1, Probability: 99.99}})
		require.Equal(t, "CHECKIN_REWARD_CONFIG_INVALID_TOTAL", infraerrors.Reason(err))
	})

	t.Run("twenty tiers are allowed and twenty one are rejected", func(t *testing.T) {
		tiers := make([]CheckinRewardTier, 20)
		for index := range tiers {
			tiers[index] = CheckinRewardTier{Amount: float64(index + 1), Probability: 5}
		}
		_, err := normalizeCheckinRewardTiers(tiers)
		require.NoError(t, err)
		tiers = append(tiers, CheckinRewardTier{Amount: 21, Probability: 1})
		_, err = normalizeCheckinRewardTiers(tiers)
		require.Equal(t, "CHECKIN_REWARD_CONFIG_TOO_MANY_TIERS", infraerrors.Reason(err))
	})

	for name, amount := range map[string]float64{"nan": math.NaN(), "positive infinity": math.Inf(1)} {
		t.Run(name+" amount is rejected", func(t *testing.T) {
			_, err := normalizeCheckinRewardTiers([]CheckinRewardTier{{Amount: amount, Probability: 100}})
			require.Equal(t, "CHECKIN_REWARD_CONFIG_INVALID_AMOUNT", infraerrors.Reason(err))
		})
	}
	for name, probability := range map[string]float64{"nan": math.NaN(), "positive infinity": math.Inf(1)} {
		t.Run(name+" probability is rejected", func(t *testing.T) {
			_, err := normalizeCheckinRewardTiers([]CheckinRewardTier{{Amount: 1, Probability: probability}})
			require.Equal(t, "CHECKIN_REWARD_CONFIG_INVALID_PROBABILITY", infraerrors.Reason(err))
		})
	}
}

func TestCheckinRewardCampaignMappingClonesAuditPointers(t *testing.T) {
	createdBy := int64(11)
	updatedBy := int64(22)
	entity := &dbent.CheckinRewardCampaign{
		ID:          1,
		Name:        "审计指针",
		Status:      domain.CheckinRewardCampaignStatusEnabled,
		StartDate:   time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC),
		EndDate:     time.Date(2026, 8, 12, 0, 0, 0, 0, time.UTC),
		RewardTiers: []domain.CheckinRewardTier{{Amount: 2, Probability: 100}},
		CreatedBy:   &createdBy,
		UpdatedBy:   &updatedBy,
	}

	mapped := checkinRewardCampaignFromEntity(entity, entity.RewardTiers, time.Date(2026, 8, 11, 0, 0, 0, 0, time.UTC), time.UTC)
	require.NotNil(t, mapped.CreatedBy)
	require.NotNil(t, mapped.UpdatedBy)
	*mapped.CreatedBy = 111
	*mapped.UpdatedBy = 222
	require.Equal(t, int64(11), *entity.CreatedBy)
	require.Equal(t, int64(22), *entity.UpdatedBy)
}

func TestDeriveCheckinRewardCampaignLifecycleUsesBeijingDate(t *testing.T) {
	beijing := time.FixedZone("Asia/Shanghai", 8*60*60)
	start := time.Date(2026, 8, 10, 0, 0, 0, 0, beijing)
	end := time.Date(2026, 8, 12, 0, 0, 0, 0, beijing)
	tests := []struct {
		name   string
		status string
		day    time.Time
		want   string
	}{
		{name: "draft", status: domain.CheckinRewardCampaignStatusDraft, day: start, want: CheckinRewardCampaignLifecycleDraft},
		{name: "disabled", status: domain.CheckinRewardCampaignStatusDisabled, day: start, want: CheckinRewardCampaignLifecycleDisabled},
		{name: "upcoming", status: domain.CheckinRewardCampaignStatusEnabled, day: start.AddDate(0, 0, -1), want: CheckinRewardCampaignLifecycleUpcoming},
		{name: "start is active", status: domain.CheckinRewardCampaignStatusEnabled, day: start, want: CheckinRewardCampaignLifecycleActive},
		{name: "end is active", status: domain.CheckinRewardCampaignStatusEnabled, day: end, want: CheckinRewardCampaignLifecycleActive},
		{name: "ended", status: domain.CheckinRewardCampaignStatusEnabled, day: end.AddDate(0, 0, 1), want: CheckinRewardCampaignLifecycleEnded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, deriveCheckinRewardCampaignLifecycle(tt.status, start, end, tt.day))
		})
	}
}

func TestResolveEffectiveCheckinConfigMarksEnabledCampaignActive(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	svc := NewCheckinService(client, nil, nil)
	createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "生效中", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-10", "2026-08-20", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}},
	)

	effective, err := svc.resolveEffectiveCheckinConfig(ctx, client, "2026-08-15", checkinCampaignBaselineConfig())
	require.NoError(t, err)
	require.Equal(t, CheckinRewardCampaignLifecycleActive, effective.Campaign.LifecycleStatus)
}

func TestUpdateConfigRejectsMalformedStoredCampaignAsDataIntegrity(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	settings := newCheckinSettingRepoStub()
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(settings)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, svc.beijingLocation) }
	createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, client, "损坏档位", domain.CheckinRewardCampaignStatusEnabled,
		"2026-08-15", "2026-08-20", []domain.CheckinRewardTier{{Amount: 3, Probability: 90}},
	)

	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled: true,
		Tiers:   []CheckinRewardTier{{Amount: 1, Probability: 100}},
	})
	require.Error(t, err)
	require.Equal(t, http.StatusInternalServerError, infraerrors.Code(err))
	require.Equal(t, "CHECKIN_CAMPAIGN_DATA_INTEGRITY", infraerrors.Reason(err))
}

func TestUpdateConfigChecksFutureIgnoresPastAndPersistsCompatibleCampaigns(t *testing.T) {
	t.Run("future enabled campaign is validated", func(t *testing.T) {
		client := newCheckinServiceTestClient(t)
		ctx := context.Background()
		settings := newCheckinSettingRepoStub()
		svc := NewCheckinService(client, nil, nil)
		svc.SetSettingRepository(settings)
		svc.now = func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, svc.beijingLocation) }
		createCheckinRewardCampaignForResolverTest(
			t, ctx, svc, client, "未来高额", domain.CheckinRewardCampaignStatusEnabled,
			"2026-08-20", "2026-08-22", []domain.CheckinRewardTier{{Amount: 3, Probability: 100}},
		)

		_, err := svc.UpdateConfig(ctx, CheckinConfig{
			Enabled:                true,
			Tiers:                  []CheckinRewardTier{{Amount: 1, Probability: 100}},
			UsageRebateEnabled:     true,
			UsageRebateRatePercent: 8,
			UsageRebateCap:         2,
			TotalRewardCap:         2,
		})
		require.Equal(t, "CHECKIN_CAMPAIGN_INCOMPATIBLE_WITH_CONFIG", infraerrors.Reason(err))
	})

	t.Run("ended enabled campaign is ignored", func(t *testing.T) {
		client := newCheckinServiceTestClient(t)
		ctx := context.Background()
		settings := newCheckinSettingRepoStub()
		svc := NewCheckinService(client, nil, nil)
		svc.SetSettingRepository(settings)
		svc.now = func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, svc.beijingLocation) }
		createCheckinRewardCampaignForResolverTest(
			t, ctx, svc, client, "已经结束", domain.CheckinRewardCampaignStatusEnabled,
			"2026-08-10", "2026-08-14", []domain.CheckinRewardTier{{Amount: 3, Probability: 100}},
		)

		_, err := svc.UpdateConfig(ctx, CheckinConfig{
			Enabled:                true,
			Tiers:                  []CheckinRewardTier{{Amount: 1, Probability: 100}},
			UsageRebateEnabled:     true,
			UsageRebateRatePercent: 8,
			UsageRebateCap:         2,
			TotalRewardCap:         2,
		})
		require.NoError(t, err)
		require.Equal(t, "true", settings.values[SettingKeyCheckinEnabled])
	})

	t.Run("compatible enabled campaign persists", func(t *testing.T) {
		client := newCheckinServiceTestClient(t)
		ctx := context.Background()
		settings := newCheckinSettingRepoStub()
		svc := NewCheckinService(client, nil, nil)
		svc.SetSettingRepository(settings)
		svc.now = func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, svc.beijingLocation) }
		createCheckinRewardCampaignForResolverTest(
			t, ctx, svc, client, "兼容活动", domain.CheckinRewardCampaignStatusEnabled,
			"2026-08-15", "2026-08-20", []domain.CheckinRewardTier{{Amount: 1.5, Probability: 100}},
		)

		updated, err := svc.UpdateConfig(ctx, CheckinConfig{
			Enabled:                true,
			Tiers:                  []CheckinRewardTier{{Amount: 1, Probability: 100}},
			UsageRebateEnabled:     true,
			UsageRebateRatePercent: 8,
			UsageRebateCap:         2,
			TotalRewardCap:         2,
		})
		require.NoError(t, err)
		require.Equal(t, 2.0, updated.TotalRewardCap)
		require.NotEmpty(t, settings.values[SettingKeyCheckinRewardConfig])
	})
}

type recordingCheckinSettingRepo struct {
	SettingRepository
	events *[]string
	fail   error
}

func (r *recordingCheckinSettingRepo) GetMultiple(ctx context.Context, keys []string) (map[string]string, error) {
	*r.events = append(*r.events, "settings_read")
	return r.SettingRepository.GetMultiple(ctx, keys)
}

func (r *recordingCheckinSettingRepo) SetMultiple(ctx context.Context, values map[string]string) error {
	*r.events = append(*r.events, "settings_write")
	if r.fail != nil {
		return r.fail
	}
	return r.SettingRepository.SetMultiple(ctx, values)
}

type checkinCampaignTxRepoStub struct {
	SettingRepository
	client *dbent.Client
	events []string
	fail   error
	mu     sync.Mutex
}

func (r *checkinCampaignTxRepoStub) WithCheckinCampaignConfigTx(
	ctx context.Context,
	fn func(client *dbent.Client, repo SettingRepository) error,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, "tx_begin", "advisory_lock")
	txRepo := &recordingCheckinSettingRepo{SettingRepository: r.SettingRepository, events: &r.events, fail: r.fail}
	if err := fn(r.client, txRepo); err != nil {
		r.events = append(r.events, "tx_rollback")
		return err
	}
	r.events = append(r.events, "tx_commit")
	return nil
}

func newPostgresDialectCheckinClient(t *testing.T) *dbent.Client {
	t.Helper()
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestUpdateConfigUsesSharedTransactionForCampaignQueryAndSettingWrite(t *testing.T) {
	txClient := newCheckinServiceTestClient(t)
	ctx := context.Background()
	baseRepo := newCheckinSettingRepoStub()
	txRepo := &checkinCampaignTxRepoStub{SettingRepository: baseRepo, client: txClient}
	txClient.CheckinRewardCampaign.Intercept(dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
		return dbent.QuerierFunc(func(ctx context.Context, query dbent.Query) (dbent.Value, error) {
			txRepo.events = append(txRepo.events, "campaign_query")
			return next.Query(ctx, query)
		})
	}))
	svc := NewCheckinService(newPostgresDialectCheckinClient(t), nil, nil)
	svc.SetSettingRepository(txRepo)

	_, err := svc.UpdateConfig(ctx, CheckinConfig{
		Enabled: true,
		Tiers:   []CheckinRewardTier{{Amount: 1, Probability: 100}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"tx_begin", "advisory_lock", "campaign_query", "settings_write", "tx_commit"}, txRepo.events)
}

func TestCheckinRewardCampaignEnableUsesAdvisoryBaselineRowLockOrder(t *testing.T) {
	txClient := newCheckinServiceTestClient(t)
	ctx := context.Background()
	baseRepo := newCheckinSettingRepoStub()
	txRepo := &checkinCampaignTxRepoStub{SettingRepository: baseRepo, client: txClient}
	svc := NewCheckinService(newPostgresDialectCheckinClient(t), nil, nil)
	svc.SetSettingRepository(txRepo)
	svc.now = func() time.Time { return time.Date(2026, 8, 15, 12, 0, 0, 0, svc.beijingLocation) }
	draft := createCheckinRewardCampaignForResolverTest(
		t, ctx, svc, txClient, "锁顺序", domain.CheckinRewardCampaignStatusDraft,
		"2026-08-20", "2026-08-21", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}},
	)
	txClient.CheckinRewardCampaign.Intercept(dbent.InterceptFunc(func(next dbent.Querier) dbent.Querier {
		return dbent.QuerierFunc(func(ctx context.Context, query dbent.Query) (dbent.Value, error) {
			txRepo.events = append(txRepo.events, "campaign_query")
			return next.Query(ctx, query)
		})
	}))

	enabled, err := svc.EnableRewardCampaign(ctx, draft.ID, 77)
	require.NoError(t, err)
	require.Equal(t, domain.CheckinRewardCampaignStatusEnabled, enabled.Status)
	require.Equal(t, int64(77), *enabled.UpdatedBy)
	require.Equal(t, []string{
		"tx_begin", "advisory_lock", "settings_read", "campaign_query", "campaign_query", "tx_commit",
	}, txRepo.events)
}

func TestUpdateConfigPostgresFailsClosedWithoutSharedTransactionRepository(t *testing.T) {
	svc := NewCheckinService(newPostgresDialectCheckinClient(t), nil, nil)
	svc.SetSettingRepository(newCheckinSettingRepoStub())

	_, err := svc.UpdateConfig(context.Background(), CheckinConfig{
		Enabled: true,
		Tiers:   []CheckinRewardTier{{Amount: 1, Probability: 100}},
	})
	require.Error(t, err)
	require.Equal(t, "CHECKIN_CAMPAIGN_TRANSACTION_UNAVAILABLE", infraerrors.Reason(err))
}

func TestUpdateConfigSharedTransactionPropagatesRepositoryFailure(t *testing.T) {
	txClient := newCheckinServiceTestClient(t)
	sentinel := errors.New("setting write failed")
	baseRepo := newCheckinSettingRepoStub()
	txRepo := &checkinCampaignTxRepoStub{SettingRepository: baseRepo, client: txClient, fail: sentinel}
	svc := NewCheckinService(newPostgresDialectCheckinClient(t), nil, nil)
	svc.SetSettingRepository(txRepo)

	_, err := svc.UpdateConfig(context.Background(), CheckinConfig{
		Enabled: true,
		Tiers:   []CheckinRewardTier{{Amount: 1, Probability: 100}},
	})
	require.ErrorIs(t, err, sentinel)
	require.Equal(t, []string{"tx_begin", "advisory_lock", "settings_write", "tx_rollback"}, txRepo.events)
	require.Empty(t, baseRepo.values)
}

func TestCheckinCampaignConfigTransactionSQLiteFallbackSerializesCallbacks(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(newCheckinSettingRepoStub())
	firstEntered := make(chan struct{})
	releaseFirst := make(chan struct{})
	secondEntered := make(chan struct{})
	firstResult := make(chan error, 1)
	secondResult := make(chan error, 1)

	go func() {
		firstResult <- svc.withCheckinCampaignConfigTx(context.Background(), func(_ *dbent.Client, _ SettingRepository) error {
			close(firstEntered)
			<-releaseFirst
			return nil
		})
	}()
	<-firstEntered
	go func() {
		secondResult <- svc.withCheckinCampaignConfigTx(context.Background(), func(_ *dbent.Client, _ SettingRepository) error {
			close(secondEntered)
			return nil
		})
	}()

	select {
	case <-secondEntered:
		t.Fatal("second SQLite callback entered before the service mutex was released")
	case <-time.After(50 * time.Millisecond):
	}
	close(releaseFirst)
	require.NoError(t, <-firstResult)
	select {
	case <-secondEntered:
	case <-time.After(time.Second):
		t.Fatal("second SQLite callback did not enter after the service mutex was released")
	}
	require.NoError(t, <-secondResult)
}

func TestGetCheckinConfigFromRepositoryUsesProvidedTransactionRepository(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	outerRepo := newCheckinSettingRepoStub()
	outerRepo.values[SettingKeyCheckinEnabled] = "false"
	txRepo := newCheckinSettingRepoStub()
	txRepo.values[SettingKeyCheckinEnabled] = "true"
	txRepo.values[SettingKeyCheckinMinTotalUsageUSD] = "12"
	txRepo.values[SettingKeyCheckinRewardConfig] = `{"tiers":[{"amount":2,"probability":100,"sort_order":1}]}`
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(outerRepo)

	config, err := svc.getCheckinConfigFromRepository(context.Background(), txRepo)
	require.NoError(t, err)
	require.True(t, config.Enabled)
	require.Equal(t, 12.0, config.MinTotalUsageUSD)
	require.Equal(t, 2.0, config.Tiers[0].Amount)
}

func newCheckinRewardCampaignLifecycleService(t *testing.T, now string) (*CheckinService, *dbent.Client) {
	t.Helper()
	client := newCheckinServiceTestClient(t)
	svc := NewCheckinService(client, nil, nil)
	svc.SetSettingRepository(newCheckinSettingRepoStub())
	parsed := checkinCampaignTestDate(t, svc, now)
	svc.now = func() time.Time { return parsed.Add(12 * time.Hour) }
	return svc, client
}

func validCheckinRewardCampaignInput(name, startDate, endDate string) CreateCheckinRewardCampaignInput {
	return CreateCheckinRewardCampaignInput{
		Name: name, StartDate: startDate, EndDate: endDate, AdminID: 41,
		RewardTiers: []CheckinRewardTier{
			{Amount: 5, Probability: 25, SortOrder: 9},
			{Amount: 2, Probability: 75, SortOrder: 3},
		},
	}
}

func TestCheckinRewardCampaignCreateCopiesNormalizedTiers(t *testing.T) {
	svc, _ := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	input := validCheckinRewardCampaignInput("  周末加奖  ", "2026-08-20", "2026-08-22")

	created, err := svc.CreateRewardCampaign(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, "周末加奖", created.Name)
	require.Equal(t, domain.CheckinRewardCampaignStatusDraft, created.Status)
	require.Equal(t, CheckinRewardCampaignLifecycleDraft, created.LifecycleStatus)
	require.Equal(t, []CheckinRewardTier{
		{Amount: 2, Probability: 75, SortOrder: 1},
		{Amount: 5, Probability: 25, SortOrder: 2},
	}, created.RewardTiers)
	require.Equal(t, int64(41), *created.CreatedBy)
	require.Equal(t, int64(41), *created.UpdatedBy)

	input.RewardTiers[0].Amount = 99
	require.Equal(t, 5.0, created.RewardTiers[1].Amount)
}

func TestCheckinRewardCampaignUpdateAllowsDraftOnly(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	draft, err := svc.CreateRewardCampaign(context.Background(), validCheckinRewardCampaignInput("草稿", "2026-08-20", "2026-08-22"))
	require.NoError(t, err)

	updated, err := svc.UpdateRewardCampaign(context.Background(), draft.ID, UpdateCheckinRewardCampaignInput{
		Name: " 新名称 ", StartDate: "2026-08-21", EndDate: "2026-08-23", AdminID: 42,
		RewardTiers: []CheckinRewardTier{{Amount: 3, Probability: 100}},
	})
	require.NoError(t, err)
	require.Equal(t, "新名称", updated.Name)
	require.Equal(t, int64(42), *updated.UpdatedBy)

	_, err = client.CheckinRewardCampaign.UpdateOneID(draft.ID).SetStatus(domain.CheckinRewardCampaignStatusEnabled).Save(context.Background())
	require.NoError(t, err)
	_, err = svc.UpdateRewardCampaign(context.Background(), draft.ID, UpdateCheckinRewardCampaignInput{
		Name: "不能修改", StartDate: "2026-08-21", EndDate: "2026-08-23",
		RewardTiers: []CheckinRewardTier{{Amount: 3, Probability: 100}},
	})
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_INVALID_STATE_TRANSITION", infraerrors.Reason(err))
}

func TestCheckinRewardCampaignEnableRejectsOverlap(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	conflict := createCheckinRewardCampaignForResolverTest(t, context.Background(), svc, client, "已启用活动", domain.CheckinRewardCampaignStatusEnabled, "2026-08-20", "2026-08-25", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}})
	draft, err := svc.CreateRewardCampaign(context.Background(), validCheckinRewardCampaignInput("冲突草稿", "2026-08-25", "2026-08-28"))
	require.NoError(t, err)

	_, err = svc.EnableRewardCampaign(context.Background(), draft.ID, 99)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_OVERLAP", infraerrors.Reason(err))
	require.Equal(t, map[string]string{
		"conflict_campaign_id":   strconv.FormatInt(conflict.ID, 10),
		"conflict_campaign_name": "已启用活动",
		"conflict_start_date":    "2026-08-20",
		"conflict_end_date":      "2026-08-25",
	}, infraerrors.FromError(err).Metadata)
}

func TestCheckinRewardCampaignEnableRejectsInvalidProbability(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	draft := createCheckinRewardCampaignForResolverTest(t, context.Background(), svc, client, "损坏草稿", domain.CheckinRewardCampaignStatusDraft, "2026-08-20", "2026-08-21", []domain.CheckinRewardTier{{Amount: 2, Probability: 90}})

	_, err := svc.EnableRewardCampaign(context.Background(), draft.ID, 99)
	require.Equal(t, "CHECKIN_REWARD_CONFIG_INVALID_TOTAL", infraerrors.Reason(err))
}

func TestCheckinRewardCampaignEnableRejectsBaselineIncompatibility(t *testing.T) {
	svc, _ := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	repo := newCheckinSettingRepoStub()
	repo.values[SettingKeyCheckinRewardConfig] = `{"tiers":[{"amount":1,"probability":100}],"usage_rebate_enabled":true,"usage_rebate_rate_percent":5,"usage_rebate_cap":1,"total_reward_cap":2}`
	svc.SetSettingRepository(repo)
	input := validCheckinRewardCampaignInput("高额草稿", "2026-08-20", "2026-08-21")
	input.RewardTiers = []CheckinRewardTier{{Amount: 3, Probability: 100}}
	draft, err := svc.CreateRewardCampaign(context.Background(), input)
	require.NoError(t, err)

	_, err = svc.EnableRewardCampaign(context.Background(), draft.ID, 99)
	require.Equal(t, "CHECKIN_CAMPAIGN_INCOMPATIBLE_WITH_CONFIG", infraerrors.Reason(err))
}

func TestCheckinRewardCampaignDisableFallsBackImmediately(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	enabled := createCheckinRewardCampaignForResolverTest(t, context.Background(), svc, client, "当前活动", domain.CheckinRewardCampaignStatusEnabled, "2026-08-15", "2026-08-20", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}})

	disabled, err := svc.DisableRewardCampaign(context.Background(), enabled.ID, 91)
	require.NoError(t, err)
	require.Equal(t, domain.CheckinRewardCampaignStatusDisabled, disabled.Status)
	require.Equal(t, CheckinRewardCampaignLifecycleDisabled, disabled.LifecycleStatus)
	require.Equal(t, int64(91), *disabled.UpdatedBy)
	effective, err := svc.resolveEffectiveCheckinConfig(context.Background(), client, "2026-08-15", checkinCampaignBaselineConfig())
	require.NoError(t, err)
	require.Nil(t, effective.Campaign)
}

func TestCheckinRewardCampaignCopyCreatesDraft(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	source := createCheckinRewardCampaignForResolverTest(t, context.Background(), svc, client, "来源", domain.CheckinRewardCampaignStatusEnabled, "2026-08-20", "2026-08-22", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}})

	copied, err := svc.CopyRewardCampaign(context.Background(), source.ID, " 来源副本 ", 88)
	require.NoError(t, err)
	require.NotEqual(t, source.ID, copied.ID)
	require.Equal(t, domain.CheckinRewardCampaignStatusDraft, copied.Status)
	require.Equal(t, "来源副本", copied.Name)
	require.Equal(t, "2026-08-20", copied.StartDate)
	require.Equal(t, []CheckinRewardTier{{Amount: 2, Probability: 100, SortOrder: 1}}, copied.RewardTiers)
	require.Equal(t, int64(88), *copied.CreatedBy)
	require.Equal(t, int64(88), *copied.UpdatedBy)
}

func TestCheckinRewardCampaignDeleteAllowsUnreferencedDraftOnly(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	draft, err := svc.CreateRewardCampaign(context.Background(), validCheckinRewardCampaignInput("可删除", "2026-08-20", "2026-08-21"))
	require.NoError(t, err)
	require.NoError(t, svc.DeleteRewardCampaign(context.Background(), draft.ID))
	_, err = svc.GetRewardCampaign(context.Background(), draft.ID)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_NOT_FOUND", infraerrors.Reason(err))

	referenced := createCheckinRewardCampaignForResolverTest(t, context.Background(), svc, client, "被引用", domain.CheckinRewardCampaignStatusDraft, "2026-08-20", "2026-08-21", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}})
	user, err := client.User.Create().SetEmail("campaign-delete@test.local").SetPasswordHash("x").SetUsername("campaign-delete").Save(context.Background())
	require.NoError(t, err)
	_, err = client.UserCheckin.Create().SetUserID(user.ID).SetCheckinDate("2026-08-15").SetRewardCampaignID(referenced.ID).Save(context.Background())
	require.NoError(t, err)
	err = svc.DeleteRewardCampaign(context.Background(), referenced.ID)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_REFERENCED", infraerrors.Reason(err))

	enabled := createCheckinRewardCampaignForResolverTest(t, context.Background(), svc, client, "启用不可删除", domain.CheckinRewardCampaignStatusEnabled, "2026-08-20", "2026-08-21", []domain.CheckinRewardTier{{Amount: 2, Probability: 100}})
	err = svc.DeleteRewardCampaign(context.Background(), enabled.ID)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_INVALID_STATE_TRANSITION", infraerrors.Reason(err))
}

func TestCheckinRewardCampaignListDerivesLifecycle(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	ctx := context.Background()
	createCheckinRewardCampaignForResolverTest(t, ctx, svc, client, "草稿", domain.CheckinRewardCampaignStatusDraft, "2026-08-30", "2026-08-31", []domain.CheckinRewardTier{{Amount: 1, Probability: 100}})
	createCheckinRewardCampaignForResolverTest(t, ctx, svc, client, "未来", domain.CheckinRewardCampaignStatusEnabled, "2026-08-20", "2026-08-21", []domain.CheckinRewardTier{{Amount: 1, Probability: 100}})
	createCheckinRewardCampaignForResolverTest(t, ctx, svc, client, "当前", domain.CheckinRewardCampaignStatusEnabled, "2026-08-15", "2026-08-16", []domain.CheckinRewardTier{{Amount: 1, Probability: 100}})
	createCheckinRewardCampaignForResolverTest(t, ctx, svc, client, "结束", domain.CheckinRewardCampaignStatusEnabled, "2026-08-10", "2026-08-14", []domain.CheckinRewardTier{{Amount: 1, Probability: 100}})
	createCheckinRewardCampaignForResolverTest(t, ctx, svc, client, "停用", domain.CheckinRewardCampaignStatusDisabled, "2026-08-25", "2026-08-26", []domain.CheckinRewardTier{{Amount: 1, Probability: 100}})

	all, err := svc.ListRewardCampaigns(ctx, "all")
	require.NoError(t, err)
	require.Len(t, all, 5)
	require.Equal(t, []string{"草稿", "停用", "未来", "当前", "结束"}, []string{all[0].Name, all[1].Name, all[2].Name, all[3].Name, all[4].Name})
	unfiltered, err := svc.ListRewardCampaigns(ctx, "")
	require.NoError(t, err)
	require.Equal(t, all, unfiltered)
	for lifecycle := range map[string]struct{}{"draft": {}, "upcoming": {}, "active": {}, "ended": {}, "disabled": {}} {
		rows, filterErr := svc.ListRewardCampaigns(ctx, lifecycle)
		require.NoError(t, filterErr)
		require.Len(t, rows, 1)
		require.Equal(t, lifecycle, rows[0].LifecycleStatus)
	}
	_, err = svc.ListRewardCampaigns(ctx, "unknown")
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_INVALID_LIFECYCLE_FILTER", infraerrors.Reason(err))
}

func TestCheckinRewardCampaignValidationAndReadProtection(t *testing.T) {
	svc, client := newCheckinRewardCampaignLifecycleService(t, "2026-08-15")
	ctx := context.Background()
	for _, input := range []CreateCheckinRewardCampaignInput{
		validCheckinRewardCampaignInput("   ", "2026-08-20", "2026-08-21"),
		validCheckinRewardCampaignInput(strings.Repeat("长", 121), "2026-08-20", "2026-08-21"),
		validCheckinRewardCampaignInput("bad date", "2026-8-20", "2026-08-21"),
		validCheckinRewardCampaignInput("bad range", "2026-08-22", "2026-08-21"),
	} {
		_, err := svc.CreateRewardCampaign(ctx, input)
		require.Error(t, err)
	}
	_, err := svc.GetRewardCampaign(ctx, 99999)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_NOT_FOUND", infraerrors.Reason(err))

	ended := createCheckinRewardCampaignForResolverTest(t, ctx, svc, client, "已结束", domain.CheckinRewardCampaignStatusEnabled, "2026-08-10", "2026-08-14", []domain.CheckinRewardTier{{Amount: 1, Probability: 100}})
	_, err = svc.DisableRewardCampaign(ctx, ended.ID, 7)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_HISTORY_PROTECTED", infraerrors.Reason(err))
	_, err = svc.EnableRewardCampaign(ctx, ended.ID, 7)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_INVALID_STATE_TRANSITION", infraerrors.Reason(err))

	endedDraft := createCheckinRewardCampaignForResolverTest(t, ctx, svc, client, "已过期草稿", domain.CheckinRewardCampaignStatusDraft, "2026-08-10", "2026-08-14", []domain.CheckinRewardTier{{Amount: 1, Probability: 100}})
	_, err = svc.EnableRewardCampaign(ctx, endedDraft.ID, 7)
	require.Equal(t, "CHECKIN_REWARD_CAMPAIGN_INVALID_STATE_TRANSITION", infraerrors.Reason(err))
}
