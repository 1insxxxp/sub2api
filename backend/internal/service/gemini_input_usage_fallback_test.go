//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiInputUsageFallbackCountsSemanticInputsOnly(t *testing.T) {
	base := map[string]any{
		"systemInstruction": map[string]any{"parts": []any{map[string]any{"text": "You are a helpful assistant."}}},
		"contents":          []any{map[string]any{"role": "user", "parts": []any{map[string]any{"text": "Explain the weather forecast."}}}},
	}
	marshal := func(v any) []byte { b, err := json.Marshal(v); require.NoError(t, err); return b }
	baseline := estimateGeminiInputTokensLocally(marshal(base))
	require.Positive(t, baseline)
	base["generationConfig"] = map[string]any{"maxOutputTokens": 100000, "responseJsonSchema": strings.Repeat("ignored ", 1000)}
	base["cachedContent"] = "projects/opaque/locations/global/cachedContents/" + strings.Repeat("opaque", 1000)
	base["contents"].([]any)[0].(map[string]any)["parts"] = []any{
		map[string]any{"text": "Explain the weather forecast.", "thoughtSignature": strings.Repeat("signature", 1000)},
		map[string]any{"inlineData": map[string]any{"mimeType": "image/png", "data": strings.Repeat("a", 100000)}},
		map[string]any{"fileData": map[string]any{"fileUri": "gs://" + strings.Repeat("opaque", 1000)}},
	}
	require.Equal(t, baseline, estimateGeminiInputTokensLocally(marshal(base)))
	require.Equal(t, baseline, estimateGeminiInputTokensLocally(marshal(map[string]any{"model": "gemini", "project": "opaque", "request": base})))

	for _, extra := range []map[string]any{
		{"functionCall": map[string]any{"name": "weather", "args": map[string]any{"city": "Hangzhou", "detail": strings.Repeat("forecast ", 100)}}},
		{"functionResponse": map[string]any{"name": "weather", "response": map[string]any{"forecast": strings.Repeat("sunny ", 100)}}},
	} {
		base["contents"].([]any)[0].(map[string]any)["parts"] = []any{map[string]any{"text": "Explain the weather forecast."}, extra}
		require.Greater(t, estimateGeminiInputTokensLocally(marshal(base)), baseline)
	}
	base["contents"].([]any)[0].(map[string]any)["parts"] = []any{map[string]any{"text": "Explain the weather forecast."}}
	base["tools"] = []any{map[string]any{"functionDeclarations": []any{map[string]any{"name": "weather", "description": strings.Repeat("Read a weather forecast. ", 100), "parameters": map[string]any{"type": "OBJECT", "properties": map[string]any{"city": map[string]any{"type": "STRING"}}}}}}}
	require.Greater(t, estimateGeminiInputTokensLocally(marshal(base)), baseline)
	require.Zero(t, estimateGeminiInputTokensLocally([]byte("not-json")))
}

func TestGeminiInputUsageFallbackPreservesProviderUsageWithoutReadingBody(t *testing.T) {
	for _, usage := range []ClaudeUsage{{InputTokens: 12}, {CacheReadInputTokens: 1200}, {CacheCreationInputTokens: 123}, {CacheCreation5mTokens: 3}, {CacheCreation1hTokens: 4}} {
		result := &ForwardResult{Usage: usage}
		req := &http.Request{GetBody: func() (io.ReadCloser, error) {
			t.Fatal("provider usage must skip request read and estimation")
			return nil, nil
		}}
		prepareGeminiInputTokenEstimateFromRequest(result, req)
		require.Zero(t, result.EstimatedInputTokens)
		require.Equal(t, usage, result.Usage)
	}
	prepareGeminiInputTokenEstimateFromRequest(nil, &http.Request{GetBody: func() (io.ReadCloser, error) { t.Fatal("failed attempt has no result to estimate"); return nil, nil }})
}

func TestGeminiInputUsageFallbackSupportsSnakeCaseWithoutDoubleCounting(t *testing.T) {
	camel := []byte(`{"systemInstruction":{"parts":[{"text":"Follow the weather guidance."}]},"contents":[{"parts":[{"text":"Weather request"},{"functionCall":{"name":"weather","args":{"city":"Hangzhou"}}},{"functionResponse":{"name":"weather","response":{"forecast":"sunny"}}}]}],"tools":[{"functionDeclarations":[{"name":"weather","description":"Read the weather forecast.","parameters":{"type":"OBJECT"}}]}]}`)
	snake := []byte(strings.NewReplacer("systemInstruction", "system_instruction", "functionCall", "function_call", "functionResponse", "function_response", "functionDeclarations", "function_declarations").Replace(string(camel)))
	want := estimateGeminiInputTokensLocally(camel)
	require.Positive(t, want)
	require.Equal(t, want, estimateGeminiInputTokensLocally(snake))
	var duplicated any
	require.NoError(t, json.Unmarshal(camel, &duplicated))
	var addAliases func(any)
	addAliases = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for _, pair := range [][2]string{{"systemInstruction", "system_instruction"}, {"functionCall", "function_call"}, {"functionResponse", "function_response"}, {"functionDeclarations", "function_declarations"}} {
				if child, ok := value[pair[0]]; ok {
					value[pair[1]] = child
				}
			}
			for _, child := range value {
				addAliases(child)
			}
		case []any:
			for _, child := range value {
				addAliases(child)
			}
		}
	}
	addAliases(duplicated)
	body, err := json.Marshal(duplicated)
	require.NoError(t, err)
	require.Equal(t, want, estimateGeminiInputTokensLocally(body), "camel and snake aliases describe one field")
}

func TestGeminiInputUsageFallbackUsesReplayBodyAndAppliesOnlyAfterGeneration(t *testing.T) {
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"Explain the weather forecast and suggest suitable clothing."}]}]}`)
	req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1/models/test:streamGenerateContent", bytes.NewReader(body))
	require.NoError(t, err)
	_, err = io.Copy(io.Discard, req.Body) // The actual request body has already been consumed by HTTP.
	require.NoError(t, err)
	result := &ForwardResult{}
	prepareGeminiInputTokenEstimateFromRequest(result, req)
	require.Positive(t, result.EstimatedInputTokens)
	require.Zero(t, result.Usage.InputTokens, "preparation must not itself make a request billable")
	applyMissingInputTokenEstimate(context.Background(), result, nil)
	require.Zero(t, result.Usage.InputTokens, "metadata-only or outputless cancellation remains uncharged")
	delivered := 7
	result.DeliveredOutputTokens = &delivered
	result.Outcome = &ResponseOutcome{HasText: true}
	applyMissingInputTokenEstimate(context.Background(), result, nil)
	require.Equal(t, result.EstimatedInputTokens, result.Usage.InputTokens)
	require.True(t, result.InputTokensEstimated)
	require.Zero(t, result.Usage.OutputTokens, "provider measurement must stay unknown")
}
