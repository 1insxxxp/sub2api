package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	modelFirstOutputTimeoutMaxSeconds = 15 * 60
)

// ModelFirstOutputTimeoutPolicy controls the wait before a text request is
// eligible for a serial account switch. It never limits output length.
type ModelFirstOutputTimeoutPolicy struct {
	Enabled        bool `json:"enabled"`
	TargetSeconds  int  `json:"target_seconds"`
	SwitchSeconds  int  `json:"switch_seconds"`
	HardCapSeconds int  `json:"hard_cap_seconds"`
}

// ModelFirstOutputTimeoutSettings contains a global policy and optional
// platform/model overrides. Model keys are case-insensitive on read.
type ModelFirstOutputTimeoutSettings struct {
	Enabled   bool                                     `json:"enabled"`
	Default   ModelFirstOutputTimeoutPolicy            `json:"default"`
	Platforms map[string]ModelFirstOutputTimeoutPolicy `json:"platforms,omitempty"`
	Models    map[string]ModelFirstOutputTimeoutPolicy `json:"models,omitempty"`
	Profiles  map[string]ModelFirstOutputTimeoutPolicy `json:"profiles"`
}

func DefaultModelFirstOutputTimeoutSettings() *ModelFirstOutputTimeoutSettings {
	return &ModelFirstOutputTimeoutSettings{
		Enabled:   true,
		Default:   ModelFirstOutputTimeoutPolicy{Enabled: true, TargetSeconds: 20, SwitchSeconds: 20, HardCapSeconds: 60},
		Platforms: map[string]ModelFirstOutputTimeoutPolicy{},
		Models:    map[string]ModelFirstOutputTimeoutPolicy{},
		Profiles: map[string]ModelFirstOutputTimeoutPolicy{
			"gemini_flash":    {true, 10, 10, 30},
			"gemini_pro":      {true, 20, 20, 60},
			"gemini_thinking": {true, 30, 30, 90},
		},
	}
}

func modelFirstOutputProfile(model string) string {
	model = strings.ToLower(strings.TrimSpace(model))
	if !strings.HasPrefix(model, "gemini-") {
		return ""
	}
	if strings.Contains(model, "thinking") {
		return "gemini_thinking"
	}
	if strings.Contains(model, "flash") {
		return "gemini_flash"
	}
	if strings.Contains(model, "pro") {
		return "gemini_pro"
	}
	return ""
}

func normalizeModelFirstOutputPolicy(policy ModelFirstOutputTimeoutPolicy) (ModelFirstOutputTimeoutPolicy, error) {
	if policy.TargetSeconds < 0 || policy.SwitchSeconds < 0 || policy.HardCapSeconds < 0 {
		return policy, errors.New("timeout values cannot be negative")
	}
	if !policy.Enabled && policy.TargetSeconds == 0 && policy.SwitchSeconds == 0 && policy.HardCapSeconds == 0 {
		return policy, nil
	}
	if policy.TargetSeconds <= 0 || policy.SwitchSeconds <= 0 || policy.HardCapSeconds <= 0 {
		return policy, errors.New("target_seconds, switch_seconds and hard_cap_seconds must all be positive")
	}
	if policy.SwitchSeconds > policy.HardCapSeconds {
		return policy, errors.New("switch_seconds cannot exceed hard_cap_seconds")
	}
	if policy.TargetSeconds > policy.SwitchSeconds {
		return policy, errors.New("target_seconds cannot exceed switch_seconds")
	}
	if policy.HardCapSeconds > modelFirstOutputTimeoutMaxSeconds {
		return policy, fmt.Errorf("hard_cap_seconds cannot exceed %d", modelFirstOutputTimeoutMaxSeconds)
	}
	return policy, nil
}

func normalizeModelFirstOutputSettings(settings *ModelFirstOutputTimeoutSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}
	defaultPolicy, err := normalizeModelFirstOutputPolicy(settings.Default)
	if err != nil {
		return fmt.Errorf("default policy: %w", err)
	}
	settings.Default = defaultPolicy
	if settings.Default.SwitchSeconds == 0 {
		return errors.New("default policy must be configured")
	}
	platforms := make(map[string]ModelFirstOutputTimeoutPolicy, len(settings.Platforms))
	for key, policy := range settings.Platforms {
		normalized, err := normalizeModelFirstOutputPolicy(policy)
		if err != nil {
			return fmt.Errorf("platform %q: %w", key, err)
		}
		platforms[strings.ToLower(strings.TrimSpace(key))] = normalized
	}
	models := make(map[string]ModelFirstOutputTimeoutPolicy, len(settings.Models))
	for key, policy := range settings.Models {
		normalized, err := normalizeModelFirstOutputPolicy(policy)
		if err != nil {
			return fmt.Errorf("model %q: %w", key, err)
		}
		models[strings.ToLower(strings.TrimSpace(key))] = normalized
	}
	for key, policy := range settings.Profiles {
		if key != "gemini_flash" && key != "gemini_pro" && key != "gemini_thinking" {
			return fmt.Errorf("unknown profile %q", key)
		}
		if _, err := normalizeModelFirstOutputPolicy(policy); err != nil {
			return fmt.Errorf("profile %q: %w", key, err)
		}
	}
	settings.Platforms = platforms
	settings.Models = models
	return nil
}

