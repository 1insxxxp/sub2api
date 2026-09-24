package service

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type modelFirstOutputContextKey string

const modelFirstOutputBudgetStartKey modelFirstOutputContextKey = "sub2api.model_first_output_budget_start"
const modelFirstOutputGuardKey modelFirstOutputContextKey = "sub2api.model_first_output_guard"

// modelFirstOutputGuard cancels only the pre-output upstream attempt. Once a
// semantic event is observed, Stop keeps the request context alive for the
// remainder of the response.
type modelFirstOutputGuard struct {
	ctx      context.Context
	cancel   context.CancelFunc
	timer    *time.Timer
	timedOut atomic.Bool
	stopped  atomic.Bool
}

func ensureModelFirstOutputBudget(c *gin.Context, now time.Time) time.Time {
	if c == nil {
		return now
	}
	if value, ok := c.Get(string(modelFirstOutputBudgetStartKey)); ok {
		if started, ok := value.(time.Time); ok && !started.IsZero() {
			return started
		}
	}
	c.Set(string(modelFirstOutputBudgetStartKey), now)
	return now
}

func modelFirstOutputAttemptDeadline(c *gin.Context, policy ModelFirstOutputTimeoutPolicy, now time.Time) time.Time {
	start := ensureModelFirstOutputBudget(c, now)
	deadline := now.Add(time.Duration(policy.SwitchSeconds) * time.Second)
	if policy.HardCapSeconds > 0 {
		hardDeadline := start.Add(time.Duration(policy.HardCapSeconds) * time.Second)
		if hardDeadline.Before(deadline) {
			deadline = hardDeadline
		}
	}
	return deadline
}

func newModelFirstOutputGuard(ctx context.Context, c *gin.Context, settings *SettingService, platform, model string) (context.Context, *modelFirstOutputGuard, ModelFirstOutputTimeoutPolicy) {
	policy := ModelFirstOutputTimeoutPolicy{}
	if settings != nil {
		policy = settings.ResolveModelFirstOutputTimeout(ctx, platform, model)
	}
	if !modelFirstOutputTimeoutEnabled(ctx, policy) {
		return ctx, nil, policy
	}
	deadline := modelFirstOutputAttemptDeadline(c, policy, time.Now())
	remaining := time.Until(deadline)
	if remaining <= 0 {
		remaining = time.Nanosecond
	}
	guardCtx, cancel := context.WithCancel(ctx)
	guard := &modelFirstOutputGuard{ctx: guardCtx, cancel: cancel}
	guard.timer = time.AfterFunc(remaining, func() {
		guard.timedOut.Store(true)
		cancel()
	})
	return context.WithValue(guardCtx, modelFirstOutputGuardKey, guard), guard, policy
}

func modelFirstOutputGuardFromContext(ctx context.Context) *modelFirstOutputGuard {
	if ctx == nil {
		return nil
	}
	guard, _ := ctx.Value(modelFirstOutputGuardKey).(*modelFirstOutputGuard)
	return guard
}

func (g *modelFirstOutputGuard) Stop() {
	if g == nil || !g.stopped.CompareAndSwap(false, true) {
		return
	}
	if g.timer != nil {
		g.timer.Stop()
	}
}

func (g *modelFirstOutputGuard) Close() {
	if g == nil {
		return
	}
	g.Stop()
	if g.cancel != nil {
		g.cancel()
	}
}

func (g *modelFirstOutputGuard) TimedOut() bool {
	return g != nil && g.timedOut.Load()
}
