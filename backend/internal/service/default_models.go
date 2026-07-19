package service

import (
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/antigravity"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/geminicli"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
)

// DefaultModelIDsForPlatform returns the curated default model IDs used when a
// schedulable account exists but does not define an explicit model_mapping.
func DefaultModelIDsForPlatform(platform string) []string {
	switch platform {
	case PlatformOpenAI:
		return openai.DefaultModelIDs()
	case PlatformGemini:
		ids := make([]string, 0, len(geminicli.DefaultModels))
		for _, model := range geminicli.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformAntigravity:
		models := antigravity.DefaultModels()
		ids := make([]string, 0, len(models))
		for _, model := range models {
			ids = append(ids, model.ID)
		}
		return ids
	case PlatformAnthropic:
		ids := make([]string, 0, len(claude.DefaultModels)+len(antigravity.DefaultModels()))
		for _, model := range claude.DefaultModels {
			ids = append(ids, model.ID)
		}
		for _, model := range antigravity.DefaultModels() {
			ids = append(ids, model.ID)
		}
		return mergeDefaultModelIDs(ids, nil)
	case PlatformGrok:
		return xai.DefaultModelIDs()
	default:
		ids := make([]string, 0, len(claude.DefaultModels))
		for _, model := range claude.DefaultModels {
			ids = append(ids, model.ID)
		}
		return ids
	}
}

func mergeDefaultModelIDs(primary, secondary []string) []string {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	merged := make([]string, 0, len(primary)+len(secondary))
	for _, model := range append(primary, secondary...) {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		merged = append(merged, model)
	}
	return merged
}
