package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
)

const (
	defaultAuthIPAutoBlockWindowMinutes       = 10
	defaultAuthIPAutoBlockRegisterThreshold   = 5
	defaultAuthIPAutoBlockVerifyCodeThreshold = 12
	defaultAuthIPAutoBlockLoginFailThreshold  = 30
)

type AuthIPAutoBlockSettings struct {
	Enabled               bool `json:"enabled"`
	WindowMinutes         int  `json:"window_minutes"`
	RegisterThreshold     int  `json:"register_threshold"`
	VerifyCodeThreshold   int  `json:"verify_code_threshold"`
	LoginFailureThreshold int  `json:"login_failure_threshold"`
}

type AuthIPBlacklistSettings struct {
	Enabled   bool                     `json:"enabled"`
	Rules     []string                 `json:"rules"`
	AutoBlock *AuthIPAutoBlockSettings `json:"auto_block"`
}

func DefaultAuthIPAutoBlockSettings() *AuthIPAutoBlockSettings {
	return &AuthIPAutoBlockSettings{
		Enabled:               false,
		WindowMinutes:         defaultAuthIPAutoBlockWindowMinutes,
		RegisterThreshold:     defaultAuthIPAutoBlockRegisterThreshold,
		VerifyCodeThreshold:   defaultAuthIPAutoBlockVerifyCodeThreshold,
		LoginFailureThreshold: defaultAuthIPAutoBlockLoginFailThreshold,
	}
}

func (s *SettingService) GetAuthIPBlacklistSettings(ctx context.Context) (*AuthIPBlacklistSettings, error) {
	keys := []string{
		SettingKeyAuthIPBlacklistEnabled,
		SettingKeyAuthIPBlacklistRules,
		SettingKeyAuthIPAutoBlockEnabled,
		SettingKeyAuthIPAutoBlockWindowMinutes,
		SettingKeyAuthIPAutoBlockRegisterThreshold,
		SettingKeyAuthIPAutoBlockVerifyCodeThreshold,
		SettingKeyAuthIPAutoBlockLoginFailureThreshold,
	}
	values, err := s.settingRepo.GetMultiple(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("get auth ip blacklist settings: %w", err)
	}

	return &AuthIPBlacklistSettings{
		Enabled:   values[SettingKeyAuthIPBlacklistEnabled] == "true",
		Rules:     normalizeAuthIPBlacklistRules(values[SettingKeyAuthIPBlacklistRules]),
		AutoBlock: parseAuthIPAutoBlockSettings(values),
	}, nil
}

func (s *SettingService) GetAuthIPAutoBlockSettings(ctx context.Context) (*AuthIPAutoBlockSettings, error) {
	settings, err := s.GetAuthIPBlacklistSettings(ctx)
	if err != nil {
		return nil, err
	}
	if settings == nil || settings.AutoBlock == nil {
		return DefaultAuthIPAutoBlockSettings(), nil
	}
	return settings.AutoBlock, nil
}

func (s *SettingService) SetAuthIPBlacklistSettings(ctx context.Context, settings *AuthIPBlacklistSettings) (*AuthIPBlacklistSettings, error) {
	if settings == nil {
		settings = &AuthIPBlacklistSettings{}
	}
	autoBlock := normalizeAuthIPAutoBlockSettings(settings.AutoBlock)
	rulesJSON, err := json.Marshal(normalizeAuthIPBlacklistRulesFromSlice(settings.Rules))
	if err != nil {
		return nil, fmt.Errorf("marshal auth ip blacklist rules: %w", err)
	}

	updates := map[string]string{
		SettingKeyAuthIPBlacklistEnabled:               strconv.FormatBool(settings.Enabled),
		SettingKeyAuthIPBlacklistRules:                 string(rulesJSON),
		SettingKeyAuthIPAutoBlockEnabled:               strconv.FormatBool(autoBlock.Enabled),
		SettingKeyAuthIPAutoBlockWindowMinutes:         strconv.Itoa(autoBlock.WindowMinutes),
		SettingKeyAuthIPAutoBlockRegisterThreshold:     strconv.Itoa(autoBlock.RegisterThreshold),
		SettingKeyAuthIPAutoBlockVerifyCodeThreshold:   strconv.Itoa(autoBlock.VerifyCodeThreshold),
		SettingKeyAuthIPAutoBlockLoginFailureThreshold: strconv.Itoa(autoBlock.LoginFailureThreshold),
	}
	if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
		return nil, fmt.Errorf("set auth ip blacklist settings: %w", err)
	}
	return s.GetAuthIPBlacklistSettings(ctx)
}

