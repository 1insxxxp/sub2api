package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const AccountExtraAntiTruncation = "anti_truncation"

type accountAntiTruncationRule struct {
	Model              string `json:"model"`
	Enabled            *bool  `json:"enabled"`
	MaxRounds          int    `json:"max_rounds,omitempty"`
	MaxOutputTokens    int    `json:"max_output_tokens,omitempty"`
	MaxDurationSeconds int    `json:"max_duration_seconds,omitempty"`
}

type accountAntiTruncationConfig struct {
	Version int                         `json:"version"`
	Rules   []accountAntiTruncationRule `json:"rules"`
}

func parseAccountAntiTruncation(raw any) (*accountAntiTruncationConfig, error) {
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config *accountAntiTruncationConfig
	if err := decoder.Decode(&config); err != nil {
		return nil, err
	}
	if config == nil || config.Version != 1 || config.Rules == nil {
		return nil, fmt.Errorf("anti_truncation requires version 1 and a rules array")
	}
	seen := make(map[string]struct{}, len(config.Rules))
	for _, rule := range config.Rules {
		if rule.Model == "" || len([]rune(rule.Model)) > 200 || strings.IndexFunc(rule.Model, func(r rune) bool {
			return unicode.IsSpace(r) || unicode.IsControl(r)
		}) >= 0 {
			return nil, fmt.Errorf("anti_truncation model must contain 1–200 characters without whitespace or controls")
		}
		if _, duplicate := seen[rule.Model]; duplicate {
			return nil, fmt.Errorf("anti_truncation model rules must be unique")
		}
		seen[rule.Model] = struct{}{}
		if rule.Enabled == nil {
			return nil, fmt.Errorf("anti_truncation rule requires enabled")
		}
		if rule.MaxRounds < 0 || rule.MaxRounds > 3 || rule.MaxOutputTokens < 0 || rule.MaxOutputTokens > 32768 || rule.MaxDurationSeconds < 0 || rule.MaxDurationSeconds > 300 {
			return nil, fmt.Errorf("anti_truncation limits must be integer values: rounds 0–3, output 0–32768, duration 0–300 seconds")
		}
	}
	return config, nil
}

// ValidateAccountAntiTruncationExtra rejects invalid administrative input. An
// absent field preserves compatibility; an explicit empty rules array disables
// continuation for every model on this account.
func ValidateAccountAntiTruncationExtra(extra map[string]any) error {
	raw, exists := extra[AccountExtraAntiTruncation]
	if !exists {
		return nil
	}
	if _, err := parseAccountAntiTruncation(raw); err != nil {
		return infraerrors.BadRequest("INVALID_ACCOUNT_ANTI_TRUNCATION", err.Error())
	}
	return nil
}

// ResolveAccountAntiTruncationPolicy resolves against the selected account on
// each attempt. configured reports whether the account owns the policy, even
// when malformed or unmatched (both fail closed rather than using legacy).
func ResolveAccountAntiTruncationPolicy(account *Account, requestedModel, routeModel string, legacy domain.AntiTruncationPolicy) (policy domain.AntiTruncationPolicy, configured bool) {
	if account == nil {
		return legacy, false
	}
	raw, exists := account.Extra[AccountExtraAntiTruncation]
	if !exists {
		return legacy, false
	}
	config, err := parseAccountAntiTruncation(raw)
	if err != nil {
		return domain.AntiTruncationPolicy{}, true
	}
	for _, model := range []string{strings.TrimSpace(requestedModel), strings.TrimSpace(routeModel)} {
		if model == "" {
			continue
		}
		for _, rule := range config.Rules {
			if rule.Model == model {
				return domain.AntiTruncationPolicy{
					Enabled: *rule.Enabled, MaxRounds: rule.MaxRounds,
					MaxOutputTokens: rule.MaxOutputTokens, MaxDurationSeconds: rule.MaxDurationSeconds,
				}, true
			}
		}
	}
	return domain.AntiTruncationPolicy{}, true
}
