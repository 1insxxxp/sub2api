package antigravity

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestTransformClaudeOutputLimitPreservesClientIntent(t *testing.T) {
	for _, model := range []string{"claude-sonnet-4-6", "gemini-3.1-pro-high"} {
		for _, limit := range []int{0, 8, 8192, 64001, 131072} {
			t.Run(model+"/limit="+strconv.Itoa(limit), func(t *testing.T) {
				req := &ClaudeRequest{Model: model, MaxTokens: limit, Messages: []ClaudeMessage{{Role: "user", Content: json.RawMessage(`"hello"`)}}}
				body, err := TransformClaudeToGeminiWithOptions(req, "test-project", model, TransformOptions{})
				require.NoError(t, err)
				wireLimit := gjson.GetBytes(body, "request.generationConfig.maxOutputTokens")
				if limit > 0 {
					require.Equal(t, int64(limit), wireLimit.Int())
				} else {
					require.False(t, wireLimit.Exists())
				}
				require.Equal(t, limit, req.MaxTokens)
			})
		}
	}
}

func TestGenerationThinkingBudgetFitsExplicitOutputLimit(t *testing.T) {
	for _, tc := range []struct {
		name       string
		model      string
		limit      int
		thinking   ThinkingConfig
		wantBudget int
		omitThink  bool
	}{
		{name: "omitted stays omitted", model: "claude-sonnet-4-6", thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, wantBudget: 8192},
		{name: "manual budget fits cap", model: "claude-sonnet-4-6", limit: 4096, thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, wantBudget: 4095},
		{name: "adaptive budget fits cap", model: "claude-opus-4-6-thinking", limit: 4096, thinking: ThinkingConfig{Type: "adaptive"}, wantBudget: 4095},
		{name: "minimum supported budget", model: "claude-sonnet-4-6", limit: 1025, thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, wantBudget: 1024},
		{name: "too small for Claude thinking", model: "claude-sonnet-4-6", limit: 1024, thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, omitThink: true},
		{name: "large cap stays large", model: "claude-sonnet-4-6", limit: 131072, thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, wantBudget: 8192},
		{name: "dynamic budget preserved", model: "claude-sonnet-4-6", limit: 4096, thinking: ThinkingConfig{Type: "enabled"}, wantBudget: -1},
		{name: "Gemini Pro budget fits cap", model: "gemini-2.5-pro", limit: 512, thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, wantBudget: 511},
		{name: "too small for Gemini Pro thinking", model: "gemini-2.5-pro", limit: 128, thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, omitThink: true},
		{name: "too small for Gemini Flash Lite thinking", model: "gemini-2.5-flash-lite", limit: 512, thinking: ThinkingConfig{Type: "enabled", BudgetTokens: 8192}, omitThink: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &ClaudeRequest{Model: tc.model, MaxTokens: tc.limit, Thinking: &tc.thinking}
			got := buildGenerationConfig(req)
			require.Equal(t, tc.limit, got.MaxOutputTokens, "thinking cannot change the client's total output cap")
			if tc.omitThink {
				require.Nil(t, got.ThinkingConfig, "skip an incompatible thinking config instead of enlarging output")
			} else {
				require.NotNil(t, got.ThinkingConfig)
				require.Equal(t, tc.wantBudget, got.ThinkingConfig.ThinkingBudget)
			}
		})
	}
}

func TestTransformClaudePreservesExplicitZeroAndNegativeOutputLimits(t *testing.T) {
	for _, limit := range []string{"0", "-1"} {
		t.Run(limit, func(t *testing.T) {
			var req ClaudeRequest
			require.NoError(t, json.Unmarshal([]byte(`{"model":"claude-sonnet-4-6","max_tokens":`+limit+`,"messages":[{"role":"user","content":"hello"}]}`), &req))
			roundTrip, err := json.Marshal(req)
			require.NoError(t, err)
			require.Equal(t, limit, gjson.GetBytes(roundTrip, "max_tokens").Raw)
			body, err := TransformClaudeToGeminiWithOptions(&req, "test-project", req.Model, TransformOptions{})
			require.NoError(t, err)
			got := gjson.GetBytes(body, "request.generationConfig.maxOutputTokens")
			require.True(t, got.Exists(), "invalid or cache-only explicit limits must not turn into uncapped generation")
			require.Equal(t, limit, got.Raw)
		})
	}
}
