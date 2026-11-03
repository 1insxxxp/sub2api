package domain

import (
	"fmt"
	"strings"
	"unicode"
)

type AntiTruncationPolicy struct {
	Enabled            bool `json:"enabled"`
	MaxRounds          int  `json:"max_rounds,omitempty"`
	MaxOutputTokens    int  `json:"max_output_tokens,omitempty"`
	MaxDurationSeconds int  `json:"max_duration_seconds,omitempty"`
}

type ModelAlias struct {
	Alias         string `json:"alias"`
	UpstreamModel string `json:"upstream_model"`
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
		if strings.IndexFunc(a.Alias, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 || strings.IndexFunc(a.UpstreamModel, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
			return nil, fmt.Errorf("model names cannot contain whitespace or control characters")
		}
		if strings.EqualFold(a.Alias, a.UpstreamModel) {
			return nil, fmt.Errorf("model alias must differ from its upstream model")
		}
		key := strings.ToLower(a.Alias)
		if _, ok := seen[key]; ok {
			return nil, fmt.Errorf("duplicate model alias %q", a.Alias)
		}
		seen[key] = struct{}{}
		out = append(out, a)
	}
	for _, a := range out {
		if _, exists := seen[strings.ToLower(a.UpstreamModel)]; exists {
			return nil, fmt.Errorf("upstream model %q cannot be another configured alias", a.UpstreamModel)
		}
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
