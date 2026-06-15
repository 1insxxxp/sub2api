package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/usagelog"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/usercheckin"
	"github.com/Wei-Shaw/sub2api/ent/usercheckinblacklist"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrCheckinBlacklisted       = infraerrors.New(http.StatusForbidden, "CHECKIN_BLACKLISTED", "check-in is not available for this user")
	ErrCheckinDisabled          = infraerrors.New(http.StatusForbidden, "CHECKIN_DISABLED", "daily check-in is currently disabled")
	ErrCheckinInsufficientSpend = infraerrors.New(http.StatusForbidden, "CHECKIN_INSUFFICIENT_SPEND", "minimum cumulative spend is required before check-in")
	ErrCheckinNotFound          = infraerrors.NotFound("CHECKIN_NOT_FOUND", "check-in record not found")
)

const (
	CheckinIneligibleReasonDisabled          = "disabled"
	CheckinIneligibleReasonBlacklisted       = "blacklisted"
	CheckinIneligibleReasonInsufficientSpend = "insufficient_spend"

	checkinRewardProbabilityScale = 100
	checkinRewardProbabilityTotal = 100 * checkinRewardProbabilityScale
	checkinRewardMaxTiers         = 20
	checkinStreakMaxRules         = 20
	checkinStreakLookbackFloor    = 400
)

type CheckinConfig struct {
	Enabled          bool                 `json:"enabled"`
	MinTotalUsageUSD float64              `json:"min_total_usage_usd"`
	Tiers            []CheckinRewardTier  `json:"tiers"`
	StreakEnabled    bool                 `json:"streak_enabled"`
	StreakRules      []CheckinStreakRule  `json:"streak_rules"`
	ProbabilityTotal float64              `json:"probability_total"`
	Preview          CheckinRewardPreview `json:"preview"`
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

type CheckinRewardPreview struct {
	MinReward     float64 `json:"min_reward"`
	MaxReward     float64 `json:"max_reward"`
	AverageReward float64 `json:"average_reward"`
}

type CheckinStatus struct {
	Enabled           bool               `json:"enabled"`
	Eligible          bool               `json:"eligible"`
	Blacklisted       bool               `json:"blacklisted"`
	CheckedIn         bool               `json:"checked_in"`
	CheckinDate       string             `json:"checkin_date"`
	StreakDay         int                `json:"streak_day,omitempty"`
	CurrentStreak     int                `json:"current_streak"`
	LifetimeDays      int                `json:"lifetime_checkin_days"`
	BaseRewardAmount  float64            `json:"base_reward_amount,omitempty"`
	BonusRewardAmount float64            `json:"bonus_reward_amount,omitempty"`
	TotalRewardAmount float64            `json:"total_reward_amount,omitempty"`
	RewardAmount      float64            `json:"reward_amount,omitempty"`
	CheckedInAt       *time.Time         `json:"checked_in_at,omitempty"`
	NextResetAt       time.Time          `json:"next_reset_at"`
	MinTotalUsageUSD  float64            `json:"min_total_usage_usd"`
	TotalUsageUSD     float64            `json:"total_usage_usd"`
	IneligibleReason  string             `json:"ineligible_reason,omitempty"`
	NextStreakRule    *CheckinStreakRule `json:"next_streak_rule,omitempty"`
	RecentRecords     []CheckinRecord    `json:"recent_records,omitempty"`
}

type CheckinResult struct {
	CheckinStatus
	AlreadyCheckedIn bool    `json:"already_checked_in"`
	BalanceBefore    float64 `json:"balance_before,omitempty"`
	BalanceAfter     float64 `json:"balance_after"`
}

type CheckinRecord struct {
	ID                int64     `json:"id"`
	UserID            int64     `json:"user_id"`
	UserEmail         string    `json:"user_email,omitempty"`
	Username          string    `json:"username,omitempty"`
	CheckinDate       string    `json:"checkin_date"`
	StreakDay         int       `json:"streak_day"`
	BaseRewardAmount  float64   `json:"base_reward_amount"`
	BonusRewardAmount float64   `json:"bonus_reward_amount"`
	TotalRewardAmount float64   `json:"total_reward_amount"`
	RewardAmount      float64   `json:"reward_amount"`
	BalanceBefore     float64   `json:"balance_before"`
	BalanceAfter      float64   `json:"balance_after"`
	CreatedAt         time.Time `json:"created_at"`
}

type CheckinBlacklistEntry struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"user_id"`
	UserEmail string     `json:"user_email,omitempty"`
	Username  string     `json:"username,omitempty"`
	Reason    string     `json:"reason,omitempty"`
	CreatedBy *int64     `json:"created_by,omitempty"`
	RemovedBy *int64     `json:"removed_by,omitempty"`
	RemovedAt *time.Time `json:"removed_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

type CheckinStats struct {
	TodayCount        int64   `json:"today_count"`
	TodayRewardTotal  float64 `json:"today_reward_total"`
	SevenDayCount     int64   `json:"seven_day_count"`
	SevenDayReward    float64 `json:"seven_day_reward_total"`
	ThirtyDayCount    int64   `json:"thirty_day_count"`
	ThirtyDayReward   float64 `json:"thirty_day_reward_total"`
	ActiveBlacklist   int64   `json:"active_blacklist_count"`
	CurrentCheckinDay string  `json:"current_checkin_day"`
}

type CheckinListFilters struct {
	UserID int64
	Date   string
	Search string
}

type AddCheckinBlacklistInput struct {
	UserID    int64
	Reason    string
	CreatedBy int64
}

type CheckinService struct {
	entClient            *dbent.Client
	settingRepo          SettingRepository
	authCacheInvalidator APIKeyAuthCacheInvalidator
	billingCacheService  BillingCache
	now                  func() time.Time
	rewardRoll           func() float64
	beijingLocation      *time.Location
}

func NewCheckinService(
	entClient *dbent.Client,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService BillingCache,
) *CheckinService {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return &CheckinService{
		entClient:            entClient,
		authCacheInvalidator: authCacheInvalidator,
		billingCacheService:  billingCacheService,
		now:                  time.Now,
		rewardRoll:           rand.Float64,
		beijingLocation:      loc,
	}
}

func ProvideCheckinService(
	entClient *dbent.Client,
	settingRepo SettingRepository,
	authCacheInvalidator APIKeyAuthCacheInvalidator,
	billingCacheService BillingCache,
) *CheckinService {
	svc := NewCheckinService(entClient, authCacheInvalidator, billingCacheService)
	svc.SetSettingRepository(settingRepo)
	return svc
}

func (s *CheckinService) SetSettingRepository(settingRepo SettingRepository) {
	s.settingRepo = settingRepo
}

func (s *CheckinService) GetStatus(ctx context.Context, userID int64) (*CheckinStatus, error) {
	checkinDate, nextReset := s.currentBeijingDay()
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	totalUsage, err := s.totalUsageUSD(ctx, userID)
	if err != nil {
		return nil, err
	}
	base := CheckinStatus{
		Enabled:          cfg.Enabled,
		Eligible:         cfg.Enabled,
		Blacklisted:      false,
		CheckedIn:        false,
		CheckinDate:      checkinDate,
		NextResetAt:      nextReset,
		MinTotalUsageUSD: cfg.MinTotalUsageUSD,
		TotalUsageUSD:    totalUsage,
	}
	if snapshot, snapshotErr := s.checkinHistorySnapshot(ctx, userID, checkinDate, cfg); snapshotErr != nil {
		return nil, snapshotErr
	} else {
		base.CurrentStreak = snapshot.CurrentStreak
		base.LifetimeDays = snapshot.LifetimeDays
		base.NextStreakRule = nextCheckinStreakRule(cfg.StreakRules, snapshot.CurrentStreak)
		base.RecentRecords = snapshot.RecentRecords
	}
	if !cfg.Enabled {
		base.Eligible = false
		base.IneligibleReason = CheckinIneligibleReasonDisabled
		return &base, nil
	}
	blacklisted, err := s.isBlacklisted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		base.Enabled = false
		base.Eligible = false
		base.Blacklisted = true
		base.IneligibleReason = CheckinIneligibleReasonBlacklisted
		return &base, nil
	}
	if !checkinSpendEligible(totalUsage, cfg.MinTotalUsageUSD) {
		base.Eligible = false
		base.IneligibleReason = CheckinIneligibleReasonInsufficientSpend
	}
	record, err := s.getCheckinByUserAndDate(ctx, userID, checkinDate)
	if err != nil {
		return nil, err
	}
	status := base
	status.CheckedIn = record != nil
	if record != nil {
		status.StreakDay = record.StreakDay
		status.CurrentStreak = record.StreakDay
		status.BaseRewardAmount = record.BaseRewardAmount
		status.BonusRewardAmount = record.BonusRewardAmount
		status.TotalRewardAmount = record.TotalRewardAmount
		status.RewardAmount = record.RewardAmount
		status.CheckedInAt = &record.CreatedAt
		status.NextStreakRule = nextCheckinStreakRule(cfg.StreakRules, record.StreakDay)
	}
	return &status, nil
}

