package service

import (
	"context"
	"time"
)

// VersionedBalanceCache prevents a database snapshot read before settlement
// from replacing the newer wallet cache after that settlement completes.
type VersionedBalanceCache interface {
	GetUserBalanceVersion(ctx context.Context, userID int64) (string, error)
	SetUserBalanceIfVersion(ctx context.Context, userID int64, balance float64, version string) (bool, error)
	// ConstrainUserBalance applies a committed wallet snapshot as an upper bound
	// to an existing cache entry and invalidates older in-progress cache fills.
	ConstrainUserBalance(ctx context.Context, userID int64, balance float64) error
}

// SubscriptionCacheData represents cached subscription data
type SubscriptionCacheData struct {
	Status       string
	ExpiresAt    time.Time
	DailyUsage   float64
	WeeklyUsage  float64
	MonthlyUsage float64
	Version      int64
}
