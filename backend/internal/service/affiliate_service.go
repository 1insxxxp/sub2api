package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

var (
	ErrAffiliateProfileNotFound             = infraerrors.NotFound("AFFILIATE_PROFILE_NOT_FOUND", "affiliate profile not found")
	ErrAffiliateCodeInvalid                 = infraerrors.BadRequest("AFFILIATE_CODE_INVALID", "invalid affiliate code")
	ErrAffiliateCodeTaken                   = infraerrors.Conflict("AFFILIATE_CODE_TAKEN", "affiliate code already in use")
	ErrAffiliateAlreadyBound                = infraerrors.Conflict("AFFILIATE_ALREADY_BOUND", "affiliate inviter already bound")
	ErrAffiliateQuotaEmpty                  = infraerrors.BadRequest("AFFILIATE_QUOTA_EMPTY", "no affiliate quota available to transfer")
	ErrAffiliateQualificationReconcileBusy  = errors.New("affiliate qualification reconciliation is busy")
	ErrAffiliateQualificationReconcileStale = errors.New("affiliate qualification reconciliation generation changed")
)

const (
	affiliateInviteesLimit = 100
	// AffiliateCodeMinLength / AffiliateCodeMaxLength bound both system-generated
	// 12-char codes and admin-customized codes (e.g. "VIP2026").
	AffiliateCodeMinLength                       = 4
	AffiliateCodeMaxLength                       = 32
	AffiliateQualificationDirtyAuditAction       = "AFFILIATE_QUALIFICATION_DIRTY"
	AffiliateQualificationDirtyFailedAuditAction = "AFFILIATE_QUALIFICATION_DIRTY_FAILED"
)

// affiliateCodeValidChar accepts uppercase letters, digits, underscore and dash.
// All input passes through strings.ToUpper before validation, so lowercase from
// users is normalized — admins may supply mixed case in their UI.
var affiliateCodeValidChar = func() [256]bool {
	var tbl [256]bool
	for c := byte('A'); c <= 'Z'; c++ {
		tbl[c] = true
	}
	for c := byte('0'); c <= '9'; c++ {
		tbl[c] = true
	}
	tbl['_'] = true
	tbl['-'] = true
	return tbl
}()

// isValidAffiliateCodeFormat validates code format for both binding (user input)
// and admin updates. Caller is expected to upper-case the input first.
func isValidAffiliateCodeFormat(code string) bool {
	if len(code) < AffiliateCodeMinLength || len(code) > AffiliateCodeMaxLength {
		return false
	}
	for i := 0; i < len(code); i++ {
		if !affiliateCodeValidChar[code[i]] {
			return false
		}
	}
	return true
}

type AffiliateSummary struct {
	UserID                  int64      `json:"user_id"`
	AffCode                 string     `json:"aff_code"`
	AffCodeCustom           bool       `json:"aff_code_custom"`
	AffRebateRatePercent    *float64   `json:"aff_rebate_rate_percent,omitempty"`
	InviterID               *int64     `json:"inviter_id,omitempty"`
	AffCount                int        `json:"aff_count"`
	AffQuota                float64    `json:"aff_quota"`
	AffFrozenQuota          float64    `json:"aff_frozen_quota"`
	AffHistoryQuota         float64    `json:"aff_history_quota"`
	QualifyingPaymentAmount float64    `json:"-"`
	QualifiedAt             *time.Time `json:"-"`
	CreatedAt               time.Time  `json:"created_at"`
	UpdatedAt               time.Time  `json:"updated_at"`
}

type AffiliateInvitee struct {
	UserID                  int64      `json:"user_id"`
	Email                   string     `json:"email"`
	Username                string     `json:"username"`
	CreatedAt               *time.Time `json:"created_at,omitempty"`
	TotalRebate             float64    `json:"total_rebate"`
	QualifyingPaymentAmount float64    `json:"qualifying_payment_amount"`
	Qualified               bool       `json:"qualified"`
	QualifiedAt             *time.Time `json:"qualified_at"`
}

type AffiliateQualification struct {
	InviteeUserID           int64      `json:"-"`
	QualifyingPaymentAmount float64    `json:"-"`
	QualifiedAt             *time.Time `json:"-"`
}

type AffiliateReconcilePendingSnapshot struct {
	Required   bool
	Generation int64
}

type AffiliateReconcileToken struct {
	Generation       int64
	WasPendingBefore bool
}

type AffiliateDetail struct {
	UserID                     int64                     `json:"user_id"`
	AffCode                    string                    `json:"aff_code"`
	InviterID                  *int64                    `json:"inviter_id,omitempty"`
	AffCount                   int                       `json:"aff_count"`
	AffQuota                   float64                   `json:"aff_quota"`
	AffFrozenQuota             float64                   `json:"aff_frozen_quota"`
	AffHistoryQuota            float64                   `json:"aff_history_quota"`
	AutomaticLevel             AffiliateTier             `json:"automatic_level"`
	AutomaticRebateRatePercent float64                   `json:"automatic_rebate_rate_percent"`
	EffectiveRebateRatePercent float64                   `json:"effective_rebate_rate_percent"`
	HasCustomRebateRate        bool                      `json:"has_custom_rebate_rate"`
	CustomRebateRatePercent    *float64                  `json:"custom_rebate_rate_percent"`
	QualifiedInviteeCount      int                       `json:"qualified_invitee_count"`
	QualificationAmount        float64                   `json:"qualification_amount"`
	NextLevelInviteeThreshold  *int                      `json:"next_level_invitee_threshold"`
	RemainingQualifiedInvitees int                       `json:"remaining_qualified_invitees"`
	Tiers                      []AffiliateTierDefinition `json:"tiers"`
	Invitees                   []AffiliateInvitee        `json:"invitees"`
}

