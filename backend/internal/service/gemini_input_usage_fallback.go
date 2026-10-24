package service

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/tidwall/gjson"
)

// prepareGeminiInputTokenEstimateFromRequest runs only for missing provider
// input/cache usage. The final HTTP request includes normalization, OAuth
// wrapping and any signature-recovery rewrite. Only a scalar escapes into the
// asynchronous settlement result; neither the request nor its contents do.
func prepareGeminiInputTokenEstimateFromRequest(result *ForwardResult, request *http.Request) {
	if result == nil || !anthropicInputUsageMissing(result.Usage) || request == nil || request.GetBody == nil {
		return
	}
	result.EstimatedInputTokens = 0
	body, err := request.GetBody()
	if err != nil || body == nil {
		return
	}
	defer func() { _ = body.Close() }()
	wireBody, err := io.ReadAll(body)
	if err == nil {
		result.EstimatedInputTokens = estimateGeminiInputTokensLocally(wireBody)
	}
}

// estimateGeminiInputTokensLocally uses the existing Gemini text-estimation
// convention. It is a fallback, not a provider measurement. Binary inputs,
// opaque signatures/cache references and generation settings are not text.
func estimateGeminiInputTokensLocally(body []byte) int {
	if !gjson.ValidBytes(body) {
		return 0
	}
	request := gjson.ParseBytes(body)
	if wrapped := request.Get("request"); wrapped.IsObject() {
		request = wrapped
	}
	total := 0
	countParts := func(parts gjson.Result) {
		parts.ForEach(func(_, part gjson.Result) bool {
			total += estimateTokensForText(part.Get("text").String())
			for _, pair := range [][2]string{{"functionCall", "function_call"}, {"functionResponse", "function_response"}} {
				if value := geminiInputUsageField(part, pair[0], pair[1]); value.IsObject() {
					total += estimateGeminiSemanticJSONTokens(value.Raw)
				}
			}
			return true
		})
	}
	countParts(geminiInputUsageField(request, "systemInstruction", "system_instruction").Get("parts"))
	request.Get("contents").ForEach(func(_, content gjson.Result) bool {
		countParts(content.Get("parts"))
		return true
	})
	request.Get("tools").ForEach(func(_, tool gjson.Result) bool {
		geminiInputUsageField(tool, "functionDeclarations", "function_declarations").ForEach(func(_, declaration gjson.Result) bool {
			total += estimateGeminiSemanticJSONTokens(declaration.Raw)
			return true
		})
		return true
	})
	return max(total, 0)
}

// Gemini accepts both JSON spellings; when both are present they still describe
// one field, so use the first present spelling instead of counting both.
func geminiInputUsageField(object gjson.Result, primary, alternate string) gjson.Result {
	if value := object.Get(primary); value.Exists() {
		return value
	}
	return object.Get(alternate)
}

func estimateGeminiSemanticJSONTokens(raw string) int {
	var value any
	if json.Unmarshal([]byte(raw), &value) != nil {
		return 0
	}
	stripGeminiOpaqueInputFields(value)
	semantic, err := json.Marshal(value)
	if err != nil {
		return 0
	}
	return estimateTokensForText(string(semantic))
}

func stripGeminiOpaqueInputFields(value any) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			switch key {
			case "inlineData", "inline_data", "fileData", "file_data", "thoughtSignature", "thought_signature":
				delete(value, key)
			default:
				stripGeminiOpaqueInputFields(child)
			}
		}
	case []any:
		for _, child := range value {
			stripGeminiOpaqueInputFields(child)
		}
	}
}
