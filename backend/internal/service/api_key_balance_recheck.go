package service

import "context"

// RecheckUserBalance verifies a wallet rejection against the authoritative user
// row. It does not write a balance snapshot back over concurrent deductions.
func (s *APIKeyService) RecheckUserBalance(ctx context.Context, userID int64) (*User, error) {
	if s == nil {
		return nil, ErrBillingServiceUnavailable
	}
	user, err := s.balanceRecheck.load(ctx, s.userRepo, userID)
	if err != nil {
		return nil, ErrBillingServiceUnavailable.WithCause(err)
	}
	return user, nil
}
