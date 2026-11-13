//go:build unit

package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestInflightInputEstimateIgnoresTransportMetadata(t *testing.T) {
	svc := newInflightEstimateGateway(t, nil)
	key := &APIKey{User: &User{ID: 1}, Group: &Group{RateMultiplier: 1}}
	base := `{"messages":[{"role":"user","content":"你好，请继续这个故事。"}]}`
	var payload map[string]any
	require.NoError(t, json.Unmarshal([]byte(base), &payload))
	payload["metadata"] = strings.Repeat("irrelevant", 300000)
	inflated, err := json.MarshalIndent(payload, "", "  ")
	require.NoError(t, err)
	first, ok := svc.EstimateInflightReservation(context.Background(), key, InflightEstimateRequest{Model: "claude-opus-4-6", Body: []byte(base), BodyBytes: len(base), MaxTokens: 512})
	require.True(t, ok)
	second, ok := svc.EstimateInflightReservation(context.Background(), key, InflightEstimateRequest{Model: "claude-opus-4-6", Body: inflated, BodyBytes: len(inflated), MaxTokens: 512})
	require.True(t, ok)
	require.Equal(t, first, second)
}

func TestInflightInputEstimateEquivalentJSONAndTools(t *testing.T) {
	a, ok := estimateInflightInputTokens("claude-opus-4-6", []byte(`{"messages":[{"role":"user","content":"你好"}]}`))
	require.True(t, ok)
	b, ok := estimateInflightInputTokens("claude-opus-4-6", []byte(`{ "messages" : [{"role":"user", "content":"\u4f60\u597d"}], "max_tokens":999999 }`))
	require.True(t, ok)
	require.Equal(t, a, b)
	tools, ok := estimateInflightInputTokens("claude-opus-4-6", []byte(`{"messages":[{"role":"user","content":"你好"}],"tools":[{"name":"weather","description":"Look up weather in a city","input_schema":{"type":"object","properties":{"city":{"type":"string"}}}}]}`))
	require.True(t, ok)
	require.Greater(t, tools, a)
}