type AffiliateTierDefinition struct {
	Level                AffiliateTier `json:"level"`
	MinQualifiedInvitees int           `json:"min_qualified_invitees"`
	RatePercent          float64       `json:"rate_percent"`
}

type AffiliateTierSnapshot struct {
	Level                 AffiliateTier `json:"level"`
	AutomaticRatePercent  float64       `json:"automatic_rate_percent"`
	QualifiedInviteeCount int           `json:"qualified_invitee_count"`
	NextTierThreshold     int           `json:"next_tier_threshold"`
	RemainingToNextTier   int           `json:"remaining_to_next_tier"`
}

type AffiliateRepository interface {
	EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error)
	GetAffiliateByCode(ctx context.Context, code string) (*AffiliateSummary, error)
	BindInviter(ctx context.Context, userID, inviterID int64) (bool, error)
	AccrueQuota(ctx context.Context, inviterID, inviteeUserID int64, amount float64, freezeHours int, sourceOrderID *int64) (bool, error)
	GetAccruedRebateFromInvitee(ctx context.Context, inviterID, inviteeUserID int64) (float64, error)
	ThawFrozenQuota(ctx context.Context, userID int64) (float64, error)
	TransferQuotaToBalance(ctx context.Context, userID int64) (float64, float64, error)
	ListInvitees(ctx context.Context, inviterID int64, limit int) ([]AffiliateInvitee, error)

	// 管理端：用户级专属配置
	UpdateUserAffCode(ctx context.Context, userID int64, newCode string) error
	ResetUserAffCode(ctx context.Context, userID int64) (string, error)
	SetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error
	BatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error
	ListUsersWithCustomSettings(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error)
	ListAffiliateInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error)
	ListAffiliateRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error)
	ListAffiliateTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error)
	GetAffiliateUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error)
}

type AffiliateTierReadRepository interface {
	ListInviteesWithQualification(ctx context.Context, inviterID int64, limit int, qualificationAmount float64) ([]AffiliateInvitee, error)
	ListAffiliateInviteRecordsWithQualification(ctx context.Context, filter AffiliateRecordFilter, qualificationAmount float64) ([]AffiliateInviteRecord, int64, error)
	GetAffiliateUserOverviewWithQualification(ctx context.Context, userID int64, qualificationAmount float64) (*AffiliateUserOverview, error)
}

type AffiliateQualificationRepository interface {
	ReconcileInviteeQualification(ctx context.Context, inviteeUserID int64, threshold float64) (*AffiliateQualification, error)
	CountQualifiedInvitees(ctx context.Context, inviterID int64, threshold float64) (int, error)
	ReconcileAllAffiliateQualifications(ctx context.Context, threshold float64, batchSize int) error
	TryWithAffiliateQualificationReconcileLock(ctx context.Context, fn func(context.Context) error) (bool, error)
	MarkReconcileRequired(ctx context.Context) (AffiliateReconcileToken, error)
	ReadReconcilePendingSnapshot(ctx context.Context) (AffiliateReconcilePendingSnapshot, error)
	ClearReconcileRequiredIfGeneration(ctx context.Context, expected int64) (bool, error)
	ListAffiliateQualificationDirtyEvents(ctx context.Context, limit int) ([]AffiliateQualificationDirtyEvent, error)
	DeleteAffiliateQualificationDirtyEvent(ctx context.Context, event AffiliateQualificationDirtyEvent) (bool, error)
	MarkAffiliateQualificationDirtyEventFailed(ctx context.Context, event AffiliateQualificationDirtyEvent, cause error) error
}

type AffiliateQualificationReadReconciler interface {
	ReconcileInviterInvitees(ctx context.Context, inviterID int64, threshold float64) error
	ReconcileInvitees(ctx context.Context, inviteeUserIDs []int64, threshold float64) error
}

type AffiliateQualificationDirtyEvent struct {
	OrderID     string `json:"-"`
	UserID      int64  `json:"userID"`
	OrderStatus string `json:"orderStatus"`
	EventType   string `json:"eventType"`
	Detail      string `json:"-"`
	ParseError  string `json:"-"`
}

// AffiliateAdminFilter 列表筛选条件
type AffiliateAdminFilter struct {
	Search   string
	Page     int
	PageSize int
}

// AffiliateAdminEntry 专属用户列表条目
type AffiliateAdminEntry struct {
	UserID               int64    `json:"user_id"`
	Email                string   `json:"email"`
	Username             string   `json:"username"`
	AffCode              string   `json:"aff_code"`
	AffCodeCustom        bool     `json:"aff_code_custom"`
	AffRebateRatePercent *float64 `json:"aff_rebate_rate_percent,omitempty"`
	AffCount             int      `json:"aff_count"`
}

type AffiliateRecordFilter struct {
	Search   string
	Page     int
	PageSize int
	StartAt  *time.Time
	EndAt    *time.Time
	SortBy   string
	SortDesc bool
}

type AffiliateInviteRecord struct {
	InviterID                  int64         `json:"inviter_id"`
	InviterEmail               string        `json:"inviter_email"`
	InviterUsername            string        `json:"inviter_username"`
	InviteeID                  int64         `json:"invitee_id"`
	InviteeEmail               string        `json:"invitee_email"`
	InviteeUsername            string        `json:"invitee_username"`
	AffCode                    string        `json:"aff_code"`
	TotalRebate                float64       `json:"total_rebate"`
	QualifyingPaymentAmount    float64       `json:"qualifying_payment_amount"`
	Qualified                  bool          `json:"qualified"`
	QualifiedAt                *time.Time    `json:"qualified_at"`
	InvitedCount               int           `json:"invited_count"`
	QualifiedInviteeCount      int           `json:"qualified_invitee_count"`
	AutomaticLevel             AffiliateTier `json:"automatic_level"`
	AutomaticRebateRatePercent float64       `json:"automatic_rebate_rate_percent"`
	CustomRebateRatePercent    *float64      `json:"custom_rebate_rate_percent"`
	EffectiveRebateRatePercent float64       `json:"effective_rebate_rate_percent"`
	CreatedAt                  time.Time     `json:"created_at"`
}

