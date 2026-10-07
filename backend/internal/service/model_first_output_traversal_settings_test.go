//go:build unit

package service

import (
	"context"
	"testing"
	"time"
)

func TestModelFirstOutputTraversalScope(t *testing.T) {
	settings := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":300}}`}}
	for _, tc := range []struct {
		name, platform, model string
		stream                bool
	}{
		{"non-stream", PlatformGemini, "gemini-3.1-pro-preview", false},
		{"other-platform", PlatformAnthropic, "claude-opus-4-6", true},
		{"native-antigravity", PlatformAntigravity, "gemini-3.1-pro-preview", true},
		{"image", PlatformGemini, "gemini-3-pro-image-preview", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := traversalTestContext(context.Background())
			parent := c.Request.Context()
			cleanup := StartModelFirstOutputTraversal(c, settings, tc.platform, tc.model, tc.stream)
			defer cleanup()
			if c.Request.Context() != parent {
				t.Fatal("ineligible request received a traversal budget")
			}
		})
	}
}

func TestModelFirstOutputTraversalAccountCleanupRetainsTotalBudget(t *testing.T) {
	settings := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":300}}`}}
	c := traversalTestContext(context.Background())
	cleanup := startModelFirstOutputTraversal(c, time.Now().Add(100*time.Millisecond))
	defer cleanup()
	ctx := c.Request.Context()
	_, guard, _ := newModelFirstOutputGuard(ctx, c, settings, PlatformGemini, "gemini-3.1-pro-preview")
	if guard == nil {
		t.Fatal("expected per-account guard")
	}
	guard.Close()
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("cleaning up a failed account must not stop the remaining traversal budget")
	}
	if !ModelFirstOutputTraversalBudgetExceeded(ctx) {
		t.Fatal("expected total budget exhaustion after account cleanup")
	}
}

func TestModelFirstOutputTraversalAccountOutputStopsTotalBudget(t *testing.T) {
	settings := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":300}}`}}
	c := traversalTestContext(context.Background())
	cleanup := startModelFirstOutputTraversal(c, time.Now().Add(100*time.Millisecond))
	defer cleanup()
	ctx := c.Request.Context()
	_, guard, _ := newModelFirstOutputGuard(ctx, c, settings, PlatformGemini, "gemini-3.1-pro-preview")
	if guard == nil {
		t.Fatal("expected per-account guard")
	}
	defer guard.Close()
	guard.Stop()
	select {
	case <-ctx.Done():
		t.Fatal("semantic output must stop the total traversal timer")
	case <-time.After(150 * time.Millisecond):
	}
}
