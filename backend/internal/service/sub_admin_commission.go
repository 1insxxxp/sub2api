package service

import (
	"context"
	"math"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var (
	ErrSubAdminCommissionForbidden   = infraerrors.Forbidden("SUB_ADMIN_COMMISSION_FORBIDDEN", "group is not assigned to this secondary admin")
	ErrSubAdminCommissionRateInvalid = infraerrors.BadRequest("SUB_ADMIN_COMMISSION_RATE_INVALID", "commission rate must be between 0 and 1")
	ErrSubAdminCommissionUserInvalid = infraerrors.BadRequest("SUB_ADMIN_COMMISSION_USER_INVALID", "target user must be a secondary admin")
	ErrSubAdminCommissionRepoMissing = infraerrors.InternalServer("SUB_ADMIN_COMMISSION_REPO_MISSING", "sub-admin commission repository is not configured")
)

type SubAdminCommissionGrant struct {
	ID            int64
	SubAdminID    int64
	SubAdminEmail string
	GroupID       int64
	GroupName     string
	GrantedDate   string
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type SubAdminCommissionCalendarDay struct {
	Date             string
	Enabled          bool
	ActualCost       float64
	CommissionAmount float64
}

type SubAdminCommissionDayGroup struct {
	GroupID          int64
	GroupName        string
	Requests         int64
	TotalTokens      int64
	ActualCost       float64
	CommissionAmount float64
}

type SubAdminCommissionUsageLog struct {
	ID                  int64
	RequestID           string
	CreatedAt           time.Time
	UserID              int64
	UserEmail           string
	APIKeyID            int64
	APIKeyName          string
	GroupID             int64
	GroupName           string
	Model               string
	RequestedModel      *string
	InputTokens         int
	OutputTokens        int
	CacheCreationTokens int
	CacheReadTokens     int
	ActualCost          float64
}

type ReplaceSubAdminCommissionGrantsInput struct {
	SubAdminID int64
	GroupIDs   []int64
	OperatorID int64
	Now        time.Time
}

type SubAdminCommissionRepository interface {
	ListAllGrants(ctx context.Context) ([]SubAdminCommissionGrant, error)
	ListGrantsForSubAdmin(ctx context.Context, subAdminID int64) ([]SubAdminCommissionGrant, error)
	ReplaceGrants(ctx context.Context, input ReplaceSubAdminCommissionGrantsInput, grantedDate string) ([]SubAdminCommissionGrant, error)
	ListCalendar(ctx context.Context, subAdminID int64, month string, commissionRate float64, now time.Time) ([]SubAdminCommissionCalendarDay, error)
	ListDayGroups(ctx context.Context, subAdminID int64, date string, commissionRate float64) ([]SubAdminCommissionDayGroup, error)
	ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]SubAdminCommissionUsageLog, pagination.PaginationResult, error)
}

type SubAdminCommissionService struct {
	repo       SubAdminCommissionRepository
	userRepo   UserRepository
	settingSvc *SettingService
}

func NewSubAdminCommissionService(repo SubAdminCommissionRepository, userRepo UserRepository, settingSvc *SettingService) *SubAdminCommissionService {
	return &SubAdminCommissionService{
		repo:       repo,
		userRepo:   userRepo,
		settingSvc: settingSvc,
	}
}

func (s *SubAdminCommissionService) GetSettings(ctx context.Context) (float64, error) {
	if s == nil || s.settingSvc == nil {
		return 0, nil
	}
	return s.settingSvc.GetSubAdminCommissionRate(ctx), nil
}

func (s *SubAdminCommissionService) SetSettings(ctx context.Context, rate float64) (float64, error) {
	if !validSubAdminCommissionRate(rate) {
		return 0, ErrSubAdminCommissionRateInvalid
	}
	if s == nil || s.settingSvc == nil {
		return 0, ErrSettingNotFound
	}
	if err := s.settingSvc.SetSubAdminCommissionRate(ctx, rate); err != nil {
		return 0, err
	}
	return rate, nil
}

func validSubAdminCommissionRate(rate float64) bool {
	return !math.IsNaN(rate) && !math.IsInf(rate, 0) && rate >= 0 && rate <= 1
}

func (s *SubAdminCommissionService) ListAllGrants(ctx context.Context) ([]SubAdminCommissionGrant, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSubAdminCommissionRepoMissing
	}
	return s.repo.ListAllGrants(ctx)
}

func (s *SubAdminCommissionService) ReplaceGrants(ctx context.Context, input ReplaceSubAdminCommissionGrantsInput) ([]SubAdminCommissionGrant, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSubAdminCommissionRepoMissing
	}
	now := input.Now
	if now.IsZero() {
		now = time.Now()
	}
	return s.repo.ReplaceGrants(ctx, input, GroupUsageDate(now))
}

func (s *SubAdminCommissionService) ListWorkbenchGrants(ctx context.Context, subAdminID int64) ([]SubAdminCommissionGrant, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSubAdminCommissionRepoMissing
	}
	return s.repo.ListGrantsForSubAdmin(ctx, subAdminID)
}

func (s *SubAdminCommissionService) ListCalendar(ctx context.Context, subAdminID int64, month string, now time.Time) ([]SubAdminCommissionCalendarDay, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSubAdminCommissionRepoMissing
	}
	if now.IsZero() {
		now = time.Now()
	}
	rate, _ := s.GetSettings(ctx)
	return s.repo.ListCalendar(ctx, subAdminID, month, rate, now)
}

func (s *SubAdminCommissionService) ListDayGroups(ctx context.Context, subAdminID int64, date string) ([]SubAdminCommissionDayGroup, error) {
	if s == nil || s.repo == nil {
		return nil, ErrSubAdminCommissionRepoMissing
	}
	rate, _ := s.GetSettings(ctx)
	return s.repo.ListDayGroups(ctx, subAdminID, date, rate)
}

func (s *SubAdminCommissionService) ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]SubAdminCommissionUsageLog, pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, pagination.PaginationResult{}, ErrSubAdminCommissionRepoMissing
	}
	return s.repo.ListDayGroupLogs(ctx, subAdminID, groupID, date, params)
}