func (s *SettingService) AddAuthIPBlacklistRule(ctx context.Context, rule string) error {
	normalized := normalizeAuthIPRule(rule)
	if normalized == "" {
		return nil
	}
	settings, err := s.GetAuthIPBlacklistSettings(ctx)
	if err != nil {
		return err
	}
	rules := normalizeAuthIPBlacklistRulesFromSlice(append(settings.Rules, normalized))
	rulesJSON, err := json.Marshal(rules)
	if err != nil {
		return fmt.Errorf("marshal auth ip blacklist rules: %w", err)
	}
	return s.settingRepo.SetMultiple(ctx, map[string]string{
		SettingKeyAuthIPBlacklistEnabled: "true",
		SettingKeyAuthIPBlacklistRules:   string(rulesJSON),
	})
}

func (s *SettingService) IsAuthIPBlocked(ctx context.Context, clientIP string) (bool, error) {
	parsedIP := net.ParseIP(strings.TrimSpace(clientIP))
	if parsedIP == nil {
		return false, nil
	}
	settings, err := s.GetAuthIPBlacklistSettings(ctx)
	if err != nil {
		return false, err
	}
	if settings == nil || !settings.Enabled {
		return false, nil
	}
	for _, rule := range settings.Rules {
		if ip := net.ParseIP(rule); ip != nil && ip.Equal(parsedIP) {
			return true, nil
		}
		if _, network, err := net.ParseCIDR(rule); err == nil && network.Contains(parsedIP) {
			return true, nil
		}
	}
	return false, nil
}

func parseAuthIPAutoBlockSettings(values map[string]string) *AuthIPAutoBlockSettings {
	settings := DefaultAuthIPAutoBlockSettings()
	settings.Enabled = values[SettingKeyAuthIPAutoBlockEnabled] == "true"
	settings.WindowMinutes = clampIntSetting(values[SettingKeyAuthIPAutoBlockWindowMinutes], settings.WindowMinutes, 1, 1440)
	settings.RegisterThreshold = clampIntSetting(values[SettingKeyAuthIPAutoBlockRegisterThreshold], settings.RegisterThreshold, 1, 1000)
	settings.VerifyCodeThreshold = clampIntSetting(values[SettingKeyAuthIPAutoBlockVerifyCodeThreshold], settings.VerifyCodeThreshold, 1, 1000)
	settings.LoginFailureThreshold = clampIntSetting(values[SettingKeyAuthIPAutoBlockLoginFailureThreshold], settings.LoginFailureThreshold, 1, 1000)
	return settings
}

func normalizeAuthIPAutoBlockSettings(settings *AuthIPAutoBlockSettings) *AuthIPAutoBlockSettings {
	defaults := DefaultAuthIPAutoBlockSettings()
	if settings == nil {
		return defaults
	}
	defaults.Enabled = settings.Enabled
	defaults.WindowMinutes = clampInt(settings.WindowMinutes, 1, 1440, defaultAuthIPAutoBlockWindowMinutes)
	defaults.RegisterThreshold = clampInt(settings.RegisterThreshold, 1, 1000, defaultAuthIPAutoBlockRegisterThreshold)
	defaults.VerifyCodeThreshold = clampInt(settings.VerifyCodeThreshold, 1, 1000, defaultAuthIPAutoBlockVerifyCodeThreshold)
	defaults.LoginFailureThreshold = clampInt(settings.LoginFailureThreshold, 1, 1000, defaultAuthIPAutoBlockLoginFailThreshold)
	return defaults
}

func normalizeAuthIPBlacklistRules(raw string) []string {
	var rules []string
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &rules); err != nil {
		return []string{}
	}
	return normalizeAuthIPBlacklistRulesFromSlice(rules)
}

func normalizeAuthIPBlacklistRulesFromSlice(rules []string) []string {
	seen := make(map[string]struct{}, len(rules))
	normalized := make([]string, 0, len(rules))
	for _, rule := range rules {
		candidate := normalizeAuthIPRule(rule)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		normalized = append(normalized, candidate)
	}
	return normalized
}

func normalizeAuthIPRule(rule string) string {
	rule = strings.TrimSpace(rule)
	if rule == "" {
		return ""
	}
	if ip := net.ParseIP(rule); ip != nil {
		return ip.String()
	}
	if _, network, err := net.ParseCIDR(rule); err == nil {
		return network.String()
	}
	return ""
}

func clampIntSetting(raw string, fallback, minValue, maxValue int) int {
	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return clampInt(value, minValue, maxValue, fallback)
}

func clampInt(value, minValue, maxValue, fallback int) int {
	if value < minValue || value > maxValue {
		return fallback
	}
	return value
}
