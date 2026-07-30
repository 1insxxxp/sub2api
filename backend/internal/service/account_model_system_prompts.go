package service

import (
	"fmt"
	"strings"
)

const MaxModelSystemPromptBytes = 32 * 1024

func NormalizeModelSystemPrompts(in map[string]string) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for rawModel, rawPrompt := range in {
		model := strings.TrimSpace(rawModel)
		if model == "" {
			return nil, fmt.Errorf("model_system_prompts contains an empty model")
		}
		prompt := strings.TrimSpace(rawPrompt)
		if prompt == "" {
			return nil, fmt.Errorf("model_system_prompts[%q] must not be empty", model)
		}
		if len(prompt) > MaxModelSystemPromptBytes {
			return nil, fmt.Errorf("model_system_prompts[%q] exceeds %d bytes", model, MaxModelSystemPromptBytes)
		}
		if _, exists := out[model]; exists {
			return nil, fmt.Errorf("model_system_prompts contains duplicate model %q", model)
		}
		out[model] = prompt
	}
	return out, nil
}
