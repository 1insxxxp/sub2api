package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveModelSystemPromptMatchesExactMappedModel(t *testing.T) {
	account := &Account{ModelSystemPrompts: map[string]string{"gpt-5.4": "Stay in character."}}

	prompt, ok := account.ResolveModelSystemPrompt("gpt-5.4")
	require.True(t, ok)
	require.Equal(t, "Stay in character.", prompt)

	_, ok = account.ResolveModelSystemPrompt("public-alias")
	require.False(t, ok)
}

func TestPrependModelSystemPrompt(t *testing.T) {
	tests := []struct {
		name     string
		protocol ModelSystemPromptProtocol
		body     string
		assert   func(t *testing.T, got map[string]any)
	}{
		{
			name:     "responses instructions",
			protocol: ModelSystemPromptOpenAIResponses,
			body:     `{"model":"gpt-5.4","instructions":"Client rules","input":"hello"}`,
			assert: func(t *testing.T, got map[string]any) {
				require.Equal(t, "Injected rules\n\nClient rules", got["instructions"])
				require.Equal(t, "hello", got["input"])
			},
		},
		{
			name:     "chat messages",
			protocol: ModelSystemPromptOpenAIChat,
			body:     `{"model":"gpt-5.4","messages":[{"role":"system","content":"Client rules"},{"role":"user","content":"hello"}]}`,
			assert: func(t *testing.T, got map[string]any) {
				messages := got["messages"].([]any)
				require.Equal(t, "system", messages[0].(map[string]any)["role"])
				require.Equal(t, "Injected rules", messages[0].(map[string]any)["content"])
				require.Equal(t, "Client rules", messages[1].(map[string]any)["content"])
			},
		},
		{
			name:     "claude string system",
			protocol: ModelSystemPromptClaude,
			body:     `{"model":"claude-opus-5","system":"Client rules","messages":[]}`,
			assert: func(t *testing.T, got map[string]any) {
				require.Equal(t, "Injected rules\n\nClient rules", got["system"])
			},
		},
		{
			name:     "claude block system",
			protocol: ModelSystemPromptClaude,
			body:     `{"model":"claude-opus-5","system":[{"type":"text","text":"Client rules","cache_control":{"type":"ephemeral"}}],"messages":[]}`,
			assert: func(t *testing.T, got map[string]any) {
				blocks := got["system"].([]any)
				require.Equal(t, "Injected rules", blocks[0].(map[string]any)["text"])
				require.Equal(t, "Client rules", blocks[1].(map[string]any)["text"])
				require.NotNil(t, blocks[1].(map[string]any)["cache_control"])
			},
		},
		{
			name:     "gemini parts",
			protocol: ModelSystemPromptGemini,
			body:     `{"contents":[],"systemInstruction":{"parts":[{"text":"Client rules"}]}}`,
			assert: func(t *testing.T, got map[string]any) {
				system := got["systemInstruction"].(map[string]any)
				parts := system["parts"].([]any)
				require.Equal(t, "Injected rules", parts[0].(map[string]any)["text"])
				require.Equal(t, "Client rules", parts[1].(map[string]any)["text"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := PrependModelSystemPrompt([]byte(tt.body), tt.protocol, "Injected rules")
			require.NoError(t, err)
			var got map[string]any
			require.NoError(t, json.Unmarshal(body, &got))
			tt.assert(t, got)
		})
	}
}

func TestPrependModelSystemPromptCreatesMissingSystemFields(t *testing.T) {
	tests := []struct {
		protocol ModelSystemPromptProtocol
		body     string
		path     []string
	}{
		{ModelSystemPromptOpenAIResponses, `{"input":"hello"}`, []string{"instructions"}},
		{ModelSystemPromptClaude, `{"messages":[]}`, []string{"system"}},
		{ModelSystemPromptGemini, `{"contents":[]}`, []string{"systemInstruction", "parts"}},
	}

	for _, tt := range tests {
		body, err := PrependModelSystemPrompt([]byte(tt.body), tt.protocol, "Injected rules")
		require.NoError(t, err)
		require.Contains(t, string(body), "Injected rules")
	}
}

func TestPrependModelSystemPromptRejectsMalformedBody(t *testing.T) {
	_, err := PrependModelSystemPrompt([]byte(`{"messages":{}}`), ModelSystemPromptOpenAIChat, "prompt")
	require.Error(t, err)
}
