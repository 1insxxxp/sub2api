package domain

import (
	"fmt"
	"strings"
)

type AntiTruncationPolicy struct {
	Enabled            bool `json:"enabled"`
	MaxRounds          int  `json:"max_rounds,omitempty"`
	MaxOutputTokens    int  `json:"max_output_tokens,omitempty"`
	MaxDurationSeconds int  `json:"max_duration_seconds,omitempty"`
}
type ModelAlias struct {
	Alias          string               `json:"alias"`
	UpstreamModel  string               `json:"upstream_model"`
	AntiTruncation AntiTruncationPolicy `json:"anti_truncation,omitempty"`
}

func NormalizeModelAliases(in []ModelAlias) ([]ModelAlias, error) {
	if len(in) == 0 {
		return []ModelAlias{}, nil
	}
	out := make([]ModelAlias, 0, len(in))
	seen := map[string]struct{}{}
	for _, a := range in {
		a.Alias = strings.TrimSpace(a.Alias)
		a.UpstreamModel = strings.TrimSpace(a.UpstreamModel)
		if a.Alias == "" || a.UpstreamModel == "" {
			return nil, fmt.Errorf("model alias and upstream_model are required")
		}
		if len([]rune(a.Alias)) > 100 || len([]rune(a.UpstreamModel)) > 200 {
			return nil, fmt.Errorf("model alias or upstream model is too long")
		}
		key := strings.ToLower(a.Alias)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate model alias %q", a.Alias)
		}
		if a.AntiTruncation.MaxRounds < 0 || a.AntiTruncation.MaxOutputTokens < 0 || a.AntiTruncation.MaxDurationSeconds < 0 {
			return nil, fmt.Errorf("anti truncation limits must be non-negative")
		}
		if a.AntiTruncation.MaxRounds > 3 {
			return nil, fmt.Errorf("max_rounds must be <= 3")
		}
		if a.AntiTruncation.MaxDurationSeconds > 300 {
			return nil, fmt.Errorf("max_duration_seconds must be <= 300")
		}
		if a.AntiTruncation.MaxOutputTokens > 32768 {
			return nil, fmt.Errorf("max_output_tokens must be <= 32768")
		}
		seen[key] = struct{}{}
		out = append(out, a)
	}
	return out, nil
}
func ResolveModelAlias(aliases []ModelAlias, requested string) (string, bool) {
	key := strings.ToLower(strings.TrimSpace(requested))
	for _, a := range aliases {
		if strings.ToLower(a.Alias) == key {
			return a.UpstreamModel, true
		}
	}
	return requested, false
}
