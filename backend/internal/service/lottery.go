package service

import (
	"context"
	cryptoRand "crypto/rand"
	stderrors "errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
)

const (
	LotteryActivityStatusDraft    = "draft"
	LotteryActivityStatusActive   = "active"
	LotteryActivityStatusDisabled = "disabled"
	LotteryActivityStatusEnded    = "ended"

	LotteryAttemptModeDaily = "daily"
	LotteryAttemptModeTotal = "total"

	LotteryPrizeTypeBalance = "balance"
	LotteryPrizeTypeProduct = "product"

	LotteryPrizeItemStatusAvailable = "available"
	LotteryPrizeItemStatusClaimed   = "claimed"

	lotteryAttemptKeyMaxLength = 128
)

var (
	ErrLotteryDisabled            = infraerrors.Forbidden("LOTTERY_DISABLED", "lottery is disabled")
	ErrLotteryActivityNotFound    = infraerrors.NotFound("LOTTERY_ACTIVITY_NOT_FOUND", "no lottery activity is available")
	ErrLotteryAttemptsExhausted   = infraerrors.Conflict("LOTTERY_ATTEMPTS_EXHAUSTED", "lottery attempts are exhausted")
	ErrLotteryNoPrize             = infraerrors.Conflict("LOTTERY_NO_PRIZE", "no lottery prize is currently available")
	ErrLotteryProductUnavailable  = infraerrors.Conflict("LOTTERY_PRODUCT_UNAVAILABLE", "the lottery product is no longer available")
	ErrLotteryActivityInvalid     = infraerrors.BadRequest("LOTTERY_ACTIVITY_INVALID", "lottery activity configuration is invalid")
	ErrLotteryPrizeInvalid        = infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "lottery prize configuration is invalid")
	ErrLotteryAttemptKeyInvalid   = infraerrors.BadRequest("LOTTERY_ATTEMPT_KEY_INVALID", "lottery attempt key is invalid")
	ErrLotteryDrawNotFound        = infraerrors.NotFound("LOTTERY_DRAW_NOT_FOUND", "lottery draw not found")
	ErrLotteryConfigurationDenied = infraerrors.Forbidden("LOTTERY_CONFIGURATION_DENIED", "lottery configuration is not available")
)

// LotteryActivity is the single global lottery activity configuration.
type LotteryActivity struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	AttemptMode  string     `json:"attempt_mode"`
	AttemptLimit int        `json:"attempt_limit"`
	StartsAt     *time.Time `json:"starts_at,omitempty"`
	EndsAt       *time.Time `json:"ends_at,omitempty"`
	CreatedBy    *int64     `json:"created_by,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// LotteryPrize describes a weighted prize. Product content is kept out of this
// type so public responses never expose the inventory before it is won.
type LotteryPrize struct {
	ID                 int64     `json:"id"`
	ActivityID         int64     `json:"activity_id"`
	Name               string    `json:"name"`
	Description        string    `json:"description"`
	Type               string    `json:"type"`
	Weight             int       `json:"weight"`
	BalanceAmount      *float64  `json:"balance_amount,omitempty"`
	Enabled            bool      `json:"enabled"`
	SortOrder          int       `json:"sort_order"`
	AvailableItemCount int       `json:"available_item_count"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// LotteryPrizeItem is an individual single-use product/code inventory item.