func (s *CheckinService) Checkin(ctx context.Context, userID int64) (*CheckinResult, error) {
	checkinDate, nextReset := s.currentBeijingDay()
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, ErrCheckinDisabled
	}
	totalUsage, err := s.totalUsageUSD(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !checkinSpendEligible(totalUsage, cfg.MinTotalUsageUSD) {
		return nil, ErrCheckinInsufficientSpend
	}
	blacklisted, err := s.isBlacklisted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		return nil, ErrCheckinBlacklisted
	}

	existing, err := s.getCheckinByUserAndDate(ctx, userID, checkinDate)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return s.alreadyCheckedInResult(ctx, existing, nextReset)
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin check-in transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	if active, err := client.UserCheckinBlacklist.Query().
		Where(
			usercheckinblacklist.UserIDEQ(userID),
			usercheckinblacklist.RemovedAtIsNil(),
		).
		Exist(txCtx); err != nil {
		return nil, fmt.Errorf("check blacklist: %w", err)
	} else if active {
		return nil, ErrCheckinBlacklisted
	}

	txTotalUsage, err := s.totalUsageUSDWithClient(txCtx, client, userID)
	if err != nil {
		return nil, err
	}
	if !checkinSpendEligible(txTotalUsage, cfg.MinTotalUsageUSD) {
		return nil, ErrCheckinInsufficientSpend
	}

	streakDay, lifetimeDays, err := s.nextCheckinCounters(txCtx, client, userID, checkinDate, cfg)
	if err != nil {
		return nil, fmt.Errorf("compute check-in streak: %w", err)
	}
	baseReward := selectCheckinReward(*cfg, s.rewardRoll())
	bonusReward := selectCheckinStreakBonus(*cfg, streakDay)
	reward := baseReward + bonusReward
	updatedUser, err := client.User.UpdateOneID(userID).
		AddBalance(reward).
		Save(txCtx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrUserNotFound.WithCause(err)
		}
		return nil, fmt.Errorf("update balance for check-in: %w", err)
	}

	balanceAfter := updatedUser.Balance
	balanceBefore := balanceAfter - reward
	createdRecord, err := client.UserCheckin.Create().
		SetUserID(userID).
		SetCheckinDate(checkinDate).
		SetStreakDay(streakDay).
		SetBaseRewardAmount(baseReward).
		SetBonusRewardAmount(bonusReward).
		SetTotalRewardAmount(reward).
		SetRewardAmount(reward).
		SetBalanceBefore(balanceBefore).
		SetBalanceAfter(balanceAfter).
		Save(txCtx)
	if err != nil {
		if isUniqueConstraintError(err) {
			return s.rollbackAndLoadExisting(ctx, tx, userID, checkinDate, nextReset)
		}
		return nil, fmt.Errorf("create check-in record: %w", err)
	}

	code, err := GenerateRedeemCode()
	if err != nil {
		return nil, fmt.Errorf("generate check-in history code: %w", err)
	}
	now := s.now()
	if _, err := client.RedeemCode.Create().
		SetCode(code).
		SetType(AdjustmentTypeCheckinReward).
		SetValue(reward).
		SetStatus(StatusUsed).
		SetUsedBy(userID).
		SetUsedAt(now).
		SetNotes(fmt.Sprintf("daily check-in reward %s", checkinDate)).
		Save(txCtx); err != nil {
		return nil, fmt.Errorf("create check-in balance history: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit check-in transaction: %w", err)
	}
	s.invalidateBalanceCaches(ctx, userID)

	checkedInAt := createdRecord.CreatedAt
	recentRecords, _ := s.ListHistoryForUser(ctx, userID, 7)
	return &CheckinResult{
		CheckinStatus: CheckinStatus{
			Enabled:           true,
			Eligible:          true,
			Blacklisted:       false,
			CheckedIn:         true,
			CheckinDate:       checkinDate,
			StreakDay:         streakDay,
			CurrentStreak:     streakDay,
			LifetimeDays:      lifetimeDays,
			BaseRewardAmount:  baseReward,
			BonusRewardAmount: bonusReward,
			TotalRewardAmount: reward,
			RewardAmount:      reward,
			CheckedInAt:       &checkedInAt,
			NextResetAt:       nextReset,
			MinTotalUsageUSD:  cfg.MinTotalUsageUSD,
			TotalUsageUSD:     txTotalUsage,
			NextStreakRule:    nextCheckinStreakRule(cfg.StreakRules, streakDay),
			RecentRecords:     recentRecords,
		},
		AlreadyCheckedIn: false,
		BalanceBefore:    balanceBefore,
		BalanceAfter:     balanceAfter,
	}, nil
}

