package service

import "strings"

// retiredAnthropicDirectModelIDs follows Anthropic's direct-API lifecycle.
// Partner platforms such as Bedrock maintain independent retirement schedules.
var retiredAnthropicDirectModelIDs = map[string]struct{}{
	"claude-1.0":                 {},
	"claude-1.1":                 {},
	"claude-1.2":                 {},
	"claude-1.3":                 {},
	"claude-instant-1.0":         {},
	"claude-instant-1.1":         {},
	"claude-instant-1.2":         {},
	"claude-2.0":                 {},
	"claude-2.1":                 {},
	"claude-3-sonnet-20240229":   {},
	"claude-3-opus-20240229":     {},
	"claude-3-5-sonnet-20240620": {},
	"claude-3-5-sonnet-20241022": {},
	"claude-3-7-sonnet-20250219": {},
	"claude-3-5-haiku-20241022":  {},
	"claude-3-haiku-20240307":    {},
	"claude-sonnet-4-20250514":   {},
	"claude-opus-4-20250514":     {},
}

func shouldHideRetiredAnthropicModel(account *Account, model string) bool {
	if account == nil || account.Platform != PlatformAnthropic || account.IsBedrock() {
		return false
	}
	_, retired := retiredAnthropicDirectModelIDs[strings.ToLower(strings.TrimSpace(model))]
	return retired
}

func filterRetiredAnthropicModels(account *Account, models []string) []string {
	if len(models) == 0 {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, model := range models {
		if shouldHideRetiredAnthropicModel(account, model) {
			continue
		}
		filtered = append(filtered, model)
	}
	return filtered
}
