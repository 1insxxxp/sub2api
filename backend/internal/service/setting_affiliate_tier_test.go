//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestAffiliateTierConfig_DefaultsAndResolution(t *testing.T) {
	cfg := DefaultAffiliateTierConfig()
	require.Equal(t, AffiliateTierConfig{
		QualificationAmount: 50,
		StandardRate:        8,
		BronzeInvitees:      3,
		BronzeRate:          10,
		SilverInvitees:      10,
		SilverRate:          12,
		GoldInvitees:        30,
		GoldRate:            15,
	}, cfg)

	tests := []struct {
		qualified    int
		level        AffiliateTier
		rate         float64
		nextInvitees int
	}{
		{qualified: 0, level: AffiliateTierStandard, rate: 8, nextInvitees: 3},
		{qualified: 2, level: AffiliateTierStandard, rate: 8, nextInvitees: 3},
		{qualified: 3, level: AffiliateTierBronze, rate: 10, nextInvitees: 10},
		{qualified: 9, level: AffiliateTierBronze, rate: 10, nextInvitees: 10},
		{qualified: 10, level: AffiliateTierSilver, rate: 12, nextInvitees: 30},
		{qualified: 29, level: AffiliateTierSilver, rate: 12, nextInvitees: 30},
		{qualified: 30, level: AffiliateTierGold, rate: 15, nextInvitees: 0},
	}
	for _, tt := range tests {
		level, rate, nextInvitees := cfg.Resolve(tt.qualified)
		require.Equal(t, tt.level, level)
		require.Equal(t, tt.rate, rate)
		require.Equal(t, tt.nextInvitees, nextInvitees)
	}
}

func TestSettingService_ParseSettings_AffiliateTierDefaultsAndDirtyValueFallback(t *testing.T) {
	svc := NewSettingService(&settingUpdateRepoStub{}, &config.Config{})

	got := svc.parseSettings(map[string]string{
		SettingKeyAffiliateQualificationAmount: "not-a-number",
		SettingKeyAffiliateBronzeInvitees:      "4",
		SettingKeyAffiliateBronzeRate:          "11",
		SettingKeyAffiliateSilverInvitees:      "12",
		SettingKeyAffiliateSilverRate:          "13",
		SettingKeyAffiliateGoldInvitees:        "35",
		SettingKeyAffiliateGoldRate:            "16",
	})

	require.Equal(t, AffiliateRebateRateDefault, got.AffiliateRebateRate)
	require.Equal(t, AffiliateQualificationAmountDefault, got.AffiliateQualificationAmount)
	require.Equal(t, 4, got.AffiliateBronzeInvitees)
	require.Equal(t, 11.0, got.AffiliateBronzeRate)
	require.Equal(t, 12, got.AffiliateSilverInvitees)
	require.Equal(t, 13.0, got.AffiliateSilverRate)
	require.Equal(t, 35, got.AffiliateGoldInvitees)
	require.Equal(t, 16.0, got.AffiliateGoldRate)
}

func TestSettingService_AffiliateTierLegacyBaseNormalizesRatesAndHelpersAgree(t *testing.T) {
	repo := &affiliateTierReconcileRepoStub{values: map[string]string{
		SettingKeyAffiliateRebateRate: "20",
	}}
	svc := NewSettingService(repo, &config.Config{})

	tier, err := svc.GetAffiliateTierConfigStrict(context.Background())
	require.NoError(t, err)
	require.Equal(t, AffiliateTierConfig{
		QualificationAmount: 50,
		StandardRate:        20,
		BronzeInvitees:      3,
		BronzeRate:          20,
		SilverInvitees:      10,
		SilverRate:          20,
		GoldInvitees:        30,
		GoldRate:            20,
	}, tier)
	require.Equal(t, tier, svc.GetAffiliateTierConfig(context.Background()))
	require.Equal(t, tier.StandardRate, svc.GetAffiliateRebateRatePercent(context.Background()))
}

func TestSettingService_AffiliateTierConfiguredAndDirtyHelpersAgree(t *testing.T) {
	tests := map[string]map[string]string{
		"configured": {
			SettingKeyAffiliateRebateRate:          "9",
			SettingKeyAffiliateQualificationAmount: "60",
			SettingKeyAffiliateBronzeInvitees:      "4",
			SettingKeyAffiliateBronzeRate:          "11",
			SettingKeyAffiliateSilverInvitees:      "12",
			SettingKeyAffiliateSilverRate:          "13",
			SettingKeyAffiliateGoldInvitees:        "40",
			SettingKeyAffiliateGoldRate:            "16",
		},
		"dirty": {
			SettingKeyAffiliateRebateRate:          "not-a-rate",
			SettingKeyAffiliateQualificationAmount: "not-an-amount",
			SettingKeyAffiliateBronzeInvitees:      "bad",
		},
	}

	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			svc := NewSettingService(&affiliateTierReconcileRepoStub{values: values}, &config.Config{})
			tier := svc.GetAffiliateTierConfig(context.Background())
			require.Equal(t, tier.StandardRate, svc.GetAffiliateRebateRatePercent(context.Background()))
		})
	}
}

