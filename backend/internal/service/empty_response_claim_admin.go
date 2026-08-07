package service

import (
	"context"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

type EmptyResponseClaimListFilters struct {
	Status    string
	UserID    int64
	GroupID   int64
	AccountID int64
	Model     string
	StartTime *time.Time
	EndTime   *time.Time
}

type EmptyResponseClaimAdminRepository interface {
	List(ctx context.Context, params pagination.PaginationParams, filters EmptyResponseClaimListFilters) ([]EmptyResponseClaim, *pagination.PaginationResult, error)
	Review(ctx context.Context, id int64, status string, reviewerID int64, note string) (*EmptyResponseClaim, error)
}

type EmptyResponseClaimBatchInput struct {
	IDs        []int64
	Action     string
	ReviewerID int64
	Note       string
}

type EmptyResponseClaimBatchResult struct {
	Succeeded []int64              `json:"succeeded"`
	Failed    map[int64]string     `json:"failed"`
	Claims    []EmptyResponseClaim `json:"claims"`
}

type EmptyResponseClaimAdminService struct {
	repo        EmptyResponseClaimAdminRepository
	compensator EmptyResponseClaimCompensator
	metrics     *EmptyResponseClaimMetricsService
}

func NewEmptyResponseClaimAdminService(repo EmptyResponseClaimAdminRepository, compensator EmptyResponseClaimCompensator) *EmptyResponseClaimAdminService {
	service := &EmptyResponseClaimAdminService{repo: repo, compensator: compensator}
	if metricsRepo, ok := repo.(EmptyResponseClaimMetricsRepository); ok {
		service.metrics = NewEmptyResponseClaimMetricsService(metricsRepo)
	}
	return service
}

func (s *EmptyResponseClaimAdminService) Metrics(ctx context.Context, start, end time.Time) (*EmptyResponseClaimMetrics, error) {
	if s == nil || s.metrics == nil {
		return nil, ErrEmptyResponseClaimNotFound
	}
	return s.metrics.Get(ctx, start, end)
}

func (s *EmptyResponseClaimAdminService) List(ctx context.Context, params pagination.PaginationParams, filters EmptyResponseClaimListFilters) ([]EmptyResponseClaim, *pagination.PaginationResult, error) {
	if s == nil || s.repo == nil {
		return nil, nil, ErrEmptyResponseClaimNotFound
	}
	return s.repo.List(ctx, params, filters)
}

func (s *EmptyResponseClaimAdminService) Approve(ctx context.Context, id, reviewerID int64, note string) (*EmptyResponseClaim, error) {
	return s.review(ctx, id, reviewerID, EmptyResponseClaimApproved, note)
}

func (s *EmptyResponseClaimAdminService) Reject(ctx context.Context, id, reviewerID int64, note string) (*EmptyResponseClaim, error) {
	if strings.TrimSpace(note) == "" {
		return nil, ErrEmptyResponseClaimInvalidInput
	}
	return s.review(ctx, id, reviewerID, EmptyResponseClaimRejected, note)
}

func (s *EmptyResponseClaimAdminService) review(ctx context.Context, id, reviewerID int64, status, note string) (*EmptyResponseClaim, error) {
	if s == nil || s.repo == nil || id <= 0 || reviewerID <= 0 {
		return nil, ErrEmptyResponseClaimInvalidInput
	}
	note = trimEmptyResponseClaimAdminNote(note)
	claim, err := s.repo.Review(ctx, id, status, reviewerID, note)
	if err != nil {
		return nil, err
	}
	if status == EmptyResponseClaimApproved && s.compensator != nil {
		if err := s.compensator.CompensateApprovedClaim(ctx, claim.ID); err != nil {
			return claim, err
		}
		claim.Status = EmptyResponseClaimCompensated
		if claim.SubscriptionID != nil {
			claim.SubscriptionRefund = claim.OriginalActualCost
			claim.BalanceRefund = 0
		} else {
			claim.BalanceRefund = claim.OriginalActualCost
			claim.SubscriptionRefund = 0
		}
		claim.APIKeyQuotaRefund = claim.OriginalActualCost
	}
	return claim, nil
}

func (s *EmptyResponseClaimAdminService) Batch(ctx context.Context, input EmptyResponseClaimBatchInput) (*EmptyResponseClaimBatchResult, error) {
	if len(input.IDs) == 0 || len(input.IDs) > 100 || input.ReviewerID <= 0 {
		return nil, ErrEmptyResponseClaimInvalidInput
	}
	if input.Action != EmptyResponseClaimApproved && input.Action != EmptyResponseClaimRejected {
		return nil, ErrEmptyResponseClaimInvalidInput
	}
	if input.Action == EmptyResponseClaimRejected && strings.TrimSpace(input.Note) == "" {
		return nil, ErrEmptyResponseClaimInvalidInput
	}
	result := &EmptyResponseClaimBatchResult{Failed: make(map[int64]string)}
	seen := make(map[int64]struct{}, len(input.IDs))
	for _, id := range input.IDs {
		if id <= 0 {
			result.Failed[id] = ErrEmptyResponseClaimInvalidInput.Error()
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		claim, err := s.review(ctx, id, input.ReviewerID, input.Action, input.Note)
		if err != nil {
			result.Failed[id] = err.Error()
			continue
		}
		result.Succeeded = append(result.Succeeded, id)
		result.Claims = append(result.Claims, *claim)
	}
	return result, nil
}

func trimEmptyResponseClaimAdminNote(value string) string {
	value = strings.TrimSpace(value)
	runes := []rune(value)
	if len(runes) > 2000 {
		return string(runes[:2000])
	}
	return value
}