func (s *CheckinService) ListRecords(ctx context.Context, page, pageSize int, filters CheckinListFilters) ([]CheckinRecord, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	q := s.entClient.UserCheckin.Query().WithUser()
	if filters.UserID > 0 {
		q = q.Where(usercheckin.UserIDEQ(filters.UserID))
	}
	if strings.TrimSpace(filters.Date) != "" {
		q = q.Where(usercheckin.CheckinDateEQ(strings.TrimSpace(filters.Date)))
	}
	if search := strings.TrimSpace(filters.Search); search != "" {
		q = q.Where(usercheckin.HasUserWith(
			user.Or(
				user.EmailContainsFold(search),
				user.UsernameContainsFold(search),
			),
		))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	entities, err := q.
		Order(dbent.Desc(usercheckin.FieldCreatedAt), dbent.Desc(usercheckin.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	records := make([]CheckinRecord, 0, len(entities))
	for _, entity := range entities {
		records = append(records, checkinRecordFromEntity(entity))
	}
	return records, int64(total), nil
}

func (s *CheckinService) GetStats(ctx context.Context) (*CheckinStats, error) {
	today, _ := s.currentBeijingDay()
	todayStart := today
	sevenStart := s.dateDaysAgo(6)
	thirtyStart := s.dateDaysAgo(29)

	todayCount, todayReward, err := s.aggregateCheckins(ctx, todayStart, today)
	if err != nil {
		return nil, err
	}
	sevenCount, sevenReward, err := s.aggregateCheckins(ctx, sevenStart, today)
	if err != nil {
		return nil, err
	}
	thirtyCount, thirtyReward, err := s.aggregateCheckins(ctx, thirtyStart, today)
	if err != nil {
		return nil, err
	}
	activeBlacklist, err := s.entClient.UserCheckinBlacklist.Query().
		Where(usercheckinblacklist.RemovedAtIsNil()).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	return &CheckinStats{
		TodayCount:        int64(todayCount),
		TodayRewardTotal:  todayReward,
		SevenDayCount:     int64(sevenCount),
		SevenDayReward:    sevenReward,
		ThirtyDayCount:    int64(thirtyCount),
		ThirtyDayReward:   thirtyReward,
		ActiveBlacklist:   int64(activeBlacklist),
		CurrentCheckinDay: today,
	}, nil
}

func (s *CheckinService) ListBlacklist(ctx context.Context, page, pageSize int, activeOnly bool, search string) ([]CheckinBlacklistEntry, int64, error) {
	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	q := s.entClient.UserCheckinBlacklist.Query().WithUser()
	if activeOnly {
		q = q.Where(usercheckinblacklist.RemovedAtIsNil())
	}
	if trimmed := strings.TrimSpace(search); trimmed != "" {
		q = q.Where(usercheckinblacklist.HasUserWith(
			user.Or(
				user.EmailContainsFold(trimmed),
				user.UsernameContainsFold(trimmed),
			),
		))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	entities, err := q.
		Order(dbent.Desc(usercheckinblacklist.FieldCreatedAt), dbent.Desc(usercheckinblacklist.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	items := make([]CheckinBlacklistEntry, 0, len(entities))
	for _, entity := range entities {
		items = append(items, checkinBlacklistEntryFromEntity(entity))
	}
	return items, int64(total), nil
}

func (s *CheckinService) AddBlacklist(ctx context.Context, input AddCheckinBlacklistInput) (*CheckinBlacklistEntry, error) {
	if input.UserID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER_ID", "invalid user id")
	}
	if _, err := s.entClient.User.Get(ctx, input.UserID); err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrUserNotFound.WithCause(err)
		}
		return nil, err
	}
	existing, err := s.entClient.UserCheckinBlacklist.Query().
		Where(
			usercheckinblacklist.UserIDEQ(input.UserID),
			usercheckinblacklist.RemovedAtIsNil(),
		).
		WithUser().
		Only(ctx)
	if err == nil {
		out := checkinBlacklistEntryFromEntity(existing)
		return &out, nil
	}
	if !dbent.IsNotFound(err) {
		return nil, err
	}

	create := s.entClient.UserCheckinBlacklist.Create().
		SetUserID(input.UserID).
		SetReason(strings.TrimSpace(input.Reason))
	if input.CreatedBy > 0 {
		create.SetCreatedBy(input.CreatedBy)
	}
	created, err := create.Save(ctx)
	if err != nil {
		if isUniqueConstraintError(err) {
			current, getErr := s.entClient.UserCheckinBlacklist.Query().
				Where(
					usercheckinblacklist.UserIDEQ(input.UserID),
					usercheckinblacklist.RemovedAtIsNil(),
				).
				WithUser().
				Only(ctx)
			if getErr == nil {
				out := checkinBlacklistEntryFromEntity(current)
				return &out, nil
			}
		}
		return nil, err
	}
	created, err = s.entClient.UserCheckinBlacklist.Query().
		Where(usercheckinblacklist.IDEQ(created.ID)).
		WithUser().
		Only(ctx)
	if err != nil {
		return nil, err
	}
	out := checkinBlacklistEntryFromEntity(created)
	return &out, nil
}

func (s *CheckinService) RemoveBlacklist(ctx context.Context, userID, removedBy int64) error {
	if userID <= 0 {
		return infraerrors.BadRequest("INVALID_USER_ID", "invalid user id")
	}
	update := s.entClient.UserCheckinBlacklist.Update().
		Where(
			usercheckinblacklist.UserIDEQ(userID),
			usercheckinblacklist.RemovedAtIsNil(),
		).
		SetRemovedAt(s.now())
	if removedBy > 0 {
		update.SetRemovedBy(removedBy)
	}
	_, err := update.Save(ctx)
	return err
}

func (s *CheckinService) GetConfig(ctx context.Context) (*CheckinConfig, error) {
	cfg := DefaultCheckinConfig()
	if s.settingRepo == nil {
		return cfg, nil
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyCheckinEnabled,
		SettingKeyCheckinMinTotalUsageUSD,
		SettingKeyCheckinRewardConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("get check-in config: %w", err)
	}
	if raw := strings.TrimSpace(values[SettingKeyCheckinEnabled]); raw != "" {
		enabled, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			return nil, infraerrors.BadRequest("INVALID_CHECKIN_ENABLED", "invalid check-in enabled setting")
		}
		cfg.Enabled = enabled
	}
	if raw := strings.TrimSpace(values[SettingKeyCheckinMinTotalUsageUSD]); raw != "" {
		minUsage, parseErr := strconv.ParseFloat(raw, 64)
		if parseErr != nil || minUsage < 0 {
			return nil, infraerrors.BadRequest("INVALID_CHECKIN_MIN_TOTAL_USAGE_USD", "minimum total usage must be a non-negative number")
		}
		cfg.MinTotalUsageUSD = minUsage
	}
	if raw := strings.TrimSpace(values[SettingKeyCheckinRewardConfig]); raw != "" {
		var rewardConfig CheckinConfig
		if err := json.Unmarshal([]byte(raw), &rewardConfig); err != nil {
			slog.Warn("checkin reward config is invalid json, falling back to defaults", "error", err)
		} else if normalized, validateErr := normalizeRewardRules(rewardConfig); validateErr != nil {
			slog.Warn("checkin reward config failed validation, falling back to defaults", "error", validateErr)
		} else {
			cfg.Tiers = normalized.Tiers
			cfg.StreakEnabled = normalized.StreakEnabled
			cfg.StreakRules = normalized.StreakRules
		}
	}
	finalized, err := normalizeCheckinConfig(*cfg)
	if err != nil {
		return nil, err
	}
	return finalized, nil
}

func (s *CheckinService) UpdateConfig(ctx context.Context, cfg CheckinConfig) (*CheckinConfig, error) {
	if s.settingRepo == nil {
		return nil, fmt.Errorf("check-in settings repository is not configured")
	}
	normalized, err := normalizeCheckinConfig(cfg)
	if err != nil {
		return nil, err
	}
	if err := s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyCheckinEnabled:          strconv.FormatBool(normalized.Enabled),
		SettingKeyCheckinMinTotalUsageUSD: strconv.FormatFloat(normalized.MinTotalUsageUSD, 'f', -1, 64),
		SettingKeyCheckinRewardConfig:     mustMarshalCheckinRewardConfig(*normalized),
	}); err != nil {
		return nil, fmt.Errorf("update check-in config: %w", err)
	}
	return normalized, nil
}

