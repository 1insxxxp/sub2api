//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestInflightEstimateCapOneAcrossGateways(t *testing.T) {
	group := newInflightUltrafastGroup("gpt-6.1-sol")
	group.ForceOpenAIUltrafast = true
	key := newInflightUltrafastKey(group)
	request := inflightUltrafastRequest("gpt-6.1-sol", "ultrafast")
	request.MaxTokens = 1_000_000
	for name, estimator := range newInflightUltrafastEstimators(t, group, nil, nil, nil) {
		t.Run(name, func(t *testing.T) {
			estimate, priced := estimator.EstimateInflightReservation(inflightUltrafastContext(key), key, request)
			require.True(t, priced)
			require.Equal(t, 1.0, estimate, "cap after tier pricing and all multipliers")
		})
	}
	billing := NewBillingService(&config.Config{}, nil)
	estimate, priced := EstimateInflightReservationCost(billing, config.InflightReservationConfig{}, "claude-sonnet-4-5", 1_000_000, 1_000_000, 2)
	require.True(t, priced)
	require.Equal(t, 1.0, estimate)
	cost, err := billing.CalculateCost("claude-sonnet-4-5", UsageTokens{InputTokens: 250_000, OutputTokens: 1_000_000}, 2)
	require.NoError(t, err)
	require.Greater(t, cost.ActualCost, 1.0, "the precharge ceiling must not discount actual billing")
}

func TestInflightEstimateCapOneAdmissionAndRelease(t *testing.T) {
	ctx := context.Background()
	user := &User{ID: 1, Balance: 1}
	cache := newMemInflightCache(1)
	svc := newInflightSvc(t, cache, 60)
	svc.userRepo = &inflightRecheckUserRepo{user: user}
	res, err := svc.ReserveInflight(ctx, user, nil, nil, 128.09169)
	require.NoError(t, err)
	require.NotNil(t, res)
	t.Cleanup(res.HandlerDone)
	require.Equal(t, 1.0, res.Amount())
	require.Equal(t, 1, cache.count())
	second, err := svc.ReserveInflight(ctx, user, nil, nil, 99)
	require.Nil(t, second)
	require.ErrorIs(t, err, ErrBalanceInFlight)
	require.Contains(t, pkgerrors.Message(err), "本次预扣额度 US$1.00")
	require.Equal(t, 1, cache.count())
	res.HandlerDone()
	require.Zero(t, cache.count())
	release, err := svc.ReserveInflightBalance(ctx, user, nil, nil, 99)
	require.NoError(t, err)
	require.Equal(t, 1, cache.count())
	release()
	require.Zero(t, cache.count())
	small, err := svc.ReserveInflight(ctx, user, nil, nil, 0.25)
	require.NoError(t, err)
	require.Equal(t, 0.25, small.Amount())
	small.HandlerDone()
	// A known fixed charge is a separate exact-price admission, not an estimate.
	fixed, err := svc.ReserveFixedCharge(ctx, user, nil, nil, nil, 8)
	require.Nil(t, fixed)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Zero(t, cache.count())
}

func TestInflightEstimateCapOneInsufficientBalanceMessage(t *testing.T) {
	user := &User{ID: 1, Balance: 0.99}
	cache := newMemInflightCache(user.Balance)
	svc := newInflightSvc(t, cache, 60)
	svc.userRepo = &inflightRecheckUserRepo{user: user}
	res, err := svc.ReserveInflight(context.Background(), user, nil, nil, 128.09169)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Contains(t, pkgerrors.Message(err), "本次预扣额度 US$1.00")
	require.NotContains(t, pkgerrors.Message(err), "128.09")
	require.Zero(t, cache.count())
}
