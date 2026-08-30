package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
)

func normalizeGroupModelsListConfig(cfg GroupModelsListConfig) GroupModelsListConfig {
	out := GroupModelsListConfig{Enabled: cfg.Enabled}
	if len(cfg.Models) == 0 {
		return out
	}

	seen := make(map[string]struct{}, len(cfg.Models))
	out.Models = make([]string, 0, len(cfg.Models))
	for _, model := range cfg.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out.Models = append(out.Models, model)
	}
	if len(out.Models) == 0 {
		out.Models = nil
	}
	return out
}

func (g *Group) CustomModelsListEnabled() bool {
	return g != nil && g.ModelsListConfig.Enabled && len(g.ModelsListConfig.Models) > 0
}

// ResolveCustomModelsList applies the exact model-list semantics used by the
// gateway for a group with an enabled custom model list. Output order and
// casing follow the administrator's selected-model snapshot.
func ResolveCustomModelsList(platform string, availableModels, selectedModels []string) []string {
	fallbackModels := DefaultModelIDsForPlatform(platform)
	return FilterModelsByCustomList(
		CustomModelsListSource(platform, availableModels, fallbackModels),
		fallbackModels,
		selectedModels,
	)
}

// CustomModelsListSource merges Anthropic's mapped models with its curated
// OAuth/default model set. Other platforms use mappings directly and fall back
// only when no mapping exists.
func CustomModelsListSource(platform string, availableModels, fallbackModels []string) []string {
	if platform == PlatformAnthropic && len(availableModels) > 0 {
		return mergeCustomModelIDs(availableModels, fallbackModels)
	}
	return availableModels
}

// FilterModelsByCustomList filters selected models against exact or trailing-*
// available patterns. Matching, casing, ordering, and duplicate behavior are
// intentionally identical to the public gateway model-list contract.
func FilterModelsByCustomList(availableModels, fallbackModels, selectedModels []string) []string {
	if len(selectedModels) == 0 {
		return availableModels
	}
	source := availableModels
	if len(source) == 0 {
		source = fallbackModels
	}
	if len(source) == 0 {
		return nil
	}

	allowed := make([]string, 0, len(source))
	for _, model := range source {
		model = strings.TrimSpace(model)
		if model != "" {
			allowed = append(allowed, model)
		}
	}

	seen := make(map[string]struct{}, len(selectedModels))
	filtered := make([]string, 0, len(selectedModels))
	for _, model := range selectedModels {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		if !CustomModelsListAllowsModel(allowed, model) {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		filtered = append(filtered, model)
	}
	return filtered
}

func CustomModelsListAllowsModel(availablePatterns []string, model string) bool {
	for _, pattern := range availablePatterns {
		if pattern == model {
			return true
		}
		if strings.HasSuffix(pattern, "*") && strings.HasPrefix(model, strings.TrimSuffix(pattern, "*")) {
			return true
		}
	}
	normalizedClaudeModel := claude.NormalizeModelID(strings.TrimSuffix(model, "-thinking"))
	if normalizedClaudeModel != model {
		for _, pattern := range availablePatterns {
			if pattern == normalizedClaudeModel {
				return true
			}
		}
	}
	return false
}

func mergeCustomModelIDs(primary, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	merged := make([]string, 0, len(primary)+len(secondary))
	for _, models := range [][]string{primary, secondary} {
		for _, model := range models {
			model = strings.TrimSpace(model)
			if model == "" {
				continue
			}
			if _, ok := seen[model]; ok {
				continue
			}
			seen[model] = struct{}{}
			merged = append(merged, model)
		}
	}
	return merged
}
