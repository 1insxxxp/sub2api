package service

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/usercheckin"
	"github.com/Wei-Shaw/sub2api/ent/usercheckinblacklist"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrCheckinBlacklisted = infraerrors.New(http.StatusForbidden, "CHECKIN_BLACKLISTED", "check-in is not available for this user")
	ErrCheckinNotFound    = infraerrors.NotFound("CHECKIN_NOT_FOUND", "check-in record not found")
)

type CheckinStatus struct {
	Enabled      bool       `json:"enabled"`
	Blacklisted  bool       `json:"blacklisted"`
	CheckedIn    bool       `json:"checked_in"`
	CheckinDate  string     `json:"checkin_date"`
	RewardAmount float64    `json:"reward_amount,omitempty"`
	CheckedInAt  *time.Time `json:"checked_in_at,omitempty"`
	NextResetAt  time.Time  `json:"next_reset_at"`
}

type CheckinResult struct {
	CheckinStatus
	AlreadyCheckedIn bool    `json:"already_checked_in"`
	BalanceBefore    float64 `json:"balance_before,omitempty"`
	BalanceAfter     float64 `json:"balance_after"`
}

type CheckinRecord struct {
	ID            int64     `json:"id"`
	UserID        int64     `json:"user_id"`
	UserEmail     string    `json:"user_email,omitempty"`
	Username      string    `json:"username,omitempty"`
	CheckinDate   string    `json:"checkin_date"`
	RewardAmount  float64   `json:"reward_amount"`
	BalanceBefore float64   `json:"balance_before"`
	BalanceAfter  float64   `json:"balance_after"`
	CreatedAt     time.Time `json:"created_at"`
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
	entClient              *dbent.Client
	authCacheInvalidator   APIKeyAuthCacheInvalidator
	billingCacheService    BillingCache
	now                    func() time.Time
	rewardRoll             func() float64
	beijingLocation        *time.Location
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

func (s *CheckinService) GetStatus(ctx context.Context, userID int64) (*CheckinStatus, error) {
	checkinDate, nextReset := s.currentBeijingDay()
	blacklisted, err := s.isBlacklisted(ctx, userID)
	if err != nil {
		return nil, err
	}
	if blacklisted {
		return &CheckinStatus{
			Enabled:     false,
			Blacklisted: true,
			CheckedIn:   false,
			CheckinDate: checkinDate,
			NextResetAt: nextReset,
		}, nil
	}
	record, err := s.getCheckinByUserAndDate(ctx, userID, checkinDate)
	if err != nil {
		return nil, err
	}
	status := &CheckinStatus{
		Enabled:     true,
		Blacklisted: false,
		CheckedIn:   record != nil,
		CheckinDate: checkinDate,
		NextResetAt: nextReset,
	}
	if record != nil {
		status.RewardAmount = record.RewardAmount
		status.CheckedInAt = &record.CreatedAt
	}
	return status, nil
}

func (s *CheckinService) Checkin(ctx context.Context, userID int64) (*CheckinResult, error) {
	checkinDate, nextReset := s.currentBeijingDay()
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

	reward := checkinRewardForRoll(s.rewardRoll())
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
	return &CheckinResult{
		CheckinStatus: CheckinStatus{
			Enabled:      true,
			Blacklisted:  false,
			CheckedIn:    true,
			CheckinDate:  checkinDate,
			RewardAmount: reward,
			CheckedInAt:  &checkedInAt,
			NextResetAt:  nextReset,
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

func (s *CheckinService) currentBeijingDay() (string, time.Time) {
	now := s.now().In(s.beijingLocation)
	year, month, day := now.Date()
	todayStart := time.Date(year, month, day, 0, 0, 0, 0, s.beijingLocation)
	nextReset := todayStart.Add(24 * time.Hour)
	return todayStart.Format("2006-01-02"), nextReset
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
	checkedInAt := record.CreatedAt
	return &CheckinResult{
		CheckinStatus: CheckinStatus{
			Enabled:      true,
			Blacklisted:  false,
			CheckedIn:    true,
			CheckinDate:  record.CheckinDate,
			RewardAmount: record.RewardAmount,
			CheckedInAt:  &checkedInAt,
			NextResetAt:  nextReset,
		},
		AlreadyCheckedIn: true,
		BalanceBefore:    record.BalanceBefore,
		BalanceAfter:     latestBalance,
	}, nil
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
	switch {
	case roll < 0.5:
		return 2
	case roll < 0.8:
		return 3
	case roll < 0.95:
		return 4
	default:
		return 5
	}
}

func checkinRecordFromEntity(entity *dbent.UserCheckin) CheckinRecord {
	out := CheckinRecord{
		ID:            entity.ID,
		UserID:        entity.UserID,
		CheckinDate:   entity.CheckinDate,
		RewardAmount:  entity.RewardAmount,
		BalanceBefore: entity.BalanceBefore,
		BalanceAfter:  entity.BalanceAfter,
		CreatedAt:     entity.CreatedAt,
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
	if limit <= 0 {
		limit = 10
	}
	entities, err := s.entClient.UserCheckin.Query().
		Where(usercheckin.UserIDEQ(userID)).
		Order(dbent.Desc(usercheckin.FieldCreatedAt), dbent.Desc(usercheckin.FieldID)).
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