func TestSettingService_GetAffiliateTierConfigStrictReturnsRepositoryError(t *testing.T) {
	wantErr := errors.New("settings unavailable")
	svc := NewSettingService(&affiliateTierReadErrorRepoStub{err: wantErr}, &config.Config{})

	_, err := svc.GetAffiliateTierConfigStrict(context.Background())

	require.ErrorIs(t, err, wantErr)
	require.Equal(t, DefaultAffiliateTierConfig(), svc.GetAffiliateTierConfig(context.Background()))
	require.Equal(t, AffiliateRebateRateDefault, svc.GetAffiliateRebateRatePercent(context.Background()))
}

func TestSettingService_GetAffiliateTierConfigStrictRejectsDirtyStoredConfig(t *testing.T) {
	svc := NewSettingService(&affiliateTierReconcileRepoStub{values: map[string]string{
		SettingKeyAffiliateRebateRate: "not-a-rate",
	}}, &config.Config{})

	_, err := svc.GetAffiliateTierConfigStrict(context.Background())

	require.Error(t, err)
	require.Contains(t, err.Error(), SettingKeyAffiliateRebateRate)
}

func TestSettingService_UpdateSettings_AffiliateTierPersistsCompleteConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})

	err := svc.UpdateSettings(context.Background(), &SystemSettings{
		AffiliateRebateRate:          9,
		AffiliateQualificationAmount: 75,
		AffiliateBronzeInvitees:      5,
		AffiliateBronzeRate:          11,
		AffiliateSilverInvitees:      15,
		AffiliateSilverRate:          14,
		AffiliateGoldInvitees:        40,
		AffiliateGoldRate:            18,
	})

	require.NoError(t, err)
	require.Equal(t, "9.00000000", repo.updates[SettingKeyAffiliateRebateRate])
	require.Equal(t, "75.00000000", repo.updates[SettingKeyAffiliateQualificationAmount])
	require.Equal(t, "5", repo.updates[SettingKeyAffiliateBronzeInvitees])
	require.Equal(t, "11.00000000", repo.updates[SettingKeyAffiliateBronzeRate])
	require.Equal(t, "15", repo.updates[SettingKeyAffiliateSilverInvitees])
	require.Equal(t, "14.00000000", repo.updates[SettingKeyAffiliateSilverRate])
	require.Equal(t, "40", repo.updates[SettingKeyAffiliateGoldInvitees])
	require.Equal(t, "18.00000000", repo.updates[SettingKeyAffiliateGoldRate])
	require.Equal(t, "true", repo.updates[SettingKeyAffiliateTierReconcileRequired])
	generation, err := strconv.ParseInt(repo.updates[SettingKeyAffiliateTierReconcileGeneration], 10, 64)
	require.NoError(t, err)
	require.Positive(t, generation)
}

func TestSettingService_UpdateSettings_AffiliateTierAdvancesPersistentReconcileGeneration(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	settings := settingsWithAffiliateTier(DefaultAffiliateTierConfig())

	require.NoError(t, svc.UpdateSettings(context.Background(), settings))
	first := repo.updates[SettingKeyAffiliateTierReconcileGeneration]
	require.NoError(t, svc.UpdateSettings(context.Background(), settings))
	second := repo.updates[SettingKeyAffiliateTierReconcileGeneration]

	require.NotEqual(t, first, second)
	require.Equal(t, "true", repo.updates[SettingKeyAffiliateTierReconcileRequired])
}

func TestSettingService_UpdateSettings_AffiliateTierMarkerWriteFailureIsReturned(t *testing.T) {
	wantErr := errors.New("settings transaction rolled back")
	repo := &settingUpdateRepoStub{setErr: wantErr}
	svc := NewSettingService(repo, &config.Config{})
	updated := false
	svc.onUpdate = func() { updated = true }

	err := svc.UpdateSettings(context.Background(), settingsWithAffiliateTier(DefaultAffiliateTierConfig()))

	require.ErrorIs(t, err, wantErr)
	require.False(t, updated)
	require.Nil(t, repo.updates)
}

func TestSettingService_UpdateSettings_AffiliateTierRejectsInvalidConfigWithoutWriting(t *testing.T) {
	valid := AffiliateTierConfig{
		QualificationAmount: 50,
		StandardRate:        8,
		BronzeInvitees:      3,
		BronzeRate:          10,
		SilverInvitees:      10,
		SilverRate:          12,
		GoldInvitees:        30,
		GoldRate:            15,
	}
	tests := map[string]AffiliateTierConfig{
		"zero qualification":     withQualificationAmount(valid, 0),
		"infinite qualification": withQualificationAmount(valid, math.Inf(1)),
		"non-positive threshold": withBronzeInvitees(valid, 0),
		"equal thresholds":       withSilverInvitees(valid, 3),
		"decreasing thresholds":  withGoldInvitees(valid, 9),
		"negative rate":          withStandardRate(valid, -1),
		"rate above one hundred": withGoldRate(valid, 101),
		"non-finite rate":        withSilverRate(valid, math.NaN()),
		"decreasing tier rates":  withSilverRate(valid, 9),
	}

	for name, tier := range tests {
		t.Run(name, func(t *testing.T) {
			repo := &settingUpdateRepoStub{}
			svc := NewSettingService(repo, &config.Config{})
			err := svc.UpdateSettings(context.Background(), settingsWithAffiliateTier(tier))

			require.Error(t, err)
			require.Equal(t, http.StatusBadRequest, infraerrors.Code(err))
			require.Equal(t, "INVALID_AFFILIATE_TIER_CONFIG", infraerrors.Reason(err))
			require.Nil(t, repo.updates)
		})
	}
}