func (s *CheckinService) currentBeijingDay() (string, time.Time) {
	now := s.now().In(s.beijingLocation)
	year, month, day := now.Date()
	todayStart := time.Date(year, month, day, 0, 0, 0, 0, s.beijingLocation)
	nextReset := todayStart.Add(24 * time.Hour)
	return todayStart.Format("2006-01-02"), nextReset
}

func DefaultCheckinConfig() *CheckinConfig {
	cfg := &CheckinConfig{
		Enabled:          true,
		MinTotalUsageUSD: 0,
		Tiers: []CheckinRewardTier{
			{Amount: 1, Probability: 32, SortOrder: 1},
			{Amount: 2, Probability: 25, SortOrder: 2},
			{Amount: 3, Probability: 18, SortOrder: 3},
			{Amount: 4, Probability: 10, SortOrder: 4},
			{Amount: 4.5, Probability: 8, SortOrder: 5},
			{Amount: 5, Probability: 5, SortOrder: 6},
			{Amount: 10, Probability: 2, SortOrder: 7},
		},
		StreakEnabled: true,
		StreakRules: []CheckinStreakRule{
			{Day: 7, BonusAmount: 10},
			{Day: 15, BonusAmount: 15},
			{Day: 30, BonusAmount: 20},
			{Day: 60, BonusAmount: 30},
			{Day: 120, BonusAmount: 50},
		},
	}
	normalized, _ := normalizeCheckinConfig(*cfg)
	return normalized
}

