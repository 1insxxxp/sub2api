package service

import (
	"context"
	"time"
)

// UsagePersistenceEvent identifies a committed settlement awaiting its usage
// detail. Recovery never repeats financial side effects.
type UsagePersistenceEvent struct {
	ID        int64
	Attempts  int
	CreatedAt time.Time
}

type UsagePersistenceOutboxRepository interface {
	Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]UsagePersistenceEvent, error)
	Deliver(ctx context.Context, id int64, workerID string) error
	RetryClaimed(ctx context.Context, id int64, workerID string, availableAt time.Time, lastError string) error
}

// DurableUsageBillingRepository commits the usage snapshot in the same
// transaction as settlement. Callers must not race recovery with a zero-cost
// placeholder when the database commit acknowledgement is uncertain.
type DurableUsageBillingRepository interface {
	UsagePersistenceEnabled() bool
}
