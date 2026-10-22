package service

import (
	"context"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

var ErrGroupAccountUserExclusionNotFound = infraerrors.NotFound("GROUP_ACCOUNT_USER_EXCLUSION_NOT_FOUND", "group account user exclusion not found")

// GroupAccountUserExclusion is an administrator-only rule; never embed it in public DTOs.
type GroupAccountUserExclusion struct {
	ID          int64     `json:"id"`
	GroupID     int64     `json:"group_id"`
	AccountID   int64     `json:"account_id"`
	AccountName string    `json:"account_name"`
	UserID      int64     `json:"user_id"`
	UserEmail   string    `json:"user_email"`
	Username    string    `json:"username"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

type GroupAccountUserExclusionRepository interface {
	List(context.Context, int64, pagination.PaginationParams) ([]GroupAccountUserExclusion, pagination.PaginationResult, error)
	Add(context.Context, int64, int64, int64) (*GroupAccountUserExclusion, error)
	Remove(context.Context, int64, int64) (int64, error)
}

type GroupAccountUserExclusionService struct {
	repo        GroupAccountUserExclusionRepository
	invalidator APIKeyAuthCacheInvalidator
}

func NewGroupAccountUserExclusionService(repo GroupAccountUserExclusionRepository, apiKeys *APIKeyService) *GroupAccountUserExclusionService {
	svc := &GroupAccountUserExclusionService{repo: repo}
	if apiKeys != nil {
		svc.invalidator = apiKeys
	}
	return svc
}

func (s *GroupAccountUserExclusionService) List(ctx context.Context, groupID int64, params pagination.PaginationParams) ([]GroupAccountUserExclusion, pagination.PaginationResult, error) {
	return s.repo.List(ctx, groupID, params)
}

func (s *GroupAccountUserExclusionService) Add(ctx context.Context, groupID, accountID, userID int64) (*GroupAccountUserExclusion, error) {
	if groupID <= 0 {
		return nil, ErrGroupNotFound
	}
	if accountID <= 0 {
		return nil, ErrAccountNotFound
	}
	if userID <= 0 {
		return nil, ErrUserNotFound
	}
	item, err := s.repo.Add(ctx, groupID, accountID, userID)
	if err != nil {
		return nil, err
	}
	if s.invalidator != nil {
		s.invalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return item, nil
}

func (s *GroupAccountUserExclusionService) Remove(ctx context.Context, groupID, exclusionID int64) error {
	userID, err := s.repo.Remove(ctx, groupID, exclusionID)
	if err != nil {
		return err
	}
	if s.invalidator != nil {
		s.invalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	return nil
}