type AffiliateRebateRecord struct {
	OrderID         int64     `json:"order_id"`
	OutTradeNo      string    `json:"out_trade_no"`
	InviterID       int64     `json:"inviter_id"`
	InviterEmail    string    `json:"inviter_email"`
	InviterUsername string    `json:"inviter_username"`
	InviteeID       int64     `json:"invitee_id"`
	InviteeEmail    string    `json:"invitee_email"`
	InviteeUsername string    `json:"invitee_username"`
	OrderAmount     float64   `json:"order_amount"`
	PayAmount       float64   `json:"pay_amount"`
	RebateAmount    float64   `json:"rebate_amount"`
	PaymentType     string    `json:"payment_type"`
	OrderStatus     string    `json:"order_status"`
	CreatedAt       time.Time `json:"created_at"`
}

type AffiliateTransferRecord struct {
	LedgerID            int64     `json:"ledger_id"`
	UserID              int64     `json:"user_id"`
	UserEmail           string    `json:"user_email"`
	Username            string    `json:"username"`
	Amount              float64   `json:"amount"`
	BalanceAfter        *float64  `json:"balance_after,omitempty"`
	AvailableQuotaAfter *float64  `json:"available_quota_after,omitempty"`
	FrozenQuotaAfter    *float64  `json:"frozen_quota_after,omitempty"`
	HistoryQuotaAfter   *float64  `json:"history_quota_after,omitempty"`
	SnapshotAvailable   bool      `json:"snapshot_available"`
	CurrentBalance      float64   `json:"-"`
	RemainingQuota      float64   `json:"-"`
	FrozenQuota         float64   `json:"-"`
	HistoryQuota        float64   `json:"-"`
	CreatedAt           time.Time `json:"created_at"`
}

type AffiliateUserOverview struct {
	UserID                     int64         `json:"user_id"`
	Email                      string        `json:"email"`
	Username                   string        `json:"username"`
	AffCode                    string        `json:"aff_code"`
	RebateRatePercent          float64       `json:"rebate_rate_percent"`
	RebateRateCustom           bool          `json:"-"`
	InvitedCount               int           `json:"invited_count"`
	RebatedInviteeCount        int           `json:"rebated_invitee_count"`
	AvailableQuota             float64       `json:"available_quota"`
	HistoryQuota               float64       `json:"history_quota"`
	AutomaticLevel             AffiliateTier `json:"automatic_level"`
	AutomaticRebateRatePercent float64       `json:"automatic_rebate_rate_percent"`
	EffectiveRebateRatePercent float64       `json:"effective_rebate_rate_percent"`
	HasCustomRebateRate        bool          `json:"has_custom_rebate_rate"`
	CustomRebateRatePercent    *float64      `json:"custom_rebate_rate_percent"`
	QualifiedInviteeCount      int           `json:"qualified_invitee_count"`
	QualificationAmount        float64       `json:"qualification_amount"`
	NextLevelInviteeThreshold  *int          `json:"next_level_invitee_threshold"`
	RemainingQualifiedInvitees int           `json:"remaining_qualified_invitees"`
}

type AffiliateService struct {
	repo                  AffiliateRepository
	tierReadRepo          AffiliateTierReadRepository
	qualificationRepo     AffiliateQualificationRepository
	qualificationReadRepo AffiliateQualificationReadReconciler
	settingService        *SettingService
	authCacheInvalidator  APIKeyAuthCacheInvalidator
	billingCacheService   *BillingCacheService
}

func NewAffiliateService(repo AffiliateRepository, settingService *SettingService, authCacheInvalidator APIKeyAuthCacheInvalidator, billingCacheService *BillingCacheService) *AffiliateService {
	qualificationRepo, _ := repo.(AffiliateQualificationRepository)
	qualificationReadRepo, _ := repo.(AffiliateQualificationReadReconciler)
	tierReadRepo, _ := repo.(AffiliateTierReadRepository)
	return &AffiliateService{
		repo:                  repo,
		tierReadRepo:          tierReadRepo,
		qualificationRepo:     qualificationRepo,
		qualificationReadRepo: qualificationReadRepo,
		settingService:        settingService,
		authCacheInvalidator:  authCacheInvalidator,
		billingCacheService:   billingCacheService,
	}
}

// IsEnabled reports whether the affiliate (邀请返利) feature is turned on.
func (s *AffiliateService) IsEnabled(ctx context.Context) bool {
	if s == nil || s.settingService == nil {
		return AffiliateEnabledDefault
	}
	return s.settingService.IsAffiliateEnabled(ctx)
}

func (s *AffiliateService) EnsureUserAffiliate(ctx context.Context, userID int64) (*AffiliateSummary, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.EnsureUserAffiliate(ctx, userID)
}

