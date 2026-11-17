//go:build unit

package handler

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type inflightPolicySnapshotTestKey struct{}

// The handler treats a prepared policy snapshot as opaque context data. This
// double isolates that handoff without loading settings or calling upstream.
type zeroCostPolicyPreparingEstimator struct {
	policy        *service.OpenAIFastPolicySettings
	prepareCalls  int
	estimateCalls int
	preparedCtx   context.Context
	estimatedCtx  context.Context
	preparedKey   *service.APIKey
	preparedReq   service.InflightEstimateRequest
}

func (e *zeroCostPolicyPreparingEstimator) PrepareInflightEstimateContext(ctx context.Context, key *service.APIKey, req service.InflightEstimateRequest) context.Context {
	e.prepareCalls++
	e.preparedKey = key
	e.preparedReq = req
	e.preparedCtx = context.WithValue(ctx, inflightPolicySnapshotTestKey{}, e.policy)
	return e.preparedCtx
}

func (e *zeroCostPolicyPreparingEstimator) EstimateInflightReservation(ctx context.Context, _ *service.APIKey, _ service.InflightEstimateRequest) (float64, bool) {
	e.estimateCalls++
	e.estimatedCtx = ctx
	return 0, true
}

func TestReserveInflightBalance_ZeroEstimatePreservesPreparedPolicyContext(t *testing.T) {
	cache := newHandlerInflightCache(10)
	cfg := &config.Config{}
	cfg.Billing.InflightReservation = config.InflightReservationConfig{Enabled: true, FailClosedOnUnpriced: true}
	billing := service.NewBillingCacheService(cache, nil, nil, nil, nil, nil, cfg, nil)
	t.Cleanup(billing.Stop)
	group := &service.Group{ID: 81, Platform: service.PlatformOpenAI, Status: service.StatusActive,
		Hydrated: true, SubscriptionType: service.SubscriptionTypeStandard}
	key := &service.APIKey{ID: 82, UserID: 83, User: &service.User{ID: 83}, GroupID: &group.ID, Group: group}
	policy := &service.OpenAIFastPolicySettings{Rules: []service.OpenAIFastPolicyRule{{
		ServiceTier: service.OpenAIFastTierUltrafast, Scope: service.BetaPolicyScopeAll, Action: service.BetaPolicyActionFilter,
	}}}
	estimator := &zeroCostPolicyPreparingEstimator{policy: policy}
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), ctxkey.Group, group))
	t.Cleanup(cancel)
	c := newInflightTestGinContext()
	c.Request = c.Request.WithContext(ctx)
	req := tokenInflightEstimate("gpt-6.1-sol", []byte(`{"model":"gpt-6.1-sol","max_tokens":32,"service_tier":"ultrafast"}`))

	done, err := reserveInflightBalance(c, billing, estimator, key, nil, req)
	require.NoError(t, err)
	require.NotNil(t, done)
	t.Cleanup(done)
	require.Equal(t, 1, estimator.prepareCalls, "load the request policy once before estimation")
	require.Equal(t, 1, estimator.estimateCalls)
	require.Same(t, key, estimator.preparedKey)
	require.Equal(t, req, estimator.preparedReq)
	require.Same(t, estimator.preparedCtx, estimator.estimatedCtx)
	require.Same(t, policy, estimator.estimatedCtx.Value(inflightPolicySnapshotTestKey{}))

	// Forwarding reads c.Request.Context() after this helper returns. The zero
	// estimate branch must propagate the same snapshot despite creating no hold.
	forwardCtx := c.Request.Context()
	require.Same(t, estimator.preparedCtx, forwardCtx)
	require.Same(t, policy, forwardCtx.Value(inflightPolicySnapshotTestKey{}))
	require.Same(t, group, forwardCtx.Value(ctxkey.Group), "retain the authenticated source group")
	require.Nil(t, service.InflightReservationFromContext(forwardCtx))
	require.Zero(t, cache.count())
	cancel()
	require.ErrorIs(t, forwardCtx.Err(), context.Canceled, "retain request cancellation")
}
