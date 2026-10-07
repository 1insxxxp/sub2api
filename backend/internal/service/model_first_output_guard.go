package service

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
)

type modelFirstOutputContextKey string

const modelFirstOutputBudgetStartKey modelFirstOutputContextKey = "sub2api.model_first_output_budget_start"
const modelFirstOutputBudgetDeadlineKey modelFirstOutputContextKey = "sub2api.model_first_output_budget_deadline"
const modelFirstOutputGuardKey modelFirstOutputContextKey = "sub2api.model_first_output_guard"

func modelFirstOutputBudgetExhausted(c *gin.Context, now time.Time) bool {
	if c == nil {
		return false
	}
	value, ok := c.Get(string(modelFirstOutputBudgetDeadlineKey))
	deadline, valid := value.(time.Time)
	return ok && valid && !deadline.IsZero() && !now.Before(deadline)
}

// modelFirstOutputGuard cancels only the pre-output upstream attempt. Once a
// semantic event is observed, Stop keeps the request context alive for the
// remainder of the response.
type modelFirstOutputGuard struct {
	ctx           context.Context
	cancel        context.CancelFunc
	timer         *time.Timer
	timedOut      atomic.Bool
	stopped       atomic.Bool
	stopKeepalive func()
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
		if c != nil {
			if value, ok := c.Get(string(modelFirstOutputBudgetDeadlineKey)); ok {
				if existing, valid := value.(time.Time); valid && !existing.IsZero() && existing.Before(hardDeadline) {
					hardDeadline = existing
				}
			}
			c.Set(string(modelFirstOutputBudgetDeadlineKey), hardDeadline)
		}
		if hardDeadline.Before(deadline) {
			deadline = hardDeadline
		}
	}
	return deadline
}

func newModelFirstOutputGuard(ctx context.Context, c *gin.Context, settings *SettingService, platform, model string) (context.Context, *modelFirstOutputGuard, ModelFirstOutputTimeoutPolicy) {
	policy := modelFirstOutputRequestPolicy(ctx, c, settings, platform, model)
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
	if platform == PlatformGemini {
		guard.stopKeepalive = startModelFirstOutputKeepalive(c, 15*time.Second)
	}
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
	if g.stopKeepalive != nil {
		g.stopKeepalive()
	}
	stopModelFirstOutputTraversal(g.ctx)
}

func (g *modelFirstOutputGuard) Close() {
	if g == nil {
		return
	}
	// Closing an unsuccessful account attempt must leave the shared traversal
	// budget active for the next account. Only semantic output calls Stop.
	g.stopped.Store(true)
	if g.timer != nil {
		g.timer.Stop()
	}
	if g.stopKeepalive != nil {
		g.stopKeepalive()
	}
	if g.cancel != nil {
		g.cancel()
	}
}

func (g *modelFirstOutputGuard) TimedOut() bool {
	return g != nil && g.timedOut.Load()
}
