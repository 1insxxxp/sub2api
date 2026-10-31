package service

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

const (
	walletBalanceRecheckTimeout     = 3 * time.Second
	walletBalanceRecheckConcurrency = 16
)

type walletBalanceReader interface {
	GetByID(context.Context, int64) (*User, error)
}

// walletBalanceRechecker is used only on an otherwise rejected wallet. It does
// not cache results: a recharge must be visible on the very next attempt.
type walletBalanceRechecker struct {
	loads singleflight.Group
	once  sync.Once
	slots chan struct{}
}

func (r *walletBalanceRechecker) load(ctx context.Context, repo walletBalanceReader, userID int64) (*User, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if repo == nil {
		return nil, fmt.Errorf("wallet balance reader unavailable")
	}
	r.once.Do(func() { r.slots = make(chan struct{}, walletBalanceRecheckConcurrency) })
	result := r.loads.DoChan(strconv.FormatInt(userID, 10), func() (any, error) {
		select {
		case r.slots <- struct{}{}:
			defer func() { <-r.slots }()
		default:
			return nil, fmt.Errorf("wallet balance recheck overloaded")
		}
		loadCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), walletBalanceRecheckTimeout)
		defer cancel()
		user, err := repo.GetByID(loadCtx, userID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, fmt.Errorf("wallet owner unavailable")
		}
		return user, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-result:
		if res.Err != nil {
			return nil, res.Err
		}
		user, ok := res.Val.(*User)
		if !ok || user == nil {
			return nil, fmt.Errorf("invalid wallet balance result")
		}
		return user, nil
	}
}