func normalizeCheckinConfig(cfg CheckinConfig) (*CheckinConfig, error) {
	if cfg.MinTotalUsageUSD < 0 {
		return nil, infraerrors.BadRequest("INVALID_CHECKIN_MIN_TOTAL_USAGE_USD", "minimum total usage must be a non-negative number")
	}
	rewardRules, err := normalizeRewardRules(cfg)
	if err != nil {
		return nil, err
	}
	normalized := &CheckinConfig{
		Enabled:          cfg.Enabled,
		MinTotalUsageUSD: cfg.MinTotalUsageUSD,
		Tiers:            rewardRules.Tiers,
		StreakEnabled:    rewardRules.StreakEnabled,
		StreakRules:      rewardRules.StreakRules,
	}
	normalized.ProbabilityTotal = checkinProbabilityTotal(normalized.Tiers)
	normalized.Preview = checkinRewardPreview(normalized.Tiers)
	return normalized, nil
}

func checkinSpendEligible(totalUsageUSD, minTotalUsageUSD float64) bool {
	return minTotalUsageUSD <= 0 || totalUsageUSD >= minTotalUsageUSD
}

func normalizeRewardRules(cfg CheckinConfig) (*CheckinConfig, error) {
	normalized := &CheckinConfig{
		Tiers:         make([]CheckinRewardTier, 0, len(cfg.Tiers)),
		StreakEnabled: cfg.StreakEnabled,
		StreakRules:   make([]CheckinStreakRule, 0, len(cfg.StreakRules)),
	}
	if len(cfg.Tiers) == 0 {
		return nil, infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_EMPTY", "at least one reward tier is required")
	}
	if len(cfg.Tiers) > checkinRewardMaxTiers {
		return nil, infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_TOO_MANY_TIERS", fmt.Sprintf("at most %d reward tiers are allowed", checkinRewardMaxTiers))
	}

	type tierWithIndex struct {
		CheckinRewardTier
		index int
	}
	tiers := make([]tierWithIndex, 0, len(cfg.Tiers))
	for index, tier := range cfg.Tiers {
		sortOrder := tier.SortOrder
		if sortOrder <= 0 {
			sortOrder = index + 1
		}
		tiers = append(tiers, tierWithIndex{
			CheckinRewardTier: CheckinRewardTier{
				Amount:      tier.Amount,
				Probability: tier.Probability,
				SortOrder:   sortOrder,
			},
			index: index,
		})
	}
	sort.SliceStable(tiers, func(i, j int) bool {
		if tiers[i].SortOrder == tiers[j].SortOrder {
			return tiers[i].index < tiers[j].index
		}
		return tiers[i].SortOrder < tiers[j].SortOrder
	})

	seenAmounts := make(map[int64]struct{}, len(tiers))
	var probabilityTotal int64
	for index, tier := range tiers {
		amountCents, err := scaledCheckinMoney(tier.Amount)
		if err != nil {
			return nil, infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_INVALID_AMOUNT", fmt.Sprintf("tier %d: %s", index+1, err.Error()))
		}
		if _, exists := seenAmounts[amountCents]; exists {
			return nil, infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_DUPLICATE_AMOUNT", "reward amounts must be unique")
		}
		seenAmounts[amountCents] = struct{}{}

		probabilityScaled, err := scaledCheckinProbability(tier.Probability)
		if err != nil {
			return nil, infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_INVALID_PROBABILITY", fmt.Sprintf("tier %d: %s", index+1, err.Error()))
		}
		probabilityTotal += probabilityScaled
		normalized.Tiers = append(normalized.Tiers, CheckinRewardTier{
			Amount:      float64(amountCents) / 100,
			Probability: float64(probabilityScaled) / checkinRewardProbabilityScale,
			SortOrder:   index + 1,
		})
	}
	if probabilityTotal != checkinRewardProbabilityTotal {
		return nil, infraerrors.BadRequest("CHECKIN_REWARD_CONFIG_INVALID_TOTAL", "reward probabilities must add up to exactly 100")
	}

	if len(cfg.StreakRules) > checkinStreakMaxRules {
		return nil, infraerrors.BadRequest("CHECKIN_STREAK_CONFIG_TOO_MANY_RULES", fmt.Sprintf("at most %d streak rules are allowed", checkinStreakMaxRules))
	}
	seenDays := make(map[int]struct{}, len(cfg.StreakRules))
	for index, rule := range cfg.StreakRules {
		if rule.Day <= 0 {
			return nil, infraerrors.BadRequest("CHECKIN_STREAK_CONFIG_INVALID_DAY", fmt.Sprintf("rule %d: day must be greater than 0", index+1))
		}
		if _, exists := seenDays[rule.Day]; exists {
			return nil, infraerrors.BadRequest("CHECKIN_STREAK_CONFIG_DUPLICATE_DAY", "streak days must be unique")
		}
		seenDays[rule.Day] = struct{}{}
		bonusCents, err := scaledCheckinMoney(rule.BonusAmount)
		if err != nil {
			return nil, infraerrors.BadRequest("CHECKIN_STREAK_CONFIG_INVALID_BONUS_AMOUNT", fmt.Sprintf("rule %d: %s", index+1, err.Error()))
		}
		normalized.StreakRules = append(normalized.StreakRules, CheckinStreakRule{
			Day:         rule.Day,
			BonusAmount: float64(bonusCents) / 100,
		})
	}
	sort.SliceStable(normalized.StreakRules, func(i, j int) bool {
		return normalized.StreakRules[i].Day < normalized.StreakRules[j].Day
	})
	return normalized, nil
}

