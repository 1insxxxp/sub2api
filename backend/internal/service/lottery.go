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

	LotteryAttemptSourceActivity = "activity"
	LotteryAttemptSourceWallet   = "wallet"

	// LotteryAttemptGrantMaxUsers bounds explicit user selections. The all-users
	// mode intentionally operates on the complete non-deleted user set.
	LotteryAttemptGrantMaxUsers = 5000

	LotteryPrizeTypeBalance = "balance"
	LotteryPrizeTypeProduct = "product"

	LotteryPrizeItemStatusAvailable = "available"
	LotteryPrizeItemStatusClaimed   = "claimed"

	lotteryAttemptKeyMaxLength       = 128
	lotteryAttemptGrantMaxAmount     = 1_000_000
	lotteryAttemptGrantMaxNoteLength = 500
)

var (
	ErrLotteryDisabled             = infraerrors.Forbidden("LOTTERY_DISABLED", "lottery is disabled")
	ErrLotteryActivityNotFound     = infraerrors.NotFound("LOTTERY_ACTIVITY_NOT_FOUND", "no lottery activity is available")
	ErrLotteryAttemptsExhausted    = infraerrors.Conflict("LOTTERY_ATTEMPTS_EXHAUSTED", "lottery attempts are exhausted")
	ErrLotteryNoPrize              = infraerrors.Conflict("LOTTERY_NO_PRIZE", "no lottery prize is currently available")
	ErrLotteryProductUnavailable   = infraerrors.Conflict("LOTTERY_PRODUCT_UNAVAILABLE", "the lottery product is no longer available")
	ErrLotteryActivityInvalid      = infraerrors.BadRequest("LOTTERY_ACTIVITY_INVALID", "lottery activity configuration is invalid")
	ErrLotteryPrizeInvalid         = infraerrors.BadRequest("LOTTERY_PRIZE_INVALID", "lottery prize configuration is invalid")
	ErrLotteryAttemptKeyInvalid    = infraerrors.BadRequest("LOTTERY_ATTEMPT_KEY_INVALID", "lottery attempt key is invalid")
	ErrLotteryAttemptGrantInvalid  = infraerrors.BadRequest("LOTTERY_ATTEMPT_GRANT_INVALID", "lottery attempt grant is invalid")
	ErrLotteryAttemptGrantTooLarge = infraerrors.BadRequest("LOTTERY_ATTEMPT_GRANT_TOO_LARGE", "lottery attempt grant targets too many users")
	ErrLotteryAttemptGrantConflict = infraerrors.Conflict("LOTTERY_ATTEMPT_GRANT_CONFLICT", "lottery attempt grant request key was already used with different parameters")
	ErrLotteryDrawNotFound         = infraerrors.NotFound("LOTTERY_DRAW_NOT_FOUND", "lottery draw not found")
	ErrLotteryConfigurationDenied  = infraerrors.Forbidden("LOTTERY_CONFIGURATION_DENIED", "lottery configuration is not available")
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
	AttemptSource  string    `json:"attempt_source"`
	CreatedAt      time.Time `json:"created_at"`
}

