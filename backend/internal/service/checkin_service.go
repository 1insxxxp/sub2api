package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"sort"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrCheckinDisabled    = infraerrors.BadRequest("CHECKIN_DISABLED", "check-in is disabled")
	ErrCheckinBlacklisted = infraerrors.Forbidden("CHECKIN_BLACKLISTED", "check-in is unavailable for this account")
)

type CheckinRewardConfig struct {
	Enabled       *bool               `json:"enabled,omitempty"`
	Tiers         []CheckinRewardTier `json:"tiers"`
	StreakEnabled bool                `json:"streak_enabled"`
	StreakRules   []CheckinStreakRule `json:"streak_rules"`
}

type CheckinRewardTier struct {
	Amount      float64 `json:"amount"`
	Probability float64 `json:"probability"`
	SortOrder   int     `json:"sort_order"`
}

type CheckinStreakRule struct {
	Day         int     `json:"day"`
	BonusAmount float64 `json:"bonus_amount"`
}

type CheckinStatus struct {
	Enabled             bool    `json:"enabled"`
	UserID              int64   `json:"user_id,omitempty"`
	CheckinDate         string  `json:"checkin_date"`
	CheckedInToday      bool    `json:"checked_in_today"`
	CurrentStreak       int     `json:"current_streak"`
	LastCheckinDate     string  `json:"last_checkin_date,omitempty"`
	LifetimeCheckinDays int     `json:"lifetime_checkin_days"`
	BaseRewardAmount    float64 `json:"base_reward_amount"`
	BonusRewardAmount   float64 `json:"bonus_reward_amount"`
	TotalRewardAmount   float64 `json:"total_reward_amount"`
	BalanceBefore       float64 `json:"balance_before"`
	BalanceAfter        float64 `json:"balance_after"`
	Blacklisted         bool    `json:"blacklisted"`
	Message             string  `json:"message,omitempty"`
}

type CheckinResult struct {
	CheckinStatus
}

type CheckinCreateInput struct {
	UserID           int64
	CheckinDate      string
	BaseRewardAmount float64
	StreakEnabled    bool
	StreakRules      []CheckinStreakRule
}

type CheckinRepository interface {
	GetStatus(ctx context.Context, userID int64, checkinDate string) (*CheckinStatus, error)
	Checkin(ctx context.Context, input CheckinCreateInput) (*CheckinResult, error)
}

type CheckinService struct {
	repo                 CheckinRepository
	settingRepo          SettingRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCache         BillingCache
	now                  func() time.Time
	randomFloat          func() float64
}

func NewCheckinService(
	repo CheckinRepository,
	settingRepo SettingRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCache BillingCache,
) *CheckinService {
	return &CheckinService{
		repo:                 repo,
		settingRepo:          settingRepo,
		authCacheInvalidator: authCacheInvalidator,
		billingCache:         billingCache,
		now:                  time.Now,
		randomFloat:          rand.Float64,
	}
}

func (s *CheckinService) SetRandomFloatForTest(fn func() float64) {
	if fn != nil {
		s.randomFloat = fn
	}
}

func (s *CheckinService) GetStatus(ctx context.Context, userID int64) (*CheckinStatus, error) {
	config, err := s.loadRewardConfig(ctx)
	if err != nil {
		return nil, err
	}
	date := s.currentCheckinDate()
	if !config.isEnabled() {
		return &CheckinStatus{
			Enabled:     false,
			UserID:      userID,
			CheckinDate: date,
			Message:     "check-in is disabled",
		}, nil
	}
	status, err := s.repo.GetStatus(ctx, userID, date)
	if err != nil {
		return nil, err
	}
	status.Enabled = true
	status.CheckinDate = date
	return status, nil
}

func (s *CheckinService) Checkin(ctx context.Context, userID int64) (*CheckinResult, error) {
	config, err := s.loadRewardConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !config.isEnabled() {
		return nil, ErrCheckinDisabled
	}
	input := CheckinCreateInput{
		UserID:           userID,
		CheckinDate:      s.currentCheckinDate(),
		BaseRewardAmount: config.pickBaseReward(s.randomFloat()),
		StreakEnabled:    config.StreakEnabled,
		StreakRules:      append([]CheckinStreakRule(nil), config.StreakRules...),
	}
	result, err := s.repo.Checkin(ctx, input)
	if err != nil {
		return nil, err
	}
	result.Enabled = true
	result.CheckinDate = input.CheckinDate
	s.invalidateBalanceCaches(ctx, userID)
	return result, nil
}

func (s *CheckinService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCache == nil {
		return
	}
	go func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("panic in check-in balance cache invalidation", "user_id", userID, "recover", r)
			}
		}()
		cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.billingCache.InvalidateUserBalance(cacheCtx, userID); err != nil {
			slog.Error("invalidate user balance cache after check-in failed", "user_id", userID, "error", err)
		}
	}()
}

func (s *CheckinService) loadRewardConfig(ctx context.Context) (CheckinRewardConfig, error) {
	if s.settingRepo == nil {
		return CheckinRewardConfig{}, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyCheckinRewardConfig)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return CheckinRewardConfig{}, nil
		}
		return CheckinRewardConfig{}, fmt.Errorf("get check-in reward config: %w", err)
	}
	var config CheckinRewardConfig
	if err := json.Unmarshal([]byte(raw), &config); err != nil {
		return CheckinRewardConfig{}, infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_INVALID", "invalid check-in reward config")
	}
	config.normalize()
	return config, nil
}

func (s *CheckinService) currentCheckinDate() string {
	now := time.Now()
	if s != nil && s.now != nil {
		now = s.now()
	}
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return now.UTC().Format(time.DateOnly)
	}
	return now.In(loc).Format(time.DateOnly)
}

func (c CheckinRewardConfig) isEnabled() bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	return len(c.Tiers) > 0
}

func (c *CheckinRewardConfig) normalize() {
	if c == nil {
		return
	}
	filtered := c.Tiers[:0]
	for _, tier := range c.Tiers {
		if tier.Amount <= 0 || tier.Probability <= 0 {
			continue
		}
		filtered = append(filtered, tier)
	}
	c.Tiers = filtered
	sort.SliceStable(c.Tiers, func(i, j int) bool {
		return c.Tiers[i].SortOrder < c.Tiers[j].SortOrder
	})
	sort.SliceStable(c.StreakRules, func(i, j int) bool {
		return c.StreakRules[i].Day < c.StreakRules[j].Day
	})
}

func (c CheckinRewardConfig) pickBaseReward(r float64) float64 {
	total := 0.0
	for _, tier := range c.Tiers {
		if tier.Probability > 0 && tier.Amount > 0 {
			total += tier.Probability
		}
	}
	if total <= 0 {
		return 0
	}
	if r < 0 {
		r = 0
	}
	if r > 1 {
		r = 1
	}
	target := r * total
	accumulated := 0.0
	for _, tier := range c.Tiers {
		if tier.Probability <= 0 || tier.Amount <= 0 {
			continue
		}
		accumulated += tier.Probability
		if target <= accumulated {
			return tier.Amount
		}
	}
	return c.Tiers[len(c.Tiers)-1].Amount
}

func CheckinBonusForStreak(streakDay int, rules []CheckinStreakRule) float64 {
	for _, rule := range rules {
		if rule.Day == streakDay && rule.BonusAmount > 0 {
			return rule.BonusAmount
		}
	}
	return 0
}