func mustMarshalCheckinRewardConfig(cfg CheckinConfig) string {
	payload := struct {
		Tiers         []CheckinRewardTier `json:"tiers"`
		StreakEnabled bool                `json:"streak_enabled"`
		StreakRules   []CheckinStreakRule `json:"streak_rules"`
	}{
		Tiers:         cfg.Tiers,
		StreakEnabled: cfg.StreakEnabled,
		StreakRules:   cfg.StreakRules,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(raw)
}

func checkinProbabilityTotal(tiers []CheckinRewardTier) float64 {
	var total float64
	for _, tier := range tiers {
		total += tier.Probability
	}
	return total
}

func checkinRewardPreview(tiers []CheckinRewardTier) CheckinRewardPreview {
	if len(tiers) == 0 {
		return CheckinRewardPreview{}
	}
	minReward := tiers[0].Amount
	maxReward := tiers[0].Amount
	var average float64
	for _, tier := range tiers {
		if tier.Amount < minReward {
			minReward = tier.Amount
		}
		if tier.Amount > maxReward {
			maxReward = tier.Amount
		}
		average += tier.Amount * tier.Probability / 100
	}
	return CheckinRewardPreview{
		MinReward:     minReward,
		MaxReward:     maxReward,
		AverageReward: average,
	}
}

func (s *CheckinService) dateDaysAgo(days int) string {
	today, _ := s.currentBeijingDay()
	parsed, err := time.ParseInLocation("2006-01-02", today, s.beijingLocation)
	if err != nil {
		return today
	}
	return parsed.AddDate(0, 0, -days).Format("2006-01-02")
}

func (s *CheckinService) isBlacklisted(ctx context.Context, userID int64) (bool, error) {
	return s.entClient.UserCheckinBlacklist.Query().
		Where(
			usercheckinblacklist.UserIDEQ(userID),
			usercheckinblacklist.RemovedAtIsNil(),
		).
		Exist(ctx)
}

func (s *CheckinService) getCheckinByUserAndDate(ctx context.Context, userID int64, checkinDate string) (*CheckinRecord, error) {
	record, err := s.entClient.UserCheckin.Query().
		Where(
			usercheckin.UserIDEQ(userID),
			usercheckin.CheckinDateEQ(checkinDate),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	out := checkinRecordFromEntity(record)
	return &out, nil
}

func (s *CheckinService) alreadyCheckedInResult(ctx context.Context, record *CheckinRecord, nextReset time.Time) (*CheckinResult, error) {
	latestBalance := record.BalanceAfter
	if userEntity, err := s.entClient.User.Get(ctx, record.UserID); err == nil {
		latestBalance = userEntity.Balance
	} else if !dbent.IsNotFound(err) {
		return nil, err
	}
	cfg, err := s.GetConfig(ctx)
	if err != nil {
		return nil, err
	}
	totalUsage, err := s.totalUsageUSD(ctx, record.UserID)
	if err != nil {
		return nil, err
	}
	history, err := s.checkinHistorySnapshot(ctx, record.UserID, record.CheckinDate, cfg)
	if err != nil {
		return nil, err
	}
	checkedInAt := record.CreatedAt
	return &CheckinResult{
		CheckinStatus: CheckinStatus{
			Enabled:           cfg.Enabled,
			Eligible:          cfg.Enabled && checkinSpendEligible(totalUsage, cfg.MinTotalUsageUSD),
			Blacklisted:       false,
			CheckedIn:         true,
			CheckinDate:       record.CheckinDate,
			StreakDay:         record.StreakDay,
			CurrentStreak:     record.StreakDay,
			LifetimeDays:      history.LifetimeDays,
			BaseRewardAmount:  record.BaseRewardAmount,
			BonusRewardAmount: record.BonusRewardAmount,
			TotalRewardAmount: record.TotalRewardAmount,
			RewardAmount:      record.RewardAmount,
			CheckedInAt:       &checkedInAt,
			NextResetAt:       nextReset,
			MinTotalUsageUSD:  cfg.MinTotalUsageUSD,
			TotalUsageUSD:     totalUsage,
			NextStreakRule:    nextCheckinStreakRule(cfg.StreakRules, record.StreakDay),
			RecentRecords:     history.RecentRecords,
		},
		AlreadyCheckedIn: true,
		BalanceBefore:    record.BalanceBefore,
		BalanceAfter:     latestBalance,
	}, nil
}

func (s *CheckinService) totalUsageUSD(ctx context.Context, userID int64) (float64, error) {
	return s.totalUsageUSDWithClient(ctx, s.entClient, userID)
}

func (s *CheckinService) totalUsageUSDWithClient(ctx context.Context, client *dbent.Client, userID int64) (float64, error) {
	var result []struct {
		Sum float64 `json:"sum"`
	}
	err := client.UsageLog.Query().
		Where(usagelog.UserIDEQ(userID)).
		Aggregate(dbent.As(dbent.Sum(usagelog.FieldTotalCost), "sum")).
		Scan(ctx, &result)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("sum user check-in usage: %w", err)
	}
	if len(result) == 0 {
		return 0, nil
	}
	return result[0].Sum, nil
}

type checkinHistorySnapshotResult struct {
	CurrentStreak int
	LifetimeDays  int
	RecentRecords []CheckinRecord
}

func (s *CheckinService) checkinHistorySnapshot(ctx context.Context, userID int64, checkinDate string, cfg *CheckinConfig) (*checkinHistorySnapshotResult, error) {
	records, err := s.ListHistoryForUser(ctx, userID, checkinHistoryLookbackLimit(cfg))
	if err != nil {
		return nil, err
	}
	lifetimeDays, err := s.countCheckinsForUser(ctx, s.entClient, userID)
	if err != nil {
		return nil, err
	}
	currentStreak := 0
	if len(records) > 0 {
		currentStreak = computeCurrentStreak(records, checkinDate, s.beijingLocation)
	}
	return &checkinHistorySnapshotResult{
		CurrentStreak: currentStreak,
		LifetimeDays:  lifetimeDays,
		RecentRecords: firstCheckinRecords(records, 7),
	}, nil
}

func (s *CheckinService) nextCheckinCounters(ctx context.Context, client *dbent.Client, userID int64, checkinDate string, cfg *CheckinConfig) (int, int, error) {
	records, err := listCheckinRecordsWithClient(ctx, client, userID, checkinHistoryLookbackLimit(cfg))
	if err != nil {
		return 0, 0, err
	}
	lifetimeDays, err := s.countCheckinsForUser(ctx, client, userID)
	if err != nil {
		return 0, 0, err
	}
	currentStreak := computeCurrentStreak(records, previousBeijingDate(checkinDate, s.beijingLocation), s.beijingLocation)
	if currentStreak <= 0 {
		currentStreak = 0
	}
	return currentStreak + 1, lifetimeDays + 1, nil
}

func (s *CheckinService) countCheckinsForUser(ctx context.Context, client *dbent.Client, userID int64) (int, error) {
	return client.UserCheckin.Query().
		Where(usercheckin.UserIDEQ(userID)).
		Count(ctx)
}

func (s *CheckinService) rollbackAndLoadExisting(ctx context.Context, tx *dbent.Tx, userID int64, checkinDate string, nextReset time.Time) (*CheckinResult, error) {
	_ = tx.Rollback()
	existing, err := s.getCheckinByUserAndDate(ctx, userID, checkinDate)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, ErrCheckinNotFound
	}
	return s.alreadyCheckedInResult(ctx, existing, nextReset)
}

