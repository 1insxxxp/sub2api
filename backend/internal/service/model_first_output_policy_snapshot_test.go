//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestModelFirstOutputTraversalFreezesPolicyForAllAttempts(t *testing.T) {
	initial := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":30,"switch_seconds":30,"hard_cap_seconds":300}}`}}
	for _, tc := range []struct {
		name     string
		platform string
		settings *SettingService
	}{
		{"disabled_during_request", PlatformGemini, &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":false}`}}},
		{"settings_unavailable", PlatformGemini, nil},
		{"antigravity_account_disabled_during_request", PlatformAntigravity, &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":false}`}}},
		{"antigravity_account_settings_unavailable", PlatformAntigravity, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
			cleanup := StartModelFirstOutputTraversal(c, initial, PlatformGemini, "gemini-3.1-pro-preview", true)
			defer cleanup()
			state := modelFirstOutputTraversalFromContext(c.Request.Context())
			if state == nil {
				t.Fatal("expected shared traversal budget")
			}
			for attempt := 0; attempt < 2; attempt++ {
				_, guard, policy := newModelFirstOutputGuard(c.Request.Context(), c, tc.settings, tc.platform, "gemini-3.1-pro-preview")
				if guard == nil {
					t.Fatal("in-flight configuration changes must not remove the guard that stops the request budget on first output")
				}
				if policy.SwitchSeconds != 30 || policy.HardCapSeconds != 300 {
					t.Fatalf("attempt %d lost frozen policy: %+v", attempt, policy)
				}
				if attempt == 1 {
					guard.Stop()
				}
				guard.Close()
			}
			state.mu.Lock()
			active := state.active
			state.mu.Unlock()
			if active {
				t.Fatal("semantic output did not release the request budget")
			}
		})
	}
}

func TestModelFirstOutputTraversalKeepsInitiallyDisabledPolicy(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	disabled := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":false}`}}
	cleanup := StartModelFirstOutputTraversal(c, disabled, PlatformGemini, "gemini-3.1-pro-preview", true)
	defer cleanup()
	enabled := &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":30,"switch_seconds":30,"hard_cap_seconds":300}}`}}
	_, guard, policy := newModelFirstOutputGuard(context.Background(), c, enabled, PlatformGemini, "gemini-3.1-pro-preview")
	if guard != nil {
		guard.Close()
		t.Fatal("enabling a setting must only change subsequent requests")
	}
	if policy.Enabled {
		t.Fatal("expected frozen disabled policy")
	}
}