func (s *AffiliateService) GetAffiliateDetail(ctx context.Context, userID int64) (*AffiliateDetail, error) {
	// Lazy thaw: move any matured frozen quota to available before reading.
	if s != nil && s.repo != nil {
		// best-effort: thaw failure is non-fatal
		_, _ = s.repo.ThawFrozenQuota(ctx, userID)
	}

	summary, err := s.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return nil, err
	}
	config, err := s.affiliateTierConfigStrict(ctx)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.resolveAffiliateTierProgressWithConfig(ctx, userID, config)
	if err != nil {
		return nil, err
	}
	invitees, err := s.listInvitees(ctx, userID, config.QualificationAmount)
	if err != nil {
		return nil, err
	}
	effectiveRate := effectiveAffiliateRebateRate(summary, snapshot.AutomaticRatePercent)
	return &AffiliateDetail{
		UserID:                     summary.UserID,
		AffCode:                    summary.AffCode,
		InviterID:                  summary.InviterID,
		AffCount:                   summary.AffCount,
		AffQuota:                   summary.AffQuota,
		AffFrozenQuota:             summary.AffFrozenQuota,
		AffHistoryQuota:            summary.AffHistoryQuota,
		AutomaticLevel:             snapshot.Level,
		AutomaticRebateRatePercent: snapshot.AutomaticRatePercent,
		EffectiveRebateRatePercent: effectiveRate,
		HasCustomRebateRate:        summary.AffRebateRatePercent != nil,
		CustomRebateRatePercent:    cloneFloat64Ptr(summary.AffRebateRatePercent),
		QualifiedInviteeCount:      snapshot.QualifiedInviteeCount,
		QualificationAmount:        config.QualificationAmount,
		NextLevelInviteeThreshold:  nullableAffiliateThreshold(snapshot.NextTierThreshold),
		RemainingQualifiedInvitees: snapshot.RemainingToNextTier,
		Tiers:                      affiliateTierDefinitions(config),
		Invitees:                   invitees,
	}, nil
}

func (s *AffiliateService) BindInviterByCode(ctx context.Context, userID int64, rawCode string) error {
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if code == "" {
		return nil
	}
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	// 总开关关闭时，注册阶段静默忽略 aff 参数（不报错，避免阻断注册流程）
	if !s.IsEnabled(ctx) {
		return nil
	}
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}

	selfSummary, err := s.repo.EnsureUserAffiliate(ctx, userID)
	if err != nil {
		return err
	}
	if selfSummary.InviterID != nil {
		return nil
	}

	inviterSummary, err := s.repo.GetAffiliateByCode(ctx, code)
	if err != nil {
		if errors.Is(err, ErrAffiliateProfileNotFound) {
			return ErrAffiliateCodeInvalid
		}
		return err
	}
	if inviterSummary == nil || inviterSummary.UserID <= 0 || inviterSummary.UserID == userID {
		return ErrAffiliateCodeInvalid
	}

	bound, err := s.repo.BindInviter(ctx, userID, inviterSummary.UserID)
	if err != nil {
		return err
	}
	if !bound {
		return ErrAffiliateAlreadyBound
	}
	return nil
}

func (s *AffiliateService) AccrueInviteRebate(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64) (float64, error) {
	return s.AccrueInviteRebateForOrder(ctx, inviteeUserID, baseRechargeAmount, nil)
}

func (s *AffiliateService) AccrueInviteRebateForOrder(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64, sourceOrderID *int64) (float64, error) {
	return s.accrueInviteRebateForOrder(ctx, inviteeUserID, baseRechargeAmount, sourceOrderID, false)
}

// AccrueTierAwareInviteRebateForOrder applies the configured automatic tier
// rate while preserving the legacy rebate lifecycle and order idempotency.
func (s *AffiliateService) AccrueTierAwareInviteRebateForOrder(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64, sourceOrderID *int64) (float64, error) {
	return s.accrueInviteRebateForOrder(ctx, inviteeUserID, baseRechargeAmount, sourceOrderID, true)
}

func (s *AffiliateService) accrueInviteRebateForOrder(ctx context.Context, inviteeUserID int64, baseRechargeAmount float64, sourceOrderID *int64, tierAware bool) (float64, error) {
	if s == nil || s.repo == nil {
		return 0, nil
	}
	if inviteeUserID <= 0 || baseRechargeAmount <= 0 || math.IsNaN(baseRechargeAmount) || math.IsInf(baseRechargeAmount, 0) {
		return 0, nil
	}
	// 总开关关闭时，新充值不再产生返利
	if !s.IsEnabled(ctx) {
		return 0, nil
	}

	inviteeSummary, err := s.repo.EnsureUserAffiliate(ctx, inviteeUserID)
	if err != nil {
		return 0, err
	}
	if inviteeSummary.InviterID == nil || *inviteeSummary.InviterID <= 0 {
		return 0, nil
	}

	// 加载邀请人 profile，优先使用专属比例（覆盖全局）
	inviterSummary, err := s.repo.EnsureUserAffiliate(ctx, *inviteeSummary.InviterID)
	if err != nil {
		return 0, err
	}
	// 有效期检查：超过返利有效期后不再产生返利
	if s.settingService != nil {
		if durationDays := s.settingService.GetAffiliateRebateDurationDays(ctx); durationDays > 0 {
			if time.Now().After(inviteeSummary.CreatedAt.AddDate(0, 0, durationDays)) {
				return 0, nil
			}
		}
	}

	rebateRatePercent := s.resolveRebateRatePercent(ctx, inviterSummary)
	if tierAware {
		rebateRatePercent, err = s.ResolveTierAwareRate(ctx, inviterSummary)
		if err != nil {
			return 0, err
		}
	}
	rebate := roundTo(baseRechargeAmount*(rebateRatePercent/100), 8)
	if rebate <= 0 {
		return 0, nil
	}

	// 单人上限检查：精确截断到剩余额度
	if s.settingService != nil {
		if perInviteeCap := s.settingService.GetAffiliateRebatePerInviteeCap(ctx); perInviteeCap > 0 {
			existing, err := s.repo.GetAccruedRebateFromInvitee(ctx, *inviteeSummary.InviterID, inviteeUserID)
			if err != nil {
				return 0, err
			}
			if existing >= perInviteeCap {
				return 0, nil
			}
			if remaining := perInviteeCap - existing; rebate > remaining {
				rebate = roundTo(remaining, 8)
			}
		}
	}

	var freezeHours int
	if s.settingService != nil {
		freezeHours = s.settingService.GetAffiliateRebateFreezeHours(ctx)
	}

	applied, err := s.repo.AccrueQuota(ctx, *inviteeSummary.InviterID, inviteeUserID, rebate, freezeHours, sourceOrderID)
	if err != nil {
		return 0, err
	}
	if !applied {
		return 0, nil
	}
	return rebate, nil
}