func (s *CheckinService) invalidateBalanceCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		go func() {
			cacheCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = s.billingCacheService.InvalidateUserBalance(cacheCtx, userID)
		}()
	}
}

func (s *CheckinService) aggregateCheckins(ctx context.Context, startDate, endDate string) (int, float64, error) {
	records, err := s.entClient.UserCheckin.Query().
		Where(
			usercheckin.CheckinDateGTE(startDate),
			usercheckin.CheckinDateLTE(endDate),
		).
		All(ctx)
	if err != nil {
		return 0, 0, err
	}
	var reward float64
	for _, record := range records {
		reward += record.RewardAmount
	}
	return len(records), reward, nil
}

func checkinRewardForRoll(roll float64) float64 {
	return selectCheckinReward(*DefaultCheckinConfig(), roll)
}

func selectCheckinReward(cfg CheckinConfig, roll float64) float64 {
	normalized, err := normalizeCheckinConfig(cfg)
	if err != nil || len(normalized.Tiers) == 0 {
		normalized = DefaultCheckinConfig()
	}
	target := int64(math.Floor(clampRewardRoll(roll) * float64(checkinRewardProbabilityTotal)))
	var cumulative int64
	for _, tier := range normalized.Tiers {
		scaledProbability, err := scaledCheckinProbability(tier.Probability)
		if err != nil {
			continue
		}
		cumulative += scaledProbability
		if target < cumulative {
			return tier.Amount
		}
	}
	return normalized.Tiers[len(normalized.Tiers)-1].Amount
}

func selectCheckinStreakBonus(cfg CheckinConfig, streakDay int) float64 {
	if streakDay <= 0 {
		return 0
	}
	normalized, err := normalizeCheckinConfig(cfg)
	if err != nil || !normalized.StreakEnabled {
		return 0
	}
	for _, rule := range normalized.StreakRules {
		if rule.Day == streakDay {
			return rule.BonusAmount
		}
	}
	return 0
}

