//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func traversalTestContext(ctx context.Context) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil).WithContext(ctx)
	return c
}

func TestModelFirstOutputTraversalCancelsWaitingWithDistinctCause(t *testing.T) {
	c := traversalTestContext(context.Background())
	cleanup := startModelFirstOutputTraversal(c, time.Now().Add(20*time.Millisecond))
	defer cleanup()
	ctx := c.Request.Context()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("account selection or concurrency wait was not canceled by the shared budget")
	}
	if !ModelFirstOutputTraversalBudgetExceeded(ctx) {
		t.Fatalf("budget cancellation must be distinguishable from client cancellation: %v", context.Cause(ctx))
	}
}

func TestModelFirstOutputTraversalSemanticOutputStopsBudget(t *testing.T) {
	c := traversalTestContext(context.Background())
	cleanup := startModelFirstOutputTraversal(c, time.Now().Add(30*time.Millisecond))
	defer cleanup()
	ctx := c.Request.Context()
	stopModelFirstOutputTraversal(ctx)
	select {
	case <-ctx.Done():
		t.Fatalf("a started response must be allowed to stream beyond the traversal budget: %v", context.Cause(ctx))
	case <-time.After(70 * time.Millisecond):
	}
}

func TestModelFirstOutputTraversalPreservesClientCancellation(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	c := traversalTestContext(parent)
	cleanup := startModelFirstOutputTraversal(c, time.Now().Add(time.Hour))
	defer cleanup()
	ctx := c.Request.Context()
	cancel()
	<-ctx.Done()
	if ModelFirstOutputTraversalBudgetExceeded(ctx) {
		t.Fatal("client cancellation was misclassified as a traversal timeout")
	}
}

func TestModelFirstOutputTraversalCleanupReleasesContextWithoutFalseTimeout(t *testing.T) {
	parent := context.Background()
	c := traversalTestContext(parent)
	cleanup := startModelFirstOutputTraversal(c, time.Now().Add(time.Hour))
	ctx := c.Request.Context()
	cleanup()
	cleanup()
	if ctx.Err() == nil {
		t.Fatal("cleanup must release the child context")
	}
	if ModelFirstOutputTraversalBudgetExceeded(ctx) {
		t.Fatal("cleanup was misclassified as a timeout")
	}
	if c.Request.Context() != parent {
		t.Fatal("cleanup must restore the original context for response logging")
	}
}

func TestModelFirstOutputTraversalSecondStartDoesNotResetBudget(t *testing.T) {
	c := traversalTestContext(context.Background())
	cleanup := startModelFirstOutputTraversal(c, time.Now().Add(20*time.Millisecond))
	defer cleanup()
	ctx := c.Request.Context()
	duplicateCleanup := startModelFirstOutputTraversal(c, time.Now().Add(time.Hour))
	duplicateCleanup()
	if ctx != c.Request.Context() {
		t.Fatal("later attempts must retain the original request budget")
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("a later attempt extended the traversal budget")
	}
}
