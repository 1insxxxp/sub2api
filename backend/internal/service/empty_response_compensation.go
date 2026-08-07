package service

import (
	"context"
	"log/slog"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

var (
	ErrEmptyResponseCompensationInvalidInput = infraerrors.BadRequest("EMPTY_RESPONSE_COMPENSATION_INVALID_INPUT", "claim_id must be greater than zero")
	ErrEmptyResponseCompensationInvalidState = infraerrors.Conflict("EMPTY_RESPONSE_COMPENSATION_INVALID_STATE", "claim is not approved for compensation")
)

type EmptyResponseCompensationResult struct {
	Applied        bool
	ClaimID        int64
	UsageLogID     int64
	UserID         int64
	APIKeyID       int64
	APIKey         string
	GroupID        *int64
	SubscriptionID *int64
	RefundAmount   float64
}

type EmptyResponseCompensationRepository interface {
	Compensate(ctx context.Context, claimID int64) (*EmptyResponseCompensationResult, error)
}

type EmptyResponseCompensationCacheInvalidator interface {
	InvalidateUserBalance(ctx context.Context, userID int64) error
	InvalidateSubscription(ctx context.Context, userID, groupID int64) error
}

type EmptyResponseCompensationService struct {
	repo      EmptyResponseCompensationRepository
	cache     EmptyResponseCompensationCacheInvalidator
	authCache APIKeyAuthCacheInvalidator
}

func NewEmptyResponseCompensationService(
	repo EmptyResponseCompensationRepository,
	cache EmptyResponseCompensationCacheInvalidator,
	authCache APIKeyAuthCacheInvalidator,
) *EmptyResponseCompensationService {
	return &EmptyResponseCompensationService{repo: repo, cache: cache, authCache: authCache}
}

func (s *EmptyResponseCompensationService) CompensateApprovedClaim(ctx context.Context, claimID int64) error {
	if claimID <= 0 {
		return ErrEmptyResponseCompensationInvalidInput
	}
	if s == nil || s.repo == nil {
		return ErrEmptyResponseCompensationInvalidState
	}
	result, err := s.repo.Compensate(ctx, claimID)
	if err != nil {
		return err
	}
	if result == nil {
		return ErrEmptyResponseCompensationInvalidState
	}

	if s.cache != nil {
		if err := s.cache.InvalidateUserBalance(ctx, result.UserID); err != nil {
			slog.Warn("empty response compensation balance cache invalidation failed",
				"claim_id", claimID, "user_id", result.UserID, "error", err)
		}
		if result.SubscriptionID != nil && result.GroupID != nil {
			if err := s.cache.InvalidateSubscription(ctx, result.UserID, *result.GroupID); err != nil {
				slog.Warn("empty response compensation subscription cache invalidation failed",
					"claim_id", claimID, "user_id", result.UserID, "group_id", *result.GroupID, "error", err)
			}
		}
	}
	if s.authCache != nil {
		if result.APIKey != "" {
			s.authCache.InvalidateAuthCacheByKey(ctx, result.APIKey)
		} else {
			s.authCache.InvalidateAuthCacheByUserID(ctx, result.UserID)
		}
	}
	return nil
}