// GetModelFirstOutputTimeoutSettings reads once per incoming request from the
// shared settings store. This intentionally avoids an instance-local cache:
// saving on one ingress must take effect for new requests on both ingresses.
func (s *SettingService) GetModelFirstOutputTimeoutSettings(ctx context.Context) *ModelFirstOutputTimeoutSettings {
	defaults := DefaultModelFirstOutputTimeoutSettings()
	if s == nil || s.settingRepo == nil {
		defaults.Enabled = false
		return defaults
	}
	if ctx == nil {
		ctx = context.Background()
	}
	dbCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyModelFirstOutputTimeoutSettings)
	if errors.Is(err, ErrSettingNotFound) || (err == nil && strings.TrimSpace(raw) == "") {
		return defaults
	}
	if err != nil {
		defaults.Enabled = false
		return defaults
	}
	candidate := &ModelFirstOutputTimeoutSettings{}
	if json.Unmarshal([]byte(raw), candidate) != nil || normalizeModelFirstOutputSettings(candidate) != nil {
		defaults.Enabled = false
		return defaults
	}
	return candidate
}

func (s *SettingService) SetModelFirstOutputTimeoutSettings(ctx context.Context, settings *ModelFirstOutputTimeoutSettings) error {
	if err := normalizeModelFirstOutputSettings(settings); err != nil {
		return err
	}
	data, err := json.Marshal(settings)
	if err != nil {
		return fmt.Errorf("marshal model first-output timeout settings: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyModelFirstOutputTimeoutSettings, string(data)); err != nil {
		return err
	}
	return nil
}

// ResolveModelFirstOutputTimeout applies model > platform > built-in defaults.
func (s *SettingService) ResolveModelFirstOutputTimeout(ctx context.Context, platform, model string) ModelFirstOutputTimeoutPolicy {
	settings := s.GetModelFirstOutputTimeoutSettings(ctx)
	policy := settings.Default
	if profile, ok := settings.Profiles[modelFirstOutputProfile(model)]; ok {
		policy = profile
	}
	if override, ok := settings.Platforms[strings.ToLower(strings.TrimSpace(platform))]; ok {
		policy = override
	}
	if override, ok := settings.Models[strings.ToLower(strings.TrimSpace(model))]; ok {
		policy = override
	}
	if !settings.Enabled {
		policy.Enabled = false
	}
	return policy
}

func cloneModelFirstOutputSettings(src *ModelFirstOutputTimeoutSettings) *ModelFirstOutputTimeoutSettings {
	if src == nil {
		return DefaultModelFirstOutputTimeoutSettings()
	}
	dst := *src
	dst.Platforms = map[string]ModelFirstOutputTimeoutPolicy{}
	for key, value := range src.Platforms {
		dst.Platforms[key] = value
	}
	dst.Models = map[string]ModelFirstOutputTimeoutPolicy{}
	for key, value := range src.Models {
		dst.Models[key] = value
	}
	dst.Profiles = map[string]ModelFirstOutputTimeoutPolicy{}
	for key, value := range src.Profiles {
		dst.Profiles[key] = value
	}
	return &dst
}

func modelFirstOutputTimeoutEnabled(ctx context.Context, policy ModelFirstOutputTimeoutPolicy) bool {
	return policy.Enabled && policy.SwitchSeconds > 0 && ctx != nil && ctx.Err() == nil
}

func newModelFirstOutputTimeoutFailoverError(c *gin.Context, account *Account, model string, policy ModelFirstOutputTimeoutPolicy, headers http.Header) *UpstreamFailoverError {
	if headers == nil {
		headers = make(http.Header)
	}
	if account != nil {
		if account.Platform == PlatformGemini || account.Platform == PlatformAntigravity {
			recordGeminiFirstOutput(account.ID, model, 0, true)
		}
		appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
			ProxyID:            opsUpstreamProxyID(account),
			ProxyName:          opsUpstreamProxyName(account),
			Platform:           account.Platform,
			AccountID:          account.ID,
			AccountName:        account.Name,
			UpstreamStatusCode: http.StatusGatewayTimeout,
			UpstreamRequestID:  headers.Get("x-request-id"),
			Kind:               "first_output_timeout",
			Message:            "upstream produced no semantic output before the configured deadline",
			Detail:             fmt.Sprintf("model=%s switch_seconds=%d hard_cap_seconds=%d", model, policy.SwitchSeconds, policy.HardCapSeconds),
		})
	}
	return &UpstreamFailoverError{
		StatusCode:               http.StatusGatewayTimeout,
		ResponseBody:             []byte(`{"type":"error","error":{"type":"first_output_timeout","message":"Upstream produced no semantic output before the configured deadline"}}`),
		ResponseHeaders:          headers.Clone(),
		SafeToFailoverAfterWrite: true,
		NextAccountAction:        NextAccountRetry,
		Reason:                   GatewayFailureReason("first_output_timeout"),
	}
}