func TestSettingService_UpdateSettings_AffiliateTierRejectsExplicitAllZeroConfig(t *testing.T) {
	repo := &settingUpdateRepoStub{}
	svc := NewSettingService(repo, &config.Config{})
	settings := &SystemSettings{}
	settings.SetAffiliateTierConfig(AffiliateTierConfig{})

	err := svc.UpdateSettings(context.Background(), settings)

	require.Error(t, err)
	require.Equal(t, "INVALID_AFFILIATE_TIER_CONFIG", infraerrors.Reason(err))
	require.Nil(t, repo.updates)
}

func TestSettingService_AffiliateTierReconcileRequired(t *testing.T) {
	repo := &affiliateTierReconcileRepoStub{values: map[string]string{
		SettingKeyAffiliateTierReconcileRequired: "true",
	}}
	svc := NewSettingService(repo, &config.Config{})

	required, err := svc.IsAffiliateTierReconcileRequired(context.Background())
	require.NoError(t, err)
	require.True(t, required)

	require.NoError(t, svc.SetAffiliateTierReconcileRequired(context.Background(), false))
	require.Equal(t, "false", repo.values[SettingKeyAffiliateTierReconcileRequired])
}

type affiliateTierReconcileRepoStub struct {
	values map[string]string
}

func (s *affiliateTierReconcileRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *affiliateTierReconcileRepoStub) GetValue(_ context.Context, key string) (string, error) {
	value, ok := s.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (s *affiliateTierReconcileRepoStub) Set(_ context.Context, key, value string) error {
	s.values[key] = value
	return nil
}

func (s *affiliateTierReconcileRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	out := make(map[string]string, len(s.values))
	for key, value := range s.values {
		out[key] = value
	}
	return out, nil
}

type affiliateTierReadErrorRepoStub struct {
	err error
}

func (s *affiliateTierReadErrorRepoStub) Get(context.Context, string) (*Setting, error) {
	return nil, s.err
}
func (s *affiliateTierReadErrorRepoStub) GetValue(context.Context, string) (string, error) {
	return "", s.err
}
func (s *affiliateTierReadErrorRepoStub) Set(context.Context, string, string) error {
	return s.err
}
func (s *affiliateTierReadErrorRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, s.err
}
func (s *affiliateTierReadErrorRepoStub) SetMultiple(context.Context, map[string]string) error {
	return s.err
}
func (s *affiliateTierReadErrorRepoStub) GetAll(context.Context) (map[string]string, error) {
	return nil, s.err
}
func (s *affiliateTierReadErrorRepoStub) Delete(context.Context, string) error {
	return s.err
}

func (s *affiliateTierReconcileRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *affiliateTierReconcileRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (s *affiliateTierReconcileRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func settingsWithAffiliateTier(tier AffiliateTierConfig) *SystemSettings {
	return &SystemSettings{
		AffiliateRebateRate:          tier.StandardRate,
		AffiliateQualificationAmount: tier.QualificationAmount,
		AffiliateBronzeInvitees:      tier.BronzeInvitees,
		AffiliateBronzeRate:          tier.BronzeRate,
		AffiliateSilverInvitees:      tier.SilverInvitees,
		AffiliateSilverRate:          tier.SilverRate,
		AffiliateGoldInvitees:        tier.GoldInvitees,
		AffiliateGoldRate:            tier.GoldRate,
	}
}

func withQualificationAmount(cfg AffiliateTierConfig, value float64) AffiliateTierConfig {
	cfg.QualificationAmount = value
	return cfg
}
func withBronzeInvitees(cfg AffiliateTierConfig, value int) AffiliateTierConfig {
	cfg.BronzeInvitees = value
	return cfg
}
func withSilverInvitees(cfg AffiliateTierConfig, value int) AffiliateTierConfig {
	cfg.SilverInvitees = value
	return cfg
}
func withGoldInvitees(cfg AffiliateTierConfig, value int) AffiliateTierConfig {
	cfg.GoldInvitees = value
	return cfg
}
func withStandardRate(cfg AffiliateTierConfig, value float64) AffiliateTierConfig {
	cfg.StandardRate = value
	return cfg
}
func withSilverRate(cfg AffiliateTierConfig, value float64) AffiliateTierConfig {
	cfg.SilverRate = value
	return cfg
}
func withGoldRate(cfg AffiliateTierConfig, value float64) AffiliateTierConfig {
	cfg.GoldRate = value
	return cfg
}
