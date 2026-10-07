package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type modelFirstOutputTraversalKey struct{}

var errModelFirstOutputTraversalBudget = errors.New("model first-output account traversal budget exhausted")

const ModelFirstOutputTraversalTimeoutMessage = "Timed out waiting for output across available accounts. Please retry."

// Unlike context.WithDeadline, this budget can stop once semantic output starts
// without canceling the context used to read the remainder of a long response.
type modelFirstOutputTraversal struct {
	mu     sync.Mutex
	active bool
	timer  *time.Timer
}

func startModelFirstOutputTraversal(c *gin.Context, deadline time.Time) func() {
	if c == nil || c.Request == nil || modelFirstOutputTraversalFromContext(c.Request.Context()) != nil {
		return func() {}
	}
	parent := c.Request.Context()
	ctx, cancel := context.WithCancelCause(parent)
	state := &modelFirstOutputTraversal{active: true}
	ctx = context.WithValue(ctx, modelFirstOutputTraversalKey{}, state)
	c.Request = c.Request.WithContext(ctx)
	state.mu.Lock()
	state.timer = time.AfterFunc(time.Until(deadline), func() {
		state.mu.Lock()
		if !state.active {
			state.mu.Unlock()
			return
		}
		state.active = false
		state.mu.Unlock()
		cancel(errModelFirstOutputTraversalBudget)
	})
	state.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			stopModelFirstOutputTraversal(ctx)
			cancel(nil)
			// Avoid reporting our deferred cleanup as a client disconnect in
			// middleware that inspects the request after the handler returns.
			c.Request = c.Request.WithContext(parent)
		})
	}
}

func modelFirstOutputTraversalFromContext(ctx context.Context) *modelFirstOutputTraversal {
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(modelFirstOutputTraversalKey{}).(*modelFirstOutputTraversal)
	return state
}

func stopModelFirstOutputTraversal(ctx context.Context) {
	state := modelFirstOutputTraversalFromContext(ctx)
	if state == nil {
		return
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.active = false
	if state.timer != nil {
		state.timer.Stop()
	}
}

// ModelFirstOutputTraversalBudgetExceeded distinguishes our pre-output budget
// from an actual client cancellation so handlers can return a useful 504.
func ModelFirstOutputTraversalBudgetExceeded(ctx context.Context) bool {
	return ctx != nil && errors.Is(context.Cause(ctx), errModelFirstOutputTraversalBudget)
}