type LotteryPrizeItem struct {
	ID        int64      `json:"id"`
	PrizeID   int64      `json:"prize_id"`
	Content   string     `json:"content,omitempty"`
	Status    string     `json:"status"`
	ClaimedBy *int64     `json:"claimed_by,omitempty"`
	ClaimedAt *time.Time `json:"claimed_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// LotteryDraw is the immutable reward snapshot shown in a user's history.
type LotteryDraw struct {
	ID             int64     `json:"id"`
	ActivityID     *int64    `json:"activity_id,omitempty"`
	PrizeID        *int64    `json:"prize_id,omitempty"`
	UserID         int64     `json:"user_id"`
	PrizeName      string    `json:"prize_name"`
	PrizeType      string    `json:"prize_type"`
	BalanceAmount  *float64  `json:"balance_amount,omitempty"`
	ProductContent *string   `json:"product_content,omitempty"`
	AttemptKey     string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
}

type LotteryActivityInput struct {
	Name         string     `json:"name"`
	Description  string     `json:"description"`
	Status       string     `json:"status"`
	AttemptMode  string     `json:"attempt_mode"`
	AttemptLimit int        `json:"attempt_limit"`
	StartsAt     *time.Time `json:"starts_at,omitempty"`
	EndsAt       *time.Time `json:"ends_at,omitempty"`
}

type LotteryPrizeInput struct {
	ID            int64    `json:"id,omitempty"`
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	Type          string   `json:"type"`
	Weight        int      `json:"weight"`
	BalanceAmount *float64 `json:"balance_amount,omitempty"`
	Enabled       bool     `json:"enabled"`
	SortOrder     int      `json:"sort_order"`
}

type LotteryActivityConfig struct {
	Activity *LotteryActivity `json:"activity,omitempty"`
	Prizes   []LotteryPrize   `json:"prizes"`
}

type LotteryPublicState struct {
	Activity          *LotteryActivity `json:"activity"`
	Prizes            []LotteryPrize   `json:"prizes"`
	AttemptsUsed      int              `json:"attempts_used"`
	AttemptsRemaining int              `json:"attempts_remaining"`
}

type LotteryDrawResult struct {
	Draw              *LotteryDraw `json:"draw"`
	AttemptsUsed      int          `json:"attempts_used"`
	AttemptsRemaining int          `json:"attempts_remaining"`
}

// LotteryRepository is deliberately expressed in service types. It lets the
// service own transaction boundaries while the repository keeps Ent details out
// of handlers and preserves row-locking in the draw path.
type LotteryRepository interface {
	GetActivity(ctx context.Context) (*LotteryActivity, error)
	GetActiveActivity(ctx context.Context, now time.Time, forUpdate bool) (*LotteryActivity, error)
	ListPrizes(ctx context.Context, activityID int64, includeInventory bool) ([]LotteryPrize, error)
	SaveActivity(ctx context.Context, activityID int64, input LotteryActivityInput, createdBy *int64) (*LotteryActivity, error)
	CreatePrize(ctx context.Context, activityID int64, input LotteryPrizeInput) (*LotteryPrize, error)
	UpdatePrize(ctx context.Context, input LotteryPrizeInput) (*LotteryPrize, error)
	DeletePrize(ctx context.Context, prizeID int64) error
	AppendPrizeItems(ctx context.Context, prizeID int64, contents []string) (int, error)
	ListPrizeItems(ctx context.Context, prizeID int64, includeContent bool) ([]LotteryPrizeItem, error)
	DeleteAvailablePrizeItems(ctx context.Context, prizeID int64, itemIDs []int64) (int, error)
	CountUserDraws(ctx context.Context, activityID, userID int64, since *time.Time) (int, error)
	GetDrawByAttemptKey(ctx context.Context, attemptKey string) (*LotteryDraw, error)
	ClaimAvailableProductItem(ctx context.Context, prizeID, userID int64, now time.Time) (*LotteryPrizeItem, error)
	CreateDraw(ctx context.Context, draw LotteryDraw) (*LotteryDraw, error)
	ListUserDraws(ctx context.Context, userID int64, offset, limit int) ([]LotteryDraw, int, error)
}

type LotteryService struct {
	entClient  *dbent.Client
	repo       LotteryRepository
	userRepo   UserRepository
	settingSvc *SettingService
}

func NewLotteryService(entClient *dbent.Client, repo LotteryRepository, userRepo UserRepository, settingSvc *SettingService) *LotteryService {
	return &LotteryService{entClient: entClient, repo: repo, userRepo: userRepo, settingSvc: settingSvc}
}

func (s *LotteryService) GetAdminConfig(ctx context.Context) (*LotteryActivityConfig, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLotteryConfigurationDenied
	}
	activity, err := s.repo.GetActivity(ctx)
	if err != nil && !stderrors.Is(err, ErrLotteryActivityNotFound) {
		return nil, err
	}
	if activity == nil {
		return &LotteryActivityConfig{Prizes: []LotteryPrize{}}, nil
	}
	prizes, err := s.repo.ListPrizes(ctx, activity.ID, true)
	if err != nil {
		return nil, err
	}
	return &LotteryActivityConfig{Activity: activity, Prizes: prizes}, nil
}

func (s *LotteryService) SaveActivity(ctx context.Context, activityID int64, input LotteryActivityInput, createdBy *int64) (*LotteryActivity, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLotteryConfigurationDenied
	}
	if err := validateLotteryActivity(input); err != nil {
		return nil, err
	}
	return s.repo.SaveActivity(ctx, activityID, input, createdBy)
}

func (s *LotteryService) CreatePrize(ctx context.Context, activityID int64, input LotteryPrizeInput) (*LotteryPrize, error) {
	if s == nil || s.repo == nil || activityID <= 0 {
		return nil, ErrLotteryConfigurationDenied
	}
	if err := validateLotteryPrize(input); err != nil {
		return nil, err
	}
	return s.repo.CreatePrize(ctx, activityID, input)
}

func (s *LotteryService) UpdatePrize(ctx context.Context, input LotteryPrizeInput) (*LotteryPrize, error) {
	if s == nil || s.repo == nil || input.ID <= 0 {
		return nil, ErrLotteryConfigurationDenied
	}
	if err := validateLotteryPrize(input); err != nil {
		return nil, err
	}
	return s.repo.UpdatePrize(ctx, input)
}

func (s *LotteryService) DeletePrize(ctx context.Context, prizeID int64) error {
	if s == nil || s.repo == nil || prizeID <= 0 {
		return ErrLotteryConfigurationDenied
	}
	return s.repo.DeletePrize(ctx, prizeID)
}

func (s *LotteryService) AppendPrizeItems(ctx context.Context, prizeID int64, contents []string) (int, error) {
	if s == nil || s.repo == nil || prizeID <= 0 {
		return 0, ErrLotteryConfigurationDenied
	}
	cleaned := make([]string, 0, len(contents))
	for _, raw := range contents {
		content := strings.TrimSpace(raw)
		if content != "" {
			cleaned = append(cleaned, content)
		}
	}
	if len(cleaned) == 0 {
		return 0, ErrLotteryPrizeInvalid
	}
	return s.repo.AppendPrizeItems(ctx, prizeID, cleaned)
}

func (s *LotteryService) ListPrizeItems(ctx context.Context, prizeID int64) ([]LotteryPrizeItem, error) {
	if s == nil || s.repo == nil || prizeID <= 0 {
		return nil, ErrLotteryConfigurationDenied
	}
	return s.repo.ListPrizeItems(ctx, prizeID, true)
}

func (s *LotteryService) DeleteAvailablePrizeItems(ctx context.Context, prizeID int64, itemIDs []int64) (int, error) {
	if s == nil || s.repo == nil || prizeID <= 0 {
		return 0, ErrLotteryConfigurationDenied
	}
	return s.repo.DeleteAvailablePrizeItems(ctx, prizeID, itemIDs)
}

func (s *LotteryService) GetPublicState(ctx context.Context, userID int64, now time.Time) (*LotteryPublicState, error) {
	if !s.lotteryEnabled(ctx) {
		return nil, ErrLotteryDisabled
	}
	activity, err := s.repo.GetActiveActivity(ctx, now, false)
	if err != nil {
		return nil, err
	}
	prizes, err := s.repo.ListPrizes(ctx, activity.ID, true)
	if err != nil {
		return nil, err
	}
	used, err := s.repo.CountUserDraws(ctx, activity.ID, userID, lotteryAttemptSince(activity.AttemptMode, now))
	if err != nil {
		return nil, err
	}
	return &LotteryPublicState{
		Activity:          activity,
		Prizes:            prizes,
		AttemptsUsed:      used,
		AttemptsRemaining: lotteryMaxInt(activity.AttemptLimit-used, 0),
	}, nil
}

func (s *LotteryService) Draw(ctx context.Context, userID int64, attemptKey string, now time.Time) (*LotteryDrawResult, error) {
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	key, err := normalizeLotteryAttemptKey(userID, attemptKey)
	if err != nil {
		return nil, err
	}
	if !s.lotteryEnabled(ctx) {
		return nil, ErrLotteryDisabled
	}
	if s.entClient == nil || s.repo == nil || s.userRepo == nil {
		return nil, ErrLotteryConfigurationDenied
	}

	var result *LotteryDrawResult
	err = s.withTx(ctx, func(txCtx context.Context) error {
		existing, lookupErr := s.repo.GetDrawByAttemptKey(txCtx, key)
		if lookupErr == nil {
			used, err := s.repo.CountUserDraws(txCtx, derefInt64(existing.ActivityID), userID, lotteryAttemptSince(LotteryAttemptModeTotal, now))
			if err != nil {
				return err
			}
			result = &LotteryDrawResult{Draw: existing, AttemptsUsed: used}
			return nil
		}
		if !stderrors.Is(lookupErr, ErrLotteryDrawNotFound) {
			return lookupErr
		}

		activity, err := s.repo.GetActiveActivity(txCtx, now, true)
		if err != nil {
			return err
		}
		since := lotteryAttemptSince(activity.AttemptMode, now)
		used, err := s.repo.CountUserDraws(txCtx, activity.ID, userID, since)
		if err != nil {
			return err
		}
		if used >= activity.AttemptLimit {
			return ErrLotteryAttemptsExhausted
		}

		prizes, err := s.repo.ListPrizes(txCtx, activity.ID, true)
		if err != nil {
			return err
		}
		totalWeight := lotteryPrizeWeightTotal(prizes)
		if totalWeight <= 0 {
			return ErrLotteryNoPrize
		}
		ticket, err := randomLotteryTicket(totalWeight)
		if err != nil {
			return fmt.Errorf("generate lottery ticket: %w", err)
		}
		prize, err := selectLotteryPrize(prizes, ticket)
		if err != nil {
			return err
		}

		draw := LotteryDraw{
			ActivityID: &activity.ID,
			PrizeID:    &prize.ID,
			UserID:     userID,
			PrizeName:  prize.Name,
			PrizeType:  prize.Type,
			AttemptKey: key,
			CreatedAt:  now,
		}
		if prize.Type == LotteryPrizeTypeBalance {
			if prize.BalanceAmount == nil {
				return ErrLotteryPrizeInvalid
			}
			if _, err := s.userRepo.AdjustBalance(txCtx, userID, *prize.BalanceAmount); err != nil {
				return fmt.Errorf("credit lottery balance prize: %w", err)
			}
			amount := *prize.BalanceAmount
			draw.BalanceAmount = &amount
		} else {
			item, err := s.repo.ClaimAvailableProductItem(txCtx, prize.ID, userID, now)
			if err != nil {
				return err
			}
			draw.ProductContent = &item.Content
		}
		created, err := s.repo.CreateDraw(txCtx, draw)
		if err != nil {
			return err
		}
		result = &LotteryDrawResult{
			Draw:              created,
			AttemptsUsed:      used + 1,
			AttemptsRemaining: lotteryMaxInt(activity.AttemptLimit-used-1, 0),
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *LotteryService) ListUserDraws(ctx context.Context, userID int64, offset, limit int) ([]LotteryDraw, int, error) {
	if userID <= 0 {
		return nil, 0, ErrUserNotFound
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListUserDraws(ctx, userID, offset, limit)
}

func (s *LotteryService) lotteryEnabled(ctx context.Context) bool {
	if s.settingSvc == nil {
		return false
	}
	settings, err := s.settingSvc.GetPublicSettings(ctx)
	return err == nil && settings != nil && settings.LotteryEnabled
}

func (s *LotteryService) withTx(ctx context.Context, fn func(context.Context) error) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin lottery transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	txCtx := dbent.NewTxContext(ctx, tx)
	if err := fn(txCtx); err != nil {
		return err
	}
	return tx.Commit()
}

func lotteryAttemptSince(mode string, now time.Time) *time.Time {
	if mode != LotteryAttemptModeDaily {
		return nil
	}
	start := timezone.StartOfDay(now)
	return &start
}

func lotteryAttemptStart(mode string, now time.Time) time.Time {
	if since := lotteryAttemptSince(mode, now); since != nil {
		return *since
	}
	return time.Time{}
}

func lotteryPrizeWeightTotal(prizes []LotteryPrize) int64 {
	var total int64
	for _, prize := range prizes {
		if !isLotteryPrizeAvailable(prize) || prize.Weight <= 0 {
			continue
		}
		if total > math.MaxInt64-int64(prize.Weight) {
			return 0
		}
		total += int64(prize.Weight)
	}
	return total
}

func selectLotteryPrize(prizes []LotteryPrize, ticket int64) (LotteryPrize, error) {
	total := lotteryPrizeWeightTotal(prizes)
	if total <= 0 || ticket < 0 || ticket >= total {
		return LotteryPrize{}, ErrLotteryNoPrize
	}
	var cursor int64
	for _, prize := range prizes {
		if !isLotteryPrizeAvailable(prize) || prize.Weight <= 0 {
			continue
		}
		cursor += int64(prize.Weight)
		if ticket < cursor {
			return prize, nil
		}
	}
	return LotteryPrize{}, ErrLotteryNoPrize
}

func isLotteryPrizeAvailable(prize LotteryPrize) bool {
	if !prize.Enabled {
		return false
	}
	switch prize.Type {
	case LotteryPrizeTypeBalance:
		return prize.BalanceAmount != nil && *prize.BalanceAmount > 0
	case LotteryPrizeTypeProduct:
		return prize.AvailableItemCount > 0
	default:
		return false
	}
}

func validateLotteryActivity(input LotteryActivityInput) error {
	if strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 120 {
		return ErrLotteryActivityInvalid
	}
	if input.Status != LotteryActivityStatusDraft && input.Status != LotteryActivityStatusActive && input.Status != LotteryActivityStatusDisabled && input.Status != LotteryActivityStatusEnded {
		return ErrLotteryActivityInvalid
	}
	if input.AttemptMode != LotteryAttemptModeDaily && input.AttemptMode != LotteryAttemptModeTotal {
		return ErrLotteryActivityInvalid
	}
	if input.AttemptLimit <= 0 {
		return ErrLotteryActivityInvalid
	}
	if input.StartsAt != nil && input.EndsAt != nil && input.StartsAt.After(*input.EndsAt) {
		return ErrLotteryActivityInvalid
	}
	return nil
}

func validateLotteryPrize(input LotteryPrizeInput) error {
	if strings.TrimSpace(input.Name) == "" || len([]rune(input.Name)) > 120 || input.Weight <= 0 {
		return ErrLotteryPrizeInvalid
	}
	switch input.Type {
	case LotteryPrizeTypeBalance:
		if input.BalanceAmount == nil || *input.BalanceAmount <= 0 {
			return ErrLotteryPrizeInvalid
		}
	case LotteryPrizeTypeProduct:
		if input.BalanceAmount != nil {
			return ErrLotteryPrizeInvalid
		}
	default:
		return ErrLotteryPrizeInvalid
	}
	return nil
}

func normalizeLotteryAttemptKey(userID int64, raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" || len(key) > lotteryAttemptKeyMaxLength || userID <= 0 {
		return "", ErrLotteryAttemptKeyInvalid
	}
	return fmt.Sprintf("%d:%s", userID, key), nil
}

func randomLotteryTicket(total int64) (int64, error) {
	n, err := cryptoRand.Int(cryptoRand.Reader, big.NewInt(total))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

func lotteryMaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
