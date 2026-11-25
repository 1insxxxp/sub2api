package service

import (
	"context"
	"fmt"
	"strconv"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	checkinModeKey      = "checkin_browser_risk_mode"
	registrationModeKey = "account_risk_registration_mode"
	windowMinutesKey    = "account_risk_window_minutes"
	challengeCountKey   = "account_risk_challenge_count"
	cooldownCountKey    = "account_risk_cooldown_count"
	cooldownMinutesKey  = "account_risk_cooldown_minutes"
)

type AccountRiskConfig struct {
	CheckinMode      string `json:"checkin_mode"`
	RegistrationMode string `json:"registration_mode"`
	WindowMinutes    int    `json:"window_minutes"`
	ChallengeCount   int    `json:"challenge_count"`
	CooldownCount    int    `json:"cooldown_count"`
	CooldownMinutes  int    `json:"cooldown_minutes"`
}

func (cfg AccountRiskConfig) validate() error {
	validMode := func(mode string) bool { return mode == "observe" || mode == "enforce" }
	if !validMode(cfg.CheckinMode) || !validMode(cfg.RegistrationMode) ||
		cfg.WindowMinutes < 1 || cfg.WindowMinutes > 1440 ||
		cfg.ChallengeCount < 1 || cfg.CooldownCount <= cfg.ChallengeCount || cfg.CooldownCount > 10000 ||
		cfg.CooldownMinutes < 1 || cfg.CooldownMinutes > 10080 {
		return infraerrors.BadRequest("ACCOUNT_RISK_CONFIG_INVALID", "invalid account risk settings")
	}
	return nil
}

func ReadAccountRiskConfig(ctx context.Context, repo SettingRepository) (AccountRiskConfig, error) {
	cfg := AccountRiskConfig{CheckinMode: "observe", RegistrationMode: "observe", WindowMinutes: 5, ChallengeCount: 3, CooldownCount: 6, CooldownMinutes: 15}
	if repo == nil {
		return cfg, nil
	}
	values, err := repo.GetMultiple(ctx, []string{checkinModeKey, registrationModeKey, windowMinutesKey, challengeCountKey, cooldownCountKey, cooldownMinutesKey})
	if err != nil {
		return cfg, err
	}
	if v, ok := values[checkinModeKey]; ok {
		cfg.CheckinMode = v
	}
	if v, ok := values[registrationModeKey]; ok {
		cfg.RegistrationMode = v
	}
	for key, field := range map[string]*int{windowMinutesKey: &cfg.WindowMinutes, challengeCountKey: &cfg.ChallengeCount, cooldownCountKey: &cfg.CooldownCount, cooldownMinutesKey: &cfg.CooldownMinutes} {
		if v, ok := values[key]; ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return cfg, fmt.Errorf("read %s: %w", key, err)
			}
			*field = n
		}
	}
	return cfg, cfg.validate()
}

func (s *CheckinService) AccountRiskConfig(ctx context.Context) (AccountRiskConfig, error) {
	return ReadAccountRiskConfig(ctx, s.settingRepo)
}

func (s *CheckinService) UpdateAccountRiskConfig(ctx context.Context, cfg AccountRiskConfig) error {
	if err := cfg.validate(); err != nil {
		return err
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		checkinModeKey:      cfg.CheckinMode,
		registrationModeKey: cfg.RegistrationMode,
		windowMinutesKey:    strconv.Itoa(cfg.WindowMinutes),
		challengeCountKey:   strconv.Itoa(cfg.ChallengeCount),
		cooldownCountKey:    strconv.Itoa(cfg.CooldownCount),
		cooldownMinutesKey:  strconv.Itoa(cfg.CooldownMinutes),
	})
}