// resolveRebateRatePercent preserves the pre-tier flat/custom behavior used by
// existing payment fulfillment. Task 4 opts into ResolveTierAwareRate.
func (s *AffiliateService) resolveRebateRatePercent(ctx context.Context, inviter *AffiliateSummary) float64 {
	if inviter != nil && inviter.AffRebateRatePercent != nil {
		custom := *inviter.AffRebateRatePercent
		if math.IsNaN(custom) || math.IsInf(custom, 0) {
			return s.globalRebateRatePercent(ctx)
		}
		return clampAffiliateRebateRate(custom)
	}
	return s.globalRebateRatePercent(ctx)
}

func (s *AffiliateService) globalRebateRatePercent(ctx context.Context) float64 {
	if s == nil || s.settingService == nil {
		return AffiliateRebateRateDefault
	}
	return s.settingService.GetAffiliateRebateRatePercent(ctx)
}

// ResolveTierAwareRate is the opt-in rate path for Task 4 payment integration.
func (s *AffiliateService) ResolveTierAwareRate(ctx context.Context, inviter *AffiliateSummary) (float64, error) {
	if inviter == nil || inviter.UserID <= 0 {
		config, err := s.affiliateTierConfigStrict(ctx)
		if err != nil {
			return 0, s.withAffiliateTierReconcileMarker(ctx, err)
		}
		return config.StandardRate, nil
	}
	snapshot, err := s.ResolveAffiliateTierSnapshot(ctx, inviter.UserID)
	if err != nil {
		return 0, s.withAffiliateTierReconcileMarker(ctx, err)
	}
	return effectiveAffiliateRebateRate(inviter, snapshot.AutomaticRatePercent), nil
}

func effectiveAffiliateRebateRate(inviter *AffiliateSummary, automaticRate float64) float64 {
	if inviter != nil && inviter.AffRebateRatePercent != nil {
		custom := *inviter.AffRebateRatePercent
		if !math.IsNaN(custom) && !math.IsInf(custom, 0) && custom >= AffiliateRebateRateMin && custom <= AffiliateRebateRateMax {
			return custom
		}
	}
	return automaticRate
}

func (s *AffiliateService) ResolveAffiliateTierSnapshot(ctx context.Context, inviterID int64) (*AffiliateTierSnapshot, error) {
	_, snapshot, err := s.resolveAffiliateTierProgress(ctx, inviterID)
	return snapshot, err
}

