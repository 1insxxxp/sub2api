//go:build unit

package service

import (
	"context"
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
