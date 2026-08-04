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

var shutdownGeminiModelIDs = map[string]struct{}{
	"gemini-3.1-flash-lite-preview":                     {},
	"gemini-2.5-pro-preview-03-25":                      {},
	"gemini-2.5-pro-preview-05-06":                      {},
	"gemini-2.5-pro-preview-06-05":                      {},
	"gemini-2.5-flash-lite-preview-09-2025":             {},
	"gemini-2.5-flash-preview-05-20":                    {},
	"gemini-2.5-flash-preview-09-25":                    {},
	"gemini-2.5-flash-image-preview":                    {},
	"gemini-2.0-flash":                                  {},
	"gemini-2.0-flash-001":                              {},
	"gemini-2.0-flash-lite":                             {},
	"gemini-2.0-flash-lite-001":                         {},
	"gemini-2.0-flash-preview-image-generation":         {},
	"gemini-2.0-flash-exp-image-generation":             {},
	"gemini-2.0-flash-lite-preview":                     {},
	"gemini-2.0-flash-lite-preview-02-05":               {},
	"gemini-2.0-flash-exp":                              {},
	"gemini-2.0-pro-exp":                                {},
	"gemini-2.0-pro-exp-02-05":                          {},
	"gemini-2.0-flash-thinking-exp":                     {},
	"gemini-2.0-flash-thinking-exp-01-21":               {},
	"gemini-2.0-flash-thinking-exp-1219":                {},
	"gemini-2.0-flash-live-001":                         {},
	"gemini-live-2.5-flash-preview":                     {},
	"gemini-2.5-flash-preview-native-audio-dialog":      {},
	"gemini-2.5-flash-exp-native-audio-thinking-dialog": {},
	"text-embedding-004":                                {},
	"embedding-001":                                     {},
	"embedding-gecko-001":                               {},
	"gemini-embedding-exp":                              {},
	"gemini-embedding-exp-03-07":                        {},
	"imagen-3.0-generate-002":                           {},
	"imagen-4.0-generate-preview-06-06":                 {},
	"imagen-4.0-ultra-generate-preview-06-06":           {},
	"veo-3.0-generate-preview":                          {},
	"veo-3.0-fast-generate-preview":                     {},
}

func shouldHideUnavailableProviderModel(account *Account, model string) bool {
	if account == nil {
		return false
	}
	model = strings.ToLower(strings.TrimSpace(model))
	switch account.Platform {
	case PlatformAnthropic:
		if account.IsBedrock() {
			return false
		}
		_, unavailable := retiredAnthropicDirectModelIDs[model]
		return unavailable
	case PlatformGemini:
		_, unavailable := shutdownGeminiModelIDs[model]
		return unavailable
	default:
		return false
	}
}

func filterUnavailableProviderModels(account *Account, models []string) []string {
	if len(models) == 0 {
		return models
	}
	filtered := make([]string, 0, len(models))
	for _, model := range models {
		if shouldHideUnavailableProviderModel(account, model) {
			continue
		}
		filtered = append(filtered, model)
	}
	return filtered
}