func (s *AffiliateService) resolveAffiliateTierProgress(ctx context.Context, inviterID int64) (AffiliateTierConfig, *AffiliateTierSnapshot, error) {
	if s == nil || s.repo == nil {
		return AffiliateTierConfig{}, nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if inviterID <= 0 {
		return AffiliateTierConfig{}, nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	config, err := s.affiliateTierConfigStrict(ctx)
	if err != nil {
		return AffiliateTierConfig{}, nil, err
	}
	snapshot, err := s.resolveAffiliateTierProgressWithConfig(ctx, inviterID, config)
	return config, snapshot, err
}

func (s *AffiliateService) resolveAffiliateTierProgressWithConfig(ctx context.Context, inviterID int64, config AffiliateTierConfig) (*AffiliateTierSnapshot, error) {
	if s.qualificationRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate qualification service unavailable")
	}
	qualifiedCount, err := s.qualificationRepo.CountQualifiedInvitees(ctx, inviterID, config.QualificationAmount)
	if err != nil {
		return nil, fmt.Errorf("count qualified affiliate invitees: %w", err)
	}
	level, automaticRate, nextThreshold := config.Resolve(qualifiedCount)
	remaining := 0
	if nextThreshold > qualifiedCount {
		remaining = nextThreshold - qualifiedCount
	}
	return &AffiliateTierSnapshot{
		Level:                 level,
		AutomaticRatePercent:  automaticRate,
		QualifiedInviteeCount: qualifiedCount,
		NextTierThreshold:     nextThreshold,
		RemainingToNextTier:   remaining,
	}, nil
}

func (s *AffiliateService) affiliateTierConfigStrict(ctx context.Context) (AffiliateTierConfig, error) {
	if s == nil || s.settingService == nil {
		return DefaultAffiliateTierConfig(), nil
	}
	return s.settingService.GetAffiliateTierConfigStrict(ctx)
}

// ReconcilePendingAffiliateQualifications is an explicit orchestration hook for
// Task 4 startup/payment flows. Existing payment and API reads must not call it implicitly.
func (s *AffiliateService) ReconcilePendingAffiliateQualifications(ctx context.Context) error {
	if s == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if s.qualificationRepo == nil {
		return s.withAffiliateTierReconcileMarker(ctx, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate qualification service unavailable"))
	}
	if dbent.TxFromContext(ctx) != nil {
		return s.withAffiliateTierReconcileMarker(ctx, fmt.Errorf("affiliate qualification reconcile requires a non-transaction context"))
	}
	acquired, err := s.qualificationRepo.TryWithAffiliateQualificationReconcileLock(ctx, func(lockCtx context.Context) error {
		if err := s.drainAffiliateQualificationDirtyEvents(lockCtx); err != nil {
			return err
		}
		snapshot, err := s.qualificationRepo.ReadReconcilePendingSnapshot(lockCtx)
		if err != nil {
			return fmt.Errorf("read affiliate qualification reconcile snapshot: %w", err)
		}
		if !snapshot.Required {
			return nil
		}
		if snapshot.Generation <= 0 {
			return fmt.Errorf("invalid affiliate qualification reconcile generation %d", snapshot.Generation)
		}
		config, err := s.affiliateTierConfigStrict(lockCtx)
		if err != nil {
			return err
		}
		if err := s.qualificationRepo.ReconcileAllAffiliateQualifications(lockCtx, config.QualificationAmount, 200); err != nil {
			return fmt.Errorf("reconcile all affiliate qualifications: %w", err)
		}
		cleared, err := s.qualificationRepo.ClearReconcileRequiredIfGeneration(lockCtx, snapshot.Generation)
		if err != nil {
			return fmt.Errorf("clear affiliate qualification reconcile generation %d: %w", snapshot.Generation, err)
		}
		if !cleared {
			return ErrAffiliateQualificationReconcileStale
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, ErrAffiliateQualificationReconcileStale) {
			return err
		}
		return s.withAffiliateTierReconcileMarker(ctx, err)
	}
	if !acquired {
		return ErrAffiliateQualificationReconcileBusy
	}
	return nil
}

func (s *AffiliateService) drainAffiliateQualificationDirtyEvents(ctx context.Context) error {
	const batchSize = 200
	forceFull := false
	for {
		events, err := s.qualificationRepo.ListAffiliateQualificationDirtyEvents(ctx, batchSize)
		if err != nil {
			return fmt.Errorf("list affiliate qualification dirty events: %w", err)
		}
		for _, event := range events {
			if event.ParseError != "" || event.UserID <= 0 || strings.TrimSpace(event.Detail) == "" {
				if !forceFull {
					if _, err := s.qualificationRepo.MarkReconcileRequired(ctx); err != nil {
						return fmt.Errorf("mark full affiliate qualification reconcile for poison event %s: %w", event.OrderID, err)
					}
					forceFull = true
				}
				if err := s.qualificationRepo.MarkAffiliateQualificationDirtyEventFailed(ctx, event, errors.New(event.ParseError)); err != nil {
					return fmt.Errorf("dead-letter affiliate qualification dirty event %s: %w", event.OrderID, err)
				}
				continue
			}
			if err := s.reconcileAffiliateQualificationDirtyEvent(ctx, event); err != nil {
				return err
			}
		}
		if len(events) < batchSize {
			return nil
		}
	}
}

func (s *AffiliateService) ReconcileAffiliateQualificationDirtyEvent(ctx context.Context, event AffiliateQualificationDirtyEvent) error {
	if s == nil || s.qualificationRepo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate qualification service unavailable")
	}
	if dbent.TxFromContext(ctx) != nil {
		return fmt.Errorf("affiliate dirty event reconcile requires a non-transaction context")
	}
	err := s.reconcileAffiliateQualificationDirtyEvent(ctx, event)
	if err != nil {
		return s.withAffiliateTierReconcileMarker(ctx, err)
	}
	return nil
}

func (s *AffiliateService) reconcileAffiliateQualificationDirtyEvent(ctx context.Context, event AffiliateQualificationDirtyEvent) error {
	if event.UserID <= 0 || strings.TrimSpace(event.OrderID) == "" || strings.TrimSpace(event.Detail) == "" {
		return fmt.Errorf("invalid affiliate qualification dirty event for order %q", event.OrderID)
	}
	config, err := s.affiliateTierConfigStrict(ctx)
	if err != nil {
		return err
	}
	if _, err := s.qualificationRepo.ReconcileInviteeQualification(ctx, event.UserID, config.QualificationAmount); err != nil {
		return fmt.Errorf("reconcile affiliate dirty event for order %s: %w", event.OrderID, err)
	}
	deleted, err := s.qualificationRepo.DeleteAffiliateQualificationDirtyEvent(ctx, event)
	if err != nil {
		return fmt.Errorf("delete affiliate qualification dirty event for order %s: %w", event.OrderID, err)
	}
	if !deleted {
		return ErrAffiliateQualificationReconcileStale
	}
	return nil
}

func (s *AffiliateService) withAffiliateTierReconcileMarker(ctx context.Context, primary error) error {
	if primary == nil || s == nil || s.qualificationRepo == nil {
		return primary
	}
	_, markerErr := s.qualificationRepo.MarkReconcileRequired(dbent.WithoutTx(ctx))
	if markerErr != nil {
		logger.LegacyPrintf("service.affiliate", "[Affiliate] Failed to bump tier reconcile generation: %v (cause: %v)", markerErr, primary)
		return errors.Join(primary, fmt.Errorf("bump affiliate qualification reconcile generation: %w", markerErr))
	}
	return primary
}

func (s *AffiliateService) MarkReconcileRequired(ctx context.Context) (AffiliateReconcileToken, error) {
	if s == nil || s.qualificationRepo == nil {
		return AffiliateReconcileToken{}, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate qualification service unavailable")
	}
	return s.qualificationRepo.MarkReconcileRequired(ctx)
}

func (s *AffiliateService) ReconcileInviteeQualificationForGeneration(ctx context.Context, inviteeUserID int64, token AffiliateReconcileToken) error {
	if s == nil || s.qualificationRepo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate qualification service unavailable")
	}
	if dbent.TxFromContext(ctx) != nil {
		return fmt.Errorf("affiliate invitee qualification reconcile requires a non-transaction context")
	}
	if token.Generation <= 0 {
		return fmt.Errorf("invalid affiliate qualification reconcile generation %d", token.Generation)
	}
	config, err := s.affiliateTierConfigStrict(ctx)
	if err != nil {
		return err
	}
	if _, err := s.qualificationRepo.ReconcileInviteeQualification(ctx, inviteeUserID, config.QualificationAmount); err != nil {
		return err
	}
	if token.WasPendingBefore {
		return nil
	}
	cleared, err := s.qualificationRepo.ClearReconcileRequiredIfGeneration(ctx, token.Generation)
	if err != nil {
		return err
	}
	if !cleared {
		return ErrAffiliateQualificationReconcileStale
	}
	return nil
}

// ReconcileInviteeQualification refreshes one invitee's qualification from
// completed orders. It deliberately does not clear the global marker: a
// successful single-user repair does not prove the rest of the dataset is
// reconciled.
func (s *AffiliateService) ReconcileInviteeQualification(ctx context.Context, inviteeUserID int64) error {
	if s == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate qualification service unavailable")
	}
	if s.qualificationRepo == nil {
		return s.withAffiliateTierReconcileMarker(ctx, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate qualification service unavailable"))
	}
	if dbent.TxFromContext(ctx) != nil {
		return s.withAffiliateTierReconcileMarker(ctx, fmt.Errorf("affiliate invitee qualification reconcile requires a non-transaction context"))
	}
	config, err := s.affiliateTierConfigStrict(ctx)
	if err == nil {
		_, err = s.qualificationRepo.ReconcileInviteeQualification(ctx, inviteeUserID, config.QualificationAmount)
	}
	if err != nil {
		// A failed local repair must keep the recovery work visible to the next
		// payment/startup attempt. Marker write is best-effort by design.
		return s.withAffiliateTierReconcileMarker(ctx, err)
	}
	return nil
}

func (s *AffiliateService) TransferAffiliateQuota(ctx context.Context, userID int64) (float64, float64, error) {
	if s == nil || s.repo == nil {
		return 0, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}

	transferred, balance, err := s.repo.TransferQuotaToBalance(ctx, userID)
	if err != nil {
		return 0, 0, err
	}
	if transferred > 0 {
		s.invalidateAffiliateCaches(ctx, userID)
	}
	return transferred, balance, nil
}

func (s *AffiliateService) listInvitees(ctx context.Context, inviterID int64, qualificationAmount float64) ([]AffiliateInvitee, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if s.tierReadRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate tier read service unavailable")
	}
	invitees, err := s.tierReadRepo.ListInviteesWithQualification(ctx, inviterID, affiliateInviteesLimit, qualificationAmount)
	if err != nil {
		return nil, err
	}
	for i := range invitees {
		invitees[i].Email = maskEmail(invitees[i].Email)
		invitees[i].Qualified = invitees[i].QualifyingPaymentAmount >= qualificationAmount
		if invitees[i].Qualified != (invitees[i].QualifiedAt != nil) {
			return nil, fmt.Errorf("affiliate invitee %d qualification state is inconsistent", invitees[i].UserID)
		}
	}
	return invitees, nil
}