func nextCheckinStreakRule(rules []CheckinStreakRule, currentStreak int) *CheckinStreakRule {
	for _, rule := range rules {
		if rule.Day > currentStreak {
			copy := rule
			return &copy
		}
	}
	return nil
}

func checkinHistoryLookbackLimit(cfg *CheckinConfig) int {
	limit := checkinStreakLookbackFloor
	if cfg == nil {
		return limit
	}
	for _, rule := range cfg.StreakRules {
		if rule.Day > limit {
			limit = rule.Day
		}
	}
	return limit
}

func clampRewardRoll(roll float64) float64 {
	if math.IsNaN(roll) || roll < 0 {
		return 0
	}
	if roll >= 1 {
		return 0.9999999999
	}
	return roll
}

func scaledCheckinMoney(value float64) (int64, error) {
	if !isFinitePositive(value) {
		return 0, fmt.Errorf("amount must be greater than 0")
	}
	scaled := math.Round(value * 100)
	if math.Abs(value*100-scaled) > 1e-9 {
		return 0, fmt.Errorf("amount must use at most 2 decimal places")
	}
	return int64(scaled), nil
}

func scaledCheckinProbability(value float64) (int64, error) {
	if !isFinitePositive(value) {
		return 0, fmt.Errorf("probability must be greater than 0")
	}
	scaled := math.Round(value * checkinRewardProbabilityScale)
	if math.Abs(value*checkinRewardProbabilityScale-scaled) > 1e-9 {
		return 0, fmt.Errorf("probability must use at most 2 decimal places")
	}
	return int64(scaled), nil
}

func isFinitePositive(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value > 0
}

func computeCurrentStreak(records []CheckinRecord, endDate string, loc *time.Location) int {
	if len(records) == 0 || strings.TrimSpace(endDate) == "" {
		return 0
	}
	dateSet := make(map[string]struct{}, len(records))
	for _, record := range records {
		dateSet[record.CheckinDate] = struct{}{}
	}
	current, err := time.ParseInLocation("2006-01-02", endDate, loc)
	if err != nil {
		return 0
	}
	streak := 0
	for {
		if _, ok := dateSet[current.Format("2006-01-02")]; !ok {
			return streak
		}
		streak++
		current = current.AddDate(0, 0, -1)
	}
}

func previousBeijingDate(date string, loc *time.Location) string {
	parsed, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return date
	}
	return parsed.AddDate(0, 0, -1).Format("2006-01-02")
}

func firstCheckinRecords(records []CheckinRecord, limit int) []CheckinRecord {
	if limit <= 0 || len(records) <= limit {
		return records
	}
	return records[:limit]
}

func listCheckinRecordsWithClient(ctx context.Context, client *dbent.Client, userID int64, limit int) ([]CheckinRecord, error) {
	if limit <= 0 {
		limit = 10
	}
	entities, err := client.UserCheckin.Query().
		Where(usercheckin.UserIDEQ(userID)).
		Order(dbent.Desc(usercheckin.FieldCheckinDate), dbent.Desc(usercheckin.FieldID)).
		Limit(limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]CheckinRecord, 0, len(entities))
	for _, entity := range entities {
		out = append(out, checkinRecordFromEntity(entity))
	}
	return out, nil
}

func checkinRecordFromEntity(entity *dbent.UserCheckin) CheckinRecord {
	totalReward := entity.TotalRewardAmount
	if totalReward == 0 {
		totalReward = entity.RewardAmount
	}
	baseReward := entity.BaseRewardAmount
	if baseReward == 0 {
		baseReward = totalReward
	}
	out := CheckinRecord{
		ID:                entity.ID,
		UserID:            entity.UserID,
		CheckinDate:       entity.CheckinDate,
		StreakDay:         entity.StreakDay,
		BaseRewardAmount:  baseReward,
		BonusRewardAmount: entity.BonusRewardAmount,
		TotalRewardAmount: totalReward,
		RewardAmount:      totalReward,
		BalanceBefore:     entity.BalanceBefore,
		BalanceAfter:      entity.BalanceAfter,
		CreatedAt:         entity.CreatedAt,
	}
	if out.StreakDay <= 0 {
		out.StreakDay = 1
	}
	if entity.Edges.User != nil {
		out.UserEmail = entity.Edges.User.Email
		out.Username = entity.Edges.User.Username
	}
	return out
}

func checkinBlacklistEntryFromEntity(entity *dbent.UserCheckinBlacklist) CheckinBlacklistEntry {
	out := CheckinBlacklistEntry{
		ID:        entity.ID,
		UserID:    entity.UserID,
		CreatedBy: entity.CreatedBy,
		RemovedBy: entity.RemovedBy,
		RemovedAt: entity.RemovedAt,
		CreatedAt: entity.CreatedAt,
		UpdatedAt: entity.UpdatedAt,
	}
	if entity.Reason != nil {
		out.Reason = *entity.Reason
	}
	if entity.Edges.User != nil {
		out.UserEmail = entity.Edges.User.Email
		out.Username = entity.Edges.User.Username
	}
	return out
}

func isUniqueConstraintError(err error) bool {
	if err == nil {
		return false
	}
	if dbent.IsConstraintError(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unique constraint") ||
		strings.Contains(msg, "duplicate key") ||
		strings.Contains(msg, "constraint failed")
}

func (s *CheckinService) ListHistoryForUser(ctx context.Context, userID int64, limit int) ([]CheckinRecord, error) {
	return listCheckinRecordsWithClient(ctx, s.entClient, userID, limit)
}
