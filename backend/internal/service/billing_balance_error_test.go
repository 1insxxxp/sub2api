//go:build unit

package service

import (
	"context"
	"math"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	pkgerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestBalanceErrorFormatting_PreservesSmallAmountsAndUnknownEstimates(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount float64
		want   string
	}{
		{"zero", 0, "US$0.00"},
		{"negative zero", math.Copysign(0, -1), "US$0.00"},
		{"whole", 5, "US$5.00"},
		{"cent", 0.02, "US$0.02"},
		{"fraction", -0.005, "US$-0.005"},
		{"wallet precision", 0.00000001, "US$0.00000001"},
		{"smaller estimate", 0.000000001, "US$1e-09"},
		{"smallest estimate", math.SmallestNonzeroFloat64, "US$5e-324"},
		{"binary roundoff", 0.1 + 0.2, "US$0.30"},
		{"invalid", math.NaN(), "unknown"},
		{"infinite", math.Inf(1), "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := NewInsufficientBalanceError(tc.amount, &tc.amount)
			require.Contains(t, err.Message, "Current balance: "+tc.want+";")
			require.Contains(t, err.Message, "estimated reservation for this request: "+tc.want+".")
		})
	}

	err := NewInsufficientBalanceError(0, nil)
	require.Contains(t, err.Message, "estimated reservation for this request: not calculated.")
}

func TestBalanceErrorDetails_DoNotMutateSharedErrorsOrOtherUsers(t *testing.T) {
	originalBalanceMessage := ErrInsufficientBalance.Message
	originalInflightMessage := ErrBalanceInFlight.Message
	firstAmount, secondAmount := 0.125, 5.0
	first := NewInsufficientBalanceError(-0.01, &firstAmount)
	second := NewInsufficientBalanceError(0.02, &secondAmount)
	inflight := newBalanceInFlightError(0.5, 0.4, 0.25)

	require.ErrorIs(t, first, ErrInsufficientBalance)
	require.ErrorIs(t, second, ErrInsufficientBalance)
	require.ErrorIs(t, inflight, ErrBalanceInFlight)
	require.Equal(t, ErrInsufficientBalance.Code, first.Code)
	require.Equal(t, ErrInsufficientBalance.Reason, first.Reason)
	require.Equal(t, ErrBalanceInFlight.Code, inflight.Code)
	require.Equal(t, ErrBalanceInFlight.Reason, inflight.Reason)
	require.NotSame(t, first, second)
	require.NotContains(t, second.Message, "US$-0.01")
	require.NotContains(t, second.Message, "US$0.125")
	require.Equal(t, originalBalanceMessage, ErrInsufficientBalance.Message)
	require.Equal(t, originalInflightMessage, ErrBalanceInFlight.Message)
}

func TestBalanceRejectionDetails_FixedChargeUsesVerifiedBalanceAndKnownAmount(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0}
	repo := &balanceLoadUserRepoStub{balance: 0.02}
	svc := NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	res, err := svc.ReserveFixedCharge(context.Background(), &User{ID: 1}, nil, nil, nil, 0.125)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Contains(t, pkgerrors.Message(err), "Current balance: US$0.02")
	require.Contains(t, pkgerrors.Message(err), "estimated reservation for this request: US$0.125")
	require.Equal(t, int64(1), repo.calls.Load())
	require.Zero(t, cache.deductCalls.Load(), "rejecting a request does not deduct its estimate")
}

func TestBalanceRejectionDetails_EligibilityDoesNotInventRequestEstimate(t *testing.T) {
	cache := &balanceEligibilityCacheStub{balance: 0}
	repo := &balanceLoadUserRepoStub{balance: -0.005}
	svc := NewBillingCacheService(cache, repo, nil, nil, nil, nil, &config.Config{}, nil)
	t.Cleanup(svc.Stop)

	err := svc.CheckBillingEligibility(context.Background(), &User{ID: 1}, nil, nil, nil, "")
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Contains(t, pkgerrors.Message(err), "Current balance: US$-0.005")
	require.Contains(t, pkgerrors.Message(err), "estimated reservation for this request: not calculated")
	require.Zero(t, cache.deductCalls.Load())
}

func TestBalanceRejectionDetails_InflightReportsCappedRequestAndOtherReservations(t *testing.T) {
	cache := newMemInflightCache(0.5)
	cache.res["other-request"] = 0.4
	svc := newInflightSvc(t, cache, 60)
	svc.cfg.Billing.InflightReservation.MaxReservationUSD = 0.25
	svc.userRepo = &inflightRecheckUserRepo{user: &User{ID: 1, Status: StatusActive, Balance: 0.5}}

	res, err := svc.ReserveInflight(context.Background(), &User{ID: 1}, nil, nil, 0.7)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrBalanceInFlight)
	require.Contains(t, pkgerrors.Message(err), "Current balance: US$0.50")
	require.Contains(t, pkgerrors.Message(err), "reserved by other requests: US$0.40")
	require.Contains(t, pkgerrors.Message(err), "estimated reservation for this request: US$0.25")
	require.NotContains(t, pkgerrors.Message(err), "US$0.70")
	require.Equal(t, 1, cache.count(), "rejected request must not register or clear a reservation")
	require.Len(t, cache.reserveIDs, 2, "retain one authoritative recheck and one atomic retry")
}

func TestBalanceRejectionDetails_InflightEmptyWalletRetainsKnownEstimate(t *testing.T) {
	cache := newMemInflightCache(0.5)
	cache.res["other-request"] = 0.4
	svc := newInflightSvc(t, cache, 60)
	svc.userRepo = &inflightRecheckUserRepo{user: &User{ID: 1, Status: StatusActive, Balance: 0}}

	res, err := svc.ReserveInflight(context.Background(), &User{ID: 1}, nil, nil, 0.7)
	require.Nil(t, res)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Contains(t, pkgerrors.Message(err), "Current balance: US$0.00")
	require.Contains(t, pkgerrors.Message(err), "estimated reservation for this request: US$0.70")
	require.Equal(t, 1, cache.count())
	require.Len(t, cache.reserveIDs, 1)
}