func (s *AffiliateService) bestEffortRecoverPendingQualifications(ctx context.Context) {
	if s == nil || s.qualificationRepo == nil {
		return
	}
	_ = s.ReconcilePendingAffiliateQualifications(ctx)
}

func affiliateTierDefinitions(config AffiliateTierConfig) []AffiliateTierDefinition {
	return []AffiliateTierDefinition{
		{Level: AffiliateTierStandard, MinQualifiedInvitees: 0, RatePercent: config.StandardRate},
		{Level: AffiliateTierBronze, MinQualifiedInvitees: config.BronzeInvitees, RatePercent: config.BronzeRate},
		{Level: AffiliateTierSilver, MinQualifiedInvitees: config.SilverInvitees, RatePercent: config.SilverRate},
		{Level: AffiliateTierGold, MinQualifiedInvitees: config.GoldInvitees, RatePercent: config.GoldRate},
	}
}

func nullableAffiliateThreshold(threshold int) *int {
	if threshold <= 0 {
		return nil
	}
	value := threshold
	return &value
}

func cloneFloat64Ptr(value *float64) *float64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func roundTo(v float64, scale int) float64 {
	factor := math.Pow10(scale)
	return math.Round(v*factor) / factor
}

func maskEmail(email string) string {
	email = strings.TrimSpace(email)
	if email == "" {
		return ""
	}
	at := strings.Index(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return "***"
	}

	local := email[:at]
	domain := email[at+1:]
	dot := strings.LastIndex(domain, ".")

	maskedLocal := maskSegment(local)
	if dot <= 0 || dot >= len(domain)-1 {
		return maskedLocal + "@" + maskSegment(domain)
	}

	domainName := domain[:dot]
	tld := domain[dot:]
	return maskedLocal + "@" + maskSegment(domainName) + tld
}

func maskSegment(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return "***"
	}
	if len(r) == 1 {
		return string(r[0]) + "***"
	}
	return string(r[0]) + "***"
}

func (s *AffiliateService) invalidateAffiliateCaches(ctx context.Context, userID int64) {
	if s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCacheService != nil {
		if err := s.billingCacheService.InvalidateUserBalance(ctx, userID); err != nil {
			logger.LegacyPrintf("service.affiliate", "[Affiliate] Failed to invalidate billing cache for user %d: %v", userID, err)
		}
	}
}

// =========================
// Admin: 专属配置管理
// =========================

// validateExclusiveRate ensures a per-user override is finite and within
// [Min, Max]. nil is always valid (means "clear / fall back to global").
func validateExclusiveRate(ratePercent *float64) error {
	if ratePercent == nil {
		return nil
	}
	v := *ratePercent
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return infraerrors.BadRequest("INVALID_RATE", "invalid rebate rate")
	}
	if v < AffiliateRebateRateMin || v > AffiliateRebateRateMax {
		return infraerrors.BadRequest("INVALID_RATE", "rebate rate out of range")
	}
	return nil
}

