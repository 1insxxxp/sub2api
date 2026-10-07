//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestModelFirstOutputGuardStopKeepsStreamingContextAlive(t *testing.T) {
	settings := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":2}}`}}
	c := &gin.Context{}
	ctx, guard, _ := newModelFirstOutputGuard(context.Background(), c, settings, PlatformOpenAI, "gpt-test")
	if guard == nil {
		t.Fatal("expected guard")
	}
	guard.Stop()
	defer guard.Close()
	select {
	case <-ctx.Done():
		t.Fatalf("context canceled after semantic output: %v", ctx.Err())
	case <-time.After(1100 * time.Millisecond):
	}
}

func TestModelFirstOutputAttemptDeadlineHonorsHardCapAcrossAttempts(t *testing.T) {
	c := &gin.Context{}
	now := time.Now()
	policy := ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 1, SwitchSeconds: 10, HardCapSeconds: 12}
	first := modelFirstOutputAttemptDeadline(c, policy, now)
	if first.Sub(now) != 10*time.Second {
		t.Fatalf("first deadline = %s", first.Sub(now))
	}
	second := modelFirstOutputAttemptDeadline(c, policy, now.Add(9*time.Second))
	if second.Sub(now) != 12*time.Second {
		t.Fatalf("hard cap deadline = %s", second.Sub(now))
	}
}

func TestModelFirstOutputBudgetTimeoutStopsTraversal(t *testing.T) {
	c := &gin.Context{}
	now := time.Now()
	policy := ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 60, SwitchSeconds: 60, HardCapSeconds: 300}
	modelFirstOutputAttemptDeadline(c, policy, now.Add(-301*time.Second))
	err := newModelFirstOutputTimeoutFailoverError(c, nil, "gemini-3.1-pro-preview", policy, nil)
	if err.ShouldRetryNextAccount() {
		t.Fatal("an exhausted request budget must not start another account")
	}
	if err.Reason != GatewayFailureReason("first_output_budget_exhausted") {
		t.Fatalf("reason = %s", err.Reason)
	}
}

func TestModelFirstOutputAttemptDeadlineCannotExtendExistingBudget(t *testing.T) {
	c := &gin.Context{}
	now := time.Now()
	policy := ModelFirstOutputTimeoutPolicy{Enabled: true, SwitchSeconds: 60, HardCapSeconds: 300}
	modelFirstOutputAttemptDeadline(c, policy, now)
	policy.HardCapSeconds = 900
	deadline := modelFirstOutputAttemptDeadline(c, policy, now.Add(290*time.Second))
	if !deadline.Equal(now.Add(300 * time.Second)) {
		t.Fatalf("an in-flight request budget was extended: %s", deadline.Sub(now))
	}
}

func TestModelFirstOutputGuardClosePreservesTraversalForNextAccount(t *testing.T) {
	c := &gin.Context{Request: httptest.NewRequest("POST", "/v1/chat/completions", nil)}
	finish := startModelFirstOutputTraversal(c, time.Now().Add(40*time.Millisecond))
	defer finish()
	settings := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":300}}`}}
	_, guard, _ := newModelFirstOutputGuard(c.Request.Context(), c, settings, PlatformGemini, "gemini-pro")
	guard.Close()
	select {
	case <-c.Request.Context().Done():
		if !ModelFirstOutputTraversalBudgetExceeded(c.Request.Context()) {
			t.Fatal("expected traversal timeout after the failed account closes")
		}
	case <-time.After(time.Second):
		t.Fatal("closing an account disabled the total traversal budget")
	}
}

func TestModelFirstOutputGuardSemanticOutputStopsBothTimers(t *testing.T) {
	c := &gin.Context{Request: httptest.NewRequest("POST", "/v1/chat/completions", nil)}
	finish := startModelFirstOutputTraversal(c, time.Now().Add(30*time.Millisecond))
	defer finish()
	settings := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":300}}`}}
	ctx, guard, _ := newModelFirstOutputGuard(c.Request.Context(), c, settings, PlatformGemini, "gemini-pro")
	defer guard.Close()
	guard.Stop()
	select {
	case <-ctx.Done():
		t.Fatalf("semantic response was interrupted by a pre-output timer: %v", ctx.Err())
	case <-time.After(60 * time.Millisecond):
	}
}

type modelFirstOutputSettingRepo struct{ value string }

func (r *modelFirstOutputSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}
func (r *modelFirstOutputSettingRepo) GetValue(context.Context, string) (string, error) {
	return r.value, nil
}
func (r *modelFirstOutputSettingRepo) Set(context.Context, string, string) error { return nil }
func (r *modelFirstOutputSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return nil, nil
}
func (r *modelFirstOutputSettingRepo) SetMultiple(context.Context, map[string]string) error {
	return nil
}
func (r *modelFirstOutputSettingRepo) GetAll(context.Context) (map[string]string, error) {
	return nil, nil
}
func (r *modelFirstOutputSettingRepo) Delete(context.Context, string) error { return nil }