func TestInflightInputEstimateFixedPriceUsesExactAmount(t *testing.T) {
	id := int64(20)
	price := 0.5
	group := &Group{ID: id, Platform: PlatformAnthropic, RateMultiplier: 2, ModelPricing: []ChannelModelPricing{{Models: []string{"fixed"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price}}}
	svc := newInflightEstimateGateway(t, nil)
	key := &APIKey{User: &User{ID: 1}, GroupID: &id, Group: group}
	for _, body := range [][]byte{nil, []byte(`not json`), []byte(strings.Repeat("x", 3000000))} {
		amount, ok := svc.EstimateInflightReservation(context.Background(), key, InflightEstimateRequest{Model: "fixed", Body: body, BodyBytes: len(body), MaxTokens: 1000000})
		require.True(t, ok)
		require.Equal(t, 1.0, amount)
	}
}

func TestInflightInputEstimateLongRoleplayAccuracy(t *testing.T) {
	text := strings.Repeat("她望着窗外的雨，轻声问道：你还记得那个约定吗？\n", 4000)
	body, err := json.Marshal(map[string]any{"messages": []map[string]string{{"role": "user", "content": text}}, "metadata": strings.Repeat("irrelevant", 300000)})
	require.NoError(t, err)
	estimated, ok := estimateInflightInputTokens("claude-opus-4-6", body)
	require.True(t, ok)
	codec, err := openAIInputTokensCodecForModel("claude-opus-4-6")
	require.NoError(t, err)
	reference, err := codec.Count(text)
	require.NoError(t, err)
	require.InDelta(t, reference, estimated, float64(reference)*0.02)
	t.Logf("body_bytes=%d old_estimate=%d tokenizer_reference=%d new_estimate=%d", len(body), len(body)/4, reference, estimated)
}

func TestInflightInputEstimateGeminiTransportParameters(t *testing.T) {
	plain := []byte(`{"contents":[{"role":"user","parts":[{"text":"你好，请继续故事"}]}],"systemInstruction":{"parts":[{"text":"保持角色设定"}]}}`)
	var request map[string]any
	require.NoError(t, json.Unmarshal(plain, &request))
	request["generationConfig"] = map[string]any{"maxOutputTokens": 1000000, "client_metadata": strings.Repeat("x", 2000000)}
	inflated, err := json.Marshal(request)
	require.NoError(t, err)
	first, ok := estimateInflightInputTokens("gemini-3.5-flash", plain)
	require.True(t, ok)
	second, ok := estimateInflightInputTokens("gemini-3.5-flash", inflated)
	require.True(t, ok)
	require.Equal(t, first, second)
}

func TestInflightInputEstimateStructuredToolResultsIncludeKeys(t *testing.T) {
	key := strings.Repeat("description of the weather ", 1000)
	response := map[string]any{key: true}
	body, err := json.Marshal(map[string]any{"contents": []any{map[string]any{"parts": []any{map[string]any{"functionResponse": map[string]any{"name": "weather", "response": response}}}}}})
	require.NoError(t, err)
	count, ok := estimateInflightInputTokens("gemini-3.5-flash", body)
	require.True(t, ok)
	codec, err := openAIInputTokensCodecForModel("gemini-3.5-flash")
	require.NoError(t, err)
	minimum, err := codec.Count(key)
	require.NoError(t, err)
	require.GreaterOrEqual(t, count, minimum)
}

func TestInflightInputEstimateOutputSchemasAreInput(t *testing.T) {
	schema := map[string]any{"type": "object", "description": strings.Repeat("An important schema description. ", 1000)}
	for _, extra := range []map[string]any{
		{"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "answer", "schema": schema}}},
		{"text": map[string]any{"format": map[string]any{"type": "json_schema", "schema": schema}}},
		{"generationConfig": map[string]any{"responseSchema": schema}},
		{"generation_config": map[string]any{"response_json_schema": schema}},
	} {
		extra["input"] = "hello"
		body, err := json.Marshal(extra)
		require.NoError(t, err)
		count, ok := estimateInflightInputTokens("gpt-4o", body)
		require.True(t, ok)
		require.Greater(t, count, 1000)
	}
}

func TestInflightInputEstimateIgnoresMessageMetadataAndSignatures(t *testing.T) {
	a, ok := estimateInflightInputTokens("gemini-3.5-flash", []byte(`{"contents":[{"parts":[{"text":"hello"}]}]}`))
	require.True(t, ok)
	body, err := json.Marshal(map[string]any{"contents": []any{map[string]any{"metadata": strings.Repeat("x ", 500000), "parts": []any{map[string]any{"text": "hello", "thoughtSignature": strings.Repeat("x ", 500000)}}}}})
	require.NoError(t, err)
	b, ok := estimateInflightInputTokens("gemini-3.5-flash", body)
	require.True(t, ok)
	require.Equal(t, a, b)
}

func TestInflightInputEstimateEmbeddingTokenIDs(t *testing.T) {
	for _, body := range []string{`{"input":[100000,100001,100002]}`, `{"input":[[100000,100001],[100002]]}`} {
		count, ok := estimateInflightInputTokens("text-embedding-3-large", []byte(body))
		require.True(t, ok)
		require.Equal(t, 3, count)
	}
}

func TestInflightInputEstimateToolMetadataIsExcludedAndSchemaKeysRetained(t *testing.T) {
	function := map[string]any{"name": "weather", "description": "Look up weather", "parameters": map[string]any{"type": "object"}}
	tool := map[string]any{"type": "function", "function": function}
	request := map[string]any{"input": "hello", "tools": []any{tool}}
	plain, err := json.Marshal(request)
	require.NoError(t, err)
	before, ok := estimateInflightInputTokens("gpt-4o", plain)
	require.True(t, ok)
	tool["metadata"] = strings.Repeat("client only ", 100000)
	function["client_metadata"] = strings.Repeat("client only ", 100000)
	inflated, err := json.Marshal(request)
	require.NoError(t, err)
	after, ok := estimateInflightInputTokens("gpt-4o", inflated)
	require.True(t, ok)
	require.Equal(t, before, after)
	function["parameters"] = map[string]any{"type": "object", "properties": map[string]any{"metadata": map[string]any{"type": "string", "description": strings.Repeat("model schema ", 1000)}}}
	schema, err := json.Marshal(request)
	require.NoError(t, err)
	count, ok := estimateInflightInputTokens("gpt-4o", schema)
	require.True(t, ok)
	require.Greater(t, count, before+1000)
}