// AdminUpdateUserAffCode 管理员改写用户的邀请码（专属邀请码）。
func (s *AffiliateService) AdminUpdateUserAffCode(ctx context.Context, userID int64, rawCode string) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	code := strings.ToUpper(strings.TrimSpace(rawCode))
	if !isValidAffiliateCodeFormat(code) {
		return ErrAffiliateCodeInvalid
	}
	return s.repo.UpdateUserAffCode(ctx, userID, code)
}

// AdminResetUserAffCode 重置用户邀请码为系统随机码。
func (s *AffiliateService) AdminResetUserAffCode(ctx context.Context, userID int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ResetUserAffCode(ctx, userID)
}

// AdminSetUserRebateRate 设置/清除用户专属返利比例。ratePercent==nil 表示清除。
func (s *AffiliateService) AdminSetUserRebateRate(ctx context.Context, userID int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	return s.repo.SetUserRebateRate(ctx, userID, ratePercent)
}

// AdminBatchSetUserRebateRate 批量设置/清除用户专属返利比例。
func (s *AffiliateService) AdminBatchSetUserRebateRate(ctx context.Context, userIDs []int64, ratePercent *float64) error {
	if s == nil || s.repo == nil {
		return infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	if err := validateExclusiveRate(ratePercent); err != nil {
		return err
	}
	cleaned := make([]int64, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid > 0 {
			cleaned = append(cleaned, uid)
		}
	}
	if len(cleaned) == 0 {
		return nil
	}
	return s.repo.BatchSetUserRebateRate(ctx, cleaned, ratePercent)
}

// AdminListCustomUsers 列出有专属配置的用户。
func (s *AffiliateService) AdminListCustomUsers(ctx context.Context, filter AffiliateAdminFilter) ([]AffiliateAdminEntry, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListUsersWithCustomSettings(ctx, filter)
}

func (s *AffiliateService) AdminListInviteRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateInviteRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	config, err := s.affiliateTierConfigStrict(ctx)
	if err != nil {
		return nil, 0, err
	}
	if s.tierReadRepo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate tier read service unavailable")
	}
	normalizedFilter := normalizeAffiliateRecordFilter(filter)
	items, total, err := s.tierReadRepo.ListAffiliateInviteRecordsWithQualification(ctx, normalizedFilter, config.QualificationAmount)
	if err != nil {
		return nil, 0, err
	}
	for i := range items {
		items[i].Qualified = items[i].QualifyingPaymentAmount >= config.QualificationAmount
		if items[i].Qualified != (items[i].QualifiedAt != nil) {
			return nil, 0, fmt.Errorf("affiliate invite record %d qualification state is inconsistent", items[i].InviteeID)
		}
		level, automaticRate, _ := config.Resolve(items[i].QualifiedInviteeCount)
		items[i].AutomaticLevel = level
		items[i].AutomaticRebateRatePercent = automaticRate
		items[i].EffectiveRebateRatePercent = effectiveAffiliateRebateRate(&AffiliateSummary{AffRebateRatePercent: items[i].CustomRebateRatePercent}, automaticRate)
	}
	return items, total, nil
}

func (s *AffiliateService) AdminListRebateRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateRebateRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateRebateRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminListTransferRecords(ctx context.Context, filter AffiliateRecordFilter) ([]AffiliateTransferRecord, int64, error) {
	if s == nil || s.repo == nil {
		return nil, 0, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	return s.repo.ListAffiliateTransferRecords(ctx, normalizeAffiliateRecordFilter(filter))
}

func (s *AffiliateService) AdminGetUserOverview(ctx context.Context, userID int64) (*AffiliateUserOverview, error) {
	if userID <= 0 {
		return nil, infraerrors.BadRequest("INVALID_USER", "invalid user")
	}
	if s == nil || s.repo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate service unavailable")
	}
	config, err := s.affiliateTierConfigStrict(ctx)
	if err != nil {
		return nil, err
	}
	if s.tierReadRepo == nil {
		return nil, infraerrors.ServiceUnavailable("SERVICE_UNAVAILABLE", "affiliate tier read service unavailable")
	}
	overview, err := s.tierReadRepo.GetAffiliateUserOverviewWithQualification(ctx, userID, config.QualificationAmount)
	if err != nil {
		return nil, err
	}
	if overview != nil {
		level, automaticRate, nextThreshold := config.Resolve(overview.QualifiedInviteeCount)
		overview.AutomaticLevel = level
		overview.AutomaticRebateRatePercent = automaticRate
		overview.HasCustomRebateRate = overview.CustomRebateRatePercent != nil
		overview.EffectiveRebateRatePercent = effectiveAffiliateRebateRate(&AffiliateSummary{AffRebateRatePercent: overview.CustomRebateRatePercent}, automaticRate)
		overview.RebateRatePercent = overview.EffectiveRebateRatePercent
		overview.RebateRateCustom = overview.HasCustomRebateRate
		overview.QualificationAmount = config.QualificationAmount
		overview.NextLevelInviteeThreshold = nullableAffiliateThreshold(nextThreshold)
		if nextThreshold > overview.QualifiedInviteeCount {
			overview.RemainingQualifiedInvitees = nextThreshold - overview.QualifiedInviteeCount
		}
	}
	return overview, nil
}

func normalizeAffiliateRecordFilter(filter AffiliateRecordFilter) AffiliateRecordFilter {
	if filter.Page <= 0 {
		filter.Page = 1
	}
	if filter.PageSize <= 0 {
		filter.PageSize = 20
	}
	if filter.PageSize > 100 {
		filter.PageSize = 100
	}
	filter.Search = strings.TrimSpace(filter.Search)
	filter.SortBy = strings.TrimSpace(filter.SortBy)
	return filter
}
