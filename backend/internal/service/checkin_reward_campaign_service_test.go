//go:build unit

package service

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
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
