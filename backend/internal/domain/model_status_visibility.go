package domain

import "strings"

// GroupModelStatusVisibility controls which models are shown on the model
// status monitor for a group. It is intentionally independent from
// GroupModelAllowlist: hiding a model here never changes request admission.
type GroupModelStatusVisibility struct {
	Enabled bool     `json:"enabled"`
	Models  []string `json:"models,omitempty"`
}

func (c GroupModelStatusVisibility) Normalize() GroupModelStatusVisibility {
	out := GroupModelStatusVisibility{Enabled: c.Enabled}
	seen := make(map[string]struct{}, len(c.Models))
	for _, model := range c.Models {
		model = strings.TrimSpace(model)
		if model == "" {
			continue
		}
		key := strings.ToLower(model)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out.Models = append(out.Models, model)
	}
	return out
}

func (c GroupModelStatusVisibility) Allows(model string) bool {
	if !c.Enabled || len(c.Models) == 0 {
		return true
	}
	model = strings.ToLower(strings.TrimSpace(model))
	for _, entry := range c.Models {
		entry = strings.ToLower(strings.TrimSpace(entry))
		if entry == "*" || entry == model {
			return true
		}
		if strings.HasSuffix(entry, "*") && strings.HasPrefix(model, strings.TrimSuffix(entry, "*")) {
			return true
		}
	}
	return false
}

// Filter keeps source order and applies the visibility configuration.
func (c GroupModelStatusVisibility) Filter(models []string) []string {
	if !c.Enabled || len(c.Models) == 0 {
		return models
	}
	out := make([]string, 0, len(models))
	for _, model := range models {
		if c.Allows(model) {
			out = append(out, model)
		}
	}
	return out
}
