package service

import (
	"context"
	"errors"
)

// AdminWorkbenchAccess contains only the independent page capabilities. It
// neither changes a user's role nor authorizes any other administrator API.
type AdminWorkbenchAccess struct {
	Commission           bool `json:"commission"`
	AffiliateLeaderboard bool `json:"affiliate_leaderboard"`
	UserSupport          bool `json:"user_support"`
	BalanceTransfer      bool `json:"balance_transfer"`
}

type AdminWorkbenchAccessRepository interface {
	GetAccess(ctx context.Context, actorID int64) (*AdminWorkbenchAccess, error)
	GetUserAccess(ctx context.Context, actorID, targetID int64) (*AdminWorkbenchAccess, error)
	SetUserAccess(ctx context.Context, actorID, targetID int64, commission, leaderboard, userSupport bool) error
}

type AdminWorkbenchAccessService struct {
	repo AdminWorkbenchAccessRepository
}

func NewAdminWorkbenchAccessService(repo AdminWorkbenchAccessRepository) *AdminWorkbenchAccessService {
	return &AdminWorkbenchAccessService{repo: repo}
}

func (s *AdminWorkbenchAccessService) GetAccess(ctx context.Context, actorID int64) (*AdminWorkbenchAccess, error) {
	if actorID <= 0 {
		return nil, ErrUserSupportForbidden
	}
	if s == nil || s.repo == nil {
		return nil, ErrUserSupportUnavailable
	}
	access, err := s.repo.GetAccess(ctx, actorID)
	return safeWorkbenchAccessResult(access, err)
}

func (s *AdminWorkbenchAccessService) GetUserAccess(ctx context.Context, actorID, targetID int64) (*AdminWorkbenchAccess, error) {
	if actorID <= 0 || targetID <= 0 {
		return nil, ErrUserSupportForbidden
	}
	if s == nil || s.repo == nil {
		return nil, ErrUserSupportUnavailable
	}
	access, err := s.repo.GetUserAccess(ctx, actorID, targetID)
	return safeWorkbenchAccessResult(access, err)
}

func (s *AdminWorkbenchAccessService) SetUserAccess(ctx context.Context, actorID, targetID int64, commission, leaderboard, userSupport bool) error {
	if actorID <= 0 || targetID <= 0 {
		return ErrUserSupportForbidden
	}
	if s == nil || s.repo == nil {
		return ErrUserSupportUnavailable
	}
	return safeWorkbenchAccessError(s.repo.SetUserAccess(ctx, actorID, targetID, commission, leaderboard, userSupport))
}

func safeWorkbenchAccessResult(access *AdminWorkbenchAccess, err error) (*AdminWorkbenchAccess, error) {
	if err != nil {
		return nil, safeWorkbenchAccessError(err)
	}
	if access == nil {
		return nil, ErrUserSupportUnavailable
	}
	return access, nil
}

func safeWorkbenchAccessError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrUserSupportForbidden) {
		return ErrUserSupportForbidden
	}
	if errors.Is(err, ErrUserSupportUserNotFound) {
		return ErrUserSupportUserNotFound
	}
	return ErrUserSupportUnavailable
}