// LotteryAdminDraw is the audit view of a completed draw. It intentionally
// omits the idempotency attempt key while including the user identity and the
// immutable reward snapshot needed by administrators.
type LotteryAdminDraw struct {
	ID             int64     `json:"id"`
	ActivityID     *int64    `json:"activity_id,omitempty"`
	PrizeID        *int64    `json:"prize_id,omitempty"`
	UserID         int64     `json:"user_id"`
	UserEmail      string    `json:"user_email,omitempty"`
	UserName       string    `json:"user_name,omitempty"`
	UserDeleted    bool      `json:"user_deleted"`
	PrizeName      string    `json:"prize_name"`
	PrizeType      string    `json:"prize_type"`
	BalanceAmount  *float64  `json:"balance_amount,omitempty"`
	ProductContent *string   `json:"product_content,omitempty"`
	AttemptSource  string    `json:"attempt_source"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewLotteryAdminDraw converts the internal draw snapshot and a user record to
// the administrator-facing audit representation. A nil or soft-deleted user
// remains identifiable by ID without leaking any stale identity fields.
func NewLotteryAdminDraw(draw LotteryDraw, user *User) LotteryAdminDraw {
	adminDraw := LotteryAdminDraw{
		ID: draw.ID, ActivityID: draw.ActivityID, PrizeID: draw.PrizeID,
		UserID: draw.UserID, PrizeName: draw.PrizeName, PrizeType: draw.PrizeType,
		BalanceAmount: draw.BalanceAmount, ProductContent: draw.ProductContent,
		AttemptSource: draw.AttemptSource, CreatedAt: draw.CreatedAt,
		UserDeleted: user == nil,
	}
	if user != nil {
		adminDraw.UserEmail = user.Email
		adminDraw.UserName = user.Username
		adminDraw.UserDeleted = user.DeletedAt != nil
		if adminDraw.UserDeleted {
			adminDraw.UserEmail = ""
			adminDraw.UserName = ""
		}
	}
	return adminDraw
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
	Activity                  *LotteryActivity `json:"activity"`
	Prizes                    []LotteryPrize   `json:"prizes"`
	AttemptsUsed              int              `json:"attempts_used"`
	ActivityAttemptsRemaining int              `json:"activity_attempts_remaining"`
	RewardAttemptsRemaining   int              `json:"reward_attempts_remaining"`
	AttemptsRemaining         int              `json:"attempts_remaining"`
}

type LotteryDrawResult struct {
	Draw                      *LotteryDraw `json:"draw"`
	AttemptsUsed              int          `json:"attempts_used"`
	ActivityAttemptsRemaining int          `json:"activity_attempts_remaining"`
	RewardAttemptsRemaining   int          `json:"reward_attempts_remaining"`
	AttemptsRemaining         int          `json:"attempts_remaining"`
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

// LotteryAdminDrawRepository is an optional capability implemented by the
// production repository for the administrator-only audit view. Keeping it
// separate preserves compatibility with existing user-facing repository
// adapters and test doubles.
type LotteryAdminDrawRepository interface {
	ListAdminDraws(ctx context.Context, offset, limit int) ([]LotteryAdminDraw, int, error)
}

// LotteryAdminAttemptBalance describes the current draw-attempt balances for
// one non-deleted user. Activity attempts are calculated from the active
// activity policy; reward attempts come from the persistent wallet.
type LotteryAdminAttemptBalance struct {
	UserID            int64  `json:"user_id"`
	UserEmail         string `json:"user_email"`
	UserName          string `json:"user_name"`
	UserStatus        string `json:"user_status"`
	ActivityRemaining int    `json:"activity_remaining"`
	RewardRemaining   int    `json:"reward_remaining"`
	TotalRemaining    int    `json:"total_remaining"`
}

// LotteryAdminAttemptBalanceQuery contains the bounded user-list query and
// the active-activity policy used to calculate each row.
type LotteryAdminAttemptBalanceQuery struct {
	Offset        int
	Limit         int
	Search        string
	ActivityID    int64
	ActivityLimit int
	ActivitySince *time.Time
}

// LotteryAdminAttemptBalanceRepository is an optional administrator read
// capability. It keeps the user list and balance aggregation out of the
// generic lottery repository contract used by user-facing adapters.
type LotteryAdminAttemptBalanceRepository interface {
	ListAdminAttemptBalances(ctx context.Context, query LotteryAdminAttemptBalanceQuery) ([]LotteryAdminAttemptBalance, int, error)
}

// LotteryAttemptGrantInput describes one administrator request to credit
// reward-wallet attempts to explicit users or every non-deleted user.
type LotteryAttemptGrantInput struct {
	All         bool    `json:"all"`
	UserIDs     []int64 `json:"user_ids"`
	Amount      int     `json:"amount"`
	Description string  `json:"description"`
	RequestKey  string  `json:"request_key"`
	CreatedBy   int64   `json:"-"`
}

// LotteryAttemptGrantResult summarizes a successful batch grant.
type LotteryAttemptGrantResult struct {
	Affected     int `json:"affected"`
	TotalGranted int `json:"total_granted"`
}

// LotteryAdminAttemptRepository is an optional administrator capability. It
// is separate from LotteryRepository so existing adapters and test doubles do
// not need to implement administrative mutations.
type LotteryAdminAttemptRepository interface {
	GrantLotteryAttempts(ctx context.Context, input LotteryAttemptGrantInput) (LotteryAttemptGrantResult, error)
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
	rewardBalance, err := lotteryAttemptBalance(ctx, s.entClient, userID)
	if err != nil {
		return nil, err
	}
	attempts := summarizeLotteryAttempts(activity.AttemptLimit, used, rewardBalance)
	return &LotteryPublicState{
		Activity:                  activity,
		Prizes:                    prizes,
		AttemptsUsed:              used,
		ActivityAttemptsRemaining: attempts.ActivityRemaining,
		RewardAttemptsRemaining:   attempts.RewardRemaining,
		AttemptsRemaining:         attempts.TotalRemaining,
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
			activity, err := s.repo.GetActiveActivity(txCtx, now, true)
			if err != nil {
				return err
			}
			used, err := s.repo.CountUserDraws(txCtx, activity.ID, userID, lotteryAttemptSince(activity.AttemptMode, now))
			if err != nil {
				return err
			}
			rewardBalance, err := lotteryAttemptBalance(txCtx, s.entClient, userID)
			if err != nil {
				return err
			}
			attempts := summarizeLotteryAttempts(activity.AttemptLimit, used, rewardBalance)
			result = lotteryDrawResult(existing, used, attempts)
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
		rewardBalance, err := lotteryAttemptBalance(txCtx, s.entClient, userID)
		if err != nil {
			return err
		}
		attempts := summarizeLotteryAttempts(activity.AttemptLimit, used, rewardBalance)
		if attempts.TotalRemaining <= 0 {
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
			ActivityID:    &activity.ID,
			PrizeID:       &prize.ID,
			UserID:        userID,
			PrizeName:     prize.Name,
			PrizeType:     prize.Type,
			AttemptKey:    key,
			AttemptSource: attempts.NextSource,
			CreatedAt:     now,
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
		if attempts.NextSource == LotteryAttemptSourceWallet {
			rewardBalance, err = debitLotteryAttempt(txCtx, s.entClient, userID, created.ID, "lottery draw")
			if err != nil {
				return err
			}
		} else {
			used++
		}
		attempts = summarizeLotteryAttempts(activity.AttemptLimit, used, rewardBalance)
		result = lotteryDrawResult(created, used, attempts)
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

// ListAdminDraws returns the newest lottery draws for the administrator audit
// page. The repository performs the draw/user lookup in a bounded page.
func (s *LotteryService) ListAdminDraws(ctx context.Context, offset, limit int) ([]LotteryAdminDraw, int, error) {
	if s == nil || s.repo == nil {
		return nil, 0, ErrLotteryConfigurationDenied
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
	adminRepo, ok := s.repo.(LotteryAdminDrawRepository)
	if !ok {
		return nil, 0, ErrLotteryConfigurationDenied
	}
	return adminRepo.ListAdminDraws(ctx, offset, limit)
}

// ListAdminAttemptBalances returns paginated current attempt balances for
// non-deleted users. The active activity is optional: when there is no active
// activity, every row simply has zero activity attempts and only wallet
// attempts remain available.
func (s *LotteryService) ListAdminAttemptBalances(ctx context.Context, page, pageSize int, search string, now time.Time) ([]LotteryAdminAttemptBalance, int, error) {
	if s == nil || s.repo == nil {
		return nil, 0, ErrLotteryConfigurationDenied
	}
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	adminRepo, ok := s.repo.(LotteryAdminAttemptBalanceRepository)
	if !ok {
		return nil, 0, ErrLotteryConfigurationDenied
	}
	query := LotteryAdminAttemptBalanceQuery{
		Offset: (page - 1) * pageSize,
		Limit:  pageSize,
		Search: strings.TrimSpace(search),
	}
	activity, err := s.repo.GetActiveActivity(ctx, now, false)
	if err != nil && !stderrors.Is(err, ErrLotteryActivityNotFound) {
		return nil, 0, err
	}
	if activity != nil {
		query.ActivityID = activity.ID
		query.ActivityLimit = activity.AttemptLimit
		query.ActivitySince = lotteryAttemptSince(activity.AttemptMode, now)
	}
	return adminRepo.ListAdminAttemptBalances(ctx, query)
}

// GrantLotteryAttempts credits reward-wallet attempts to selected users or all
// non-deleted users. The production repository performs the whole batch in one
// transaction and records an audit row per target user.
func (s *LotteryService) GrantLotteryAttempts(ctx context.Context, input LotteryAttemptGrantInput) (*LotteryAttemptGrantResult, error) {
	if s == nil || s.repo == nil {
		return nil, ErrLotteryConfigurationDenied
	}
	input.UserIDs = normalizeLotteryAttemptGrantUserIDs(input.UserIDs)
	input.Description = strings.TrimSpace(input.Description)
	input.RequestKey = strings.TrimSpace(input.RequestKey)
	if err := validateLotteryAttemptGrant(input); err != nil {
		return nil, err
	}
	adminRepo, ok := s.repo.(LotteryAdminAttemptRepository)
	if !ok {
		return nil, ErrLotteryConfigurationDenied
	}
	result, err := adminRepo.GrantLotteryAttempts(ctx, input)
	if err != nil {
		return nil, err
	}
	return &result, nil
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
	if input.AttemptLimit < 0 {
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

func validateLotteryAttemptGrant(input LotteryAttemptGrantInput) error {
	if input.CreatedBy <= 0 || input.Amount <= 0 || input.Amount > lotteryAttemptGrantMaxAmount {
		return ErrLotteryAttemptGrantInvalid
	}
	if input.RequestKey == "" || len(input.RequestKey) > lotteryAttemptKeyMaxLength {
		return ErrLotteryAttemptGrantInvalid
	}
	if len([]rune(strings.TrimSpace(input.Description))) > lotteryAttemptGrantMaxNoteLength {
		return ErrLotteryAttemptGrantInvalid
	}
	if (input.All && len(input.UserIDs) > 0) || (!input.All && len(input.UserIDs) == 0) {
		return ErrLotteryAttemptGrantInvalid
	}
	if !input.All && len(input.UserIDs) > LotteryAttemptGrantMaxUsers {
		return ErrLotteryAttemptGrantTooLarge
	}
	for _, userID := range input.UserIDs {
		if userID <= 0 {
			return ErrLotteryAttemptGrantInvalid
		}
	}
	return nil
}

func normalizeLotteryAttemptGrantUserIDs(userIDs []int64) []int64 {
	if len(userIDs) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(userIDs))
	result := make([]int64, 0, len(userIDs))
	for _, userID := range userIDs {
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}
		result = append(result, userID)
	}
	return result
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

type lotteryAttemptSummary struct {
	ActivityRemaining int
	RewardRemaining   int
	TotalRemaining    int
	NextSource        string
}

func summarizeLotteryAttempts(activityLimit, activityUsed, rewardBalance int) lotteryAttemptSummary {
	activityRemaining := lotteryMaxInt(activityLimit-activityUsed, 0)
	rewardRemaining := lotteryMaxInt(rewardBalance, 0)
	summary := lotteryAttemptSummary{
		ActivityRemaining: activityRemaining,
		RewardRemaining:   rewardRemaining,
		TotalRemaining:    activityRemaining + rewardRemaining,
	}
	if activityRemaining > 0 {
		summary.NextSource = LotteryAttemptSourceActivity
	} else if rewardRemaining > 0 {
		summary.NextSource = LotteryAttemptSourceWallet
	}
	return summary
}

func lotteryDrawResult(draw *LotteryDraw, used int, attempts lotteryAttemptSummary) *LotteryDrawResult {
	return &LotteryDrawResult{
		Draw:                      draw,
		AttemptsUsed:              used,
		ActivityAttemptsRemaining: attempts.ActivityRemaining,
		RewardAttemptsRemaining:   attempts.RewardRemaining,
		AttemptsRemaining:         attempts.TotalRemaining,
	}
}

func derefInt64(value *int64) int64 {
	if value == nil {
		return 0
	}
	return *value
}
