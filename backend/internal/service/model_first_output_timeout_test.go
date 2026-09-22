//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestResolveModelFirstOutputTimeout_UsesGeminiModelDefaults(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})

	flash := svc.ResolveModelFirstOutputTimeout(context.Background(), PlatformGemini, "gemini-3.8-flash")
	pro := svc.ResolveModelFirstOutputTimeout(context.Background(), PlatformGemini, "gemini-3.1-pro")
	thinking := svc.ResolveModelFirstOutputTimeout(context.Background(), PlatformGemini, "gemini-thinking")

	require.Equal(t, 10, flash.TargetSeconds)
	require.Equal(t, 10, flash.SwitchSeconds)
	require.Equal(t, 30, flash.HardCapSeconds)
	require.Equal(t, 20, pro.TargetSeconds)
	require.Equal(t, 20, pro.SwitchSeconds)
	require.Equal(t, 60, pro.HardCapSeconds)
	require.Equal(t, 30, thinking.TargetSeconds)
	require.Equal(t, 30, thinking.SwitchSeconds)
	require.Equal(t, 90, thinking.HardCapSeconds)
}

func TestSetModelFirstOutputTimeoutSettings_RejectsInvalidOrder(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	settings := DefaultModelFirstOutputTimeoutSettings()
	settings.Default.TargetSeconds = 31
	settings.Default.SwitchSeconds = 30

	require.Error(t, svc.SetModelFirstOutputTimeoutSettings(context.Background(), settings))
}

func TestResolveModelFirstOutputTimeout_ModelOverrideWins(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewSettingService(repo, &config.Config{})
	settings := DefaultModelFirstOutputTimeoutSettings()
	settings.Platforms[PlatformGemini] = ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 12, SwitchSeconds: 18, HardCapSeconds: 36}
	settings.Models["gemini-3.8-flash"] = ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 8, SwitchSeconds: 12, HardCapSeconds: 24}
	require.NoError(t, svc.SetModelFirstOutputTimeoutSettings(context.Background(), settings))

	got := svc.ResolveModelFirstOutputTimeout(context.Background(), PlatformGemini, "GEMINI-3.8-FLASH")
	require.Equal(t, 8, got.TargetSeconds)
	require.Equal(t, 12, got.SwitchSeconds)
}

func TestResolveModelFirstOutputTimeout_GlobalAndProfilesEditable(t *testing.T) {
	svc := NewSettingService(newMockSettingRepo(), &config.Config{})
	settings := DefaultModelFirstOutputTimeoutSettings()
	settings.Default = ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 15, SwitchSeconds: 25, HardCapSeconds: 50}
	settings.Profiles["gemini_flash"] = ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 7, SwitchSeconds: 11, HardCapSeconds: 22}
	require.NoError(t, svc.SetModelFirstOutputTimeoutSettings(context.Background(), settings))
	require.Equal(t, 25, svc.ResolveModelFirstOutputTimeout(context.Background(), PlatformAnthropic, "claude-sonnet").SwitchSeconds)
	require.Equal(t, 11, svc.ResolveModelFirstOutputTimeout(context.Background(), PlatformGemini, "gemini-3.8-flash").SwitchSeconds)
	// Platform is a transport; names such as product or flashdance must not be classified as Gemini.
	require.Equal(t, 25, svc.ResolveModelFirstOutputTimeout(context.Background(), PlatformOpenAI, "flashdance-product").SwitchSeconds)
}

func TestModelFirstOutputTimeoutSettings_UpdateVisibleAcrossInstances(t *testing.T) {
	repo := newMockSettingRepo()
	first := NewSettingService(repo, &config.Config{})
	second := NewSettingService(repo, &config.Config{})
	_ = second.GetModelFirstOutputTimeoutSettings(context.Background())
	settings := DefaultModelFirstOutputTimeoutSettings()
	settings.Enabled = false
	require.NoError(t, first.SetModelFirstOutputTimeoutSettings(context.Background(), settings))
	require.False(t, second.GetModelFirstOutputTimeoutSettings(context.Background()).Enabled)
}
