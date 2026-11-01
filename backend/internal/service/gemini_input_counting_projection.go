package service

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

func geminiInputCountingCurrentAttempt(c *gin.Context) *GeminiUpstreamAttempt {
	if c == nil {
		return nil
	}
	value, _ := c.Get(geminiUpstreamAttemptKey)
	attempt, _ := value.(*GeminiUpstreamAttempt)
	if attempt == nil || attempt.ended {
		return nil
	}
	return attempt
}

func geminiInputCountingAttempt(c *gin.Context) *GeminiUpstreamAttempt {
	attempt := geminiInputCountingCurrentAttempt(c)
	if attempt == nil || attempt.inputCounting == nil || attempt.inputCounting.AccountID != attempt.event.AccountID {
		return nil
	}
	return attempt
}

// Client adapters receive a copy; provider accounting and cancellation recovery
// continue to use the untouched upstream usage on the attempt/result.
func geminiInputCountingClientUsage(c *gin.Context, raw *ClaudeUsage) ClaudeUsage {
	if raw == nil {
		return ClaudeUsage{}
	}
	projected := *raw
	attempt := geminiInputCountingAttempt(c)
	if attempt == nil {
		return projected
	}
	result := &ForwardResult{
		Usage: *raw, GeminiInputCounting: attempt.inputCounting,
		GeminiUsageFallback: &GeminiUsageFallback{UsageFields: attempt.event.UsageFields},
	}
	if c.Request != nil {
		result.Outcome = ResponseOutcomeSnapshotFromContext(c.Request.Context())
	}
	if attempt.event.UsageFields&1 != 0 {
		result.InputTokensSource = UsageTokenSourceUpstream
	}
	if input, discounted := geminiCustomerInputTokens(result); discounted {
		projected.InputTokens = input
	}
	return projected
}

func projectGeminiInputCountingChatUsage(c *gin.Context, response *apicompat.ChatCompletionsResponse, raw *ClaudeUsage) {
	if response == nil || response.Usage == nil || raw == nil {
		return
	}
	usage := geminiInputCountingClientUsage(c, raw)
	if usage.InputTokens == raw.InputTokens {
		return
	}
	discount := raw.InputTokens - usage.InputTokens
	response.Usage.PromptTokens -= discount
	response.Usage.TotalTokens -= discount
}

func projectGeminiInputCountingMessagesUsage(c *gin.Context, response map[string]any, raw *ClaudeUsage) {
	if usage, ok := response["usage"].(map[string]any); ok && raw != nil {
		projected := geminiInputCountingClientUsage(c, raw)
		if projected.InputTokens != raw.InputTokens {
			usage["input_tokens"] = projected.InputTokens
		}
	}
}

// Native usage frames are cumulative and can report prompt/cache in different
// frames. Once both are known, repeat the effective prompt count on the latest
// usage frame so its terminal measurement agrees with billing and other APIs.
func projectGeminiInputCountingNativeUsage(c *gin.Context, raw []byte) []byte {
	attempt := geminiInputCountingCurrentAttempt(c)
	if attempt == nil || (attempt.inputCounting == nil && !attempt.nativeInputCountEmitted) || !gjson.ValidBytes(raw) {
		return raw
	}
	if root := gjson.ParseBytes(raw); root.IsArray() {
		updated := raw
		for index, item := range root.Array() {
			projected := projectGeminiInputCountingNativeUsage(c, []byte(item.Raw))
			if string(projected) != item.Raw {
				var err error
				updated, err = sjson.SetRawBytes(updated, strconv.Itoa(index), projected)
				if err != nil {
					return raw
				}
			}
		}
		return updated
	}
	path := "usageMetadata"
	hasMetadata := true
	if !gjson.GetBytes(raw, path).IsObject() {
		path = "response.usageMetadata"
		if !gjson.GetBytes(raw, path).IsObject() {
			// A late media response can invalidate a previously emitted discount
			// without repeating usage. Restore only already measured input.
			if !attempt.nativeInputCountEmitted || attempt.inputCounting != nil || attempt.event.UsageFields&1 == 0 {
				return raw
			}
			path = "usageMetadata"
			if gjson.GetBytes(raw, "response").IsObject() {
				path = "response.usageMetadata"
			}
			hasMetadata = false
		}
	}
	if attempt.event.UsageFields&1 == 0 {
		return raw
	}
	gross, cache := int(attempt.event.InputTokens), int(attempt.event.CacheReadTokens)
	// A reported zero on this particular frame stays zero even when an earlier
	// or later cumulative frame reports a positive prompt measurement.
	if prompt := gjson.GetBytes(raw, path+".promptTokenCount"); prompt.Type == gjson.Number && prompt.Int() >= 0 {
		gross = int(prompt.Int())
		if gross == 0 {
			return raw
		}
	}
	if cached := gjson.GetBytes(raw, path+".cachedContentTokenCount"); cached.Type == gjson.Number && cached.Int() >= 0 {
		cache = int(cached.Int())
	}
	usage := ClaudeUsage{InputTokens: max(0, gross-cache), CacheReadInputTokens: cache, OutputTokens: int(attempt.event.OutputTokens)}
	if measured := extractGeminiUsage(raw); measured != nil {
		usage.ImageOutputTokens = measured.ImageOutputTokens
	}
	projected := geminiInputCountingClientUsage(c, &usage)
	discounted := projected.InputTokens < usage.InputTokens
	if !discounted && !attempt.nativeInputCountEmitted {
		return raw
	}
	if !hasMetadata && discounted {
		return raw
	}
	prompt := gross
	if discounted {
		prompt = projected.InputTokens + cache
		attempt.nativeInputCountEmitted = true
	}
	updated, err := sjson.SetBytes(raw, path+".promptTokenCount", prompt)
	if err != nil {
		return raw
	}
	if total := gjson.GetBytes(raw, path+".totalTokenCount"); total.Type == gjson.Number && total.Int() >= int64(gross) {
		updated, err = sjson.SetBytes(updated, path+".totalTokenCount", total.Int()-int64(gross-prompt))
		if err != nil {
			return raw
		}
	}
	return updated
}

func geminiInputCountingResponseHasMedia(raw []byte) bool {
	root := gjson.ParseBytes(raw)
	if response := root.Get("response"); response.IsObject() {
		root = response
	}
	media := false
	root.Get("candidates").ForEach(func(_, candidate gjson.Result) bool {
		candidate.Get("content.parts").ForEach(func(_, part gjson.Result) bool {
			media = part.Get("inlineData").Exists() || part.Get("fileData").Exists()
			return !media
		})
		return !media
	})
	if media {
		return true
	}
	for _, field := range []string{"promptTokensDetails", "candidatesTokensDetails"} {
		root.Get("usageMetadata." + field).ForEach(func(_, detail gjson.Result) bool {
			modality := strings.ToUpper(detail.Get("modality").String())
			media = modality != "" && modality != "TEXT"
			return !media
		})
		if media {
			return true
		}
	}
	return false
}
