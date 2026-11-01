package apicompat

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestOutputLimitPresenceSurvivesProtocolBridges(t *testing.T) {
	for _, value := range []int{0, -1, 8, 128000} {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			var chat ChatCompletionsRequest
			require.NoError(t, json.Unmarshal([]byte(fmt.Sprintf(`{"model":"claude-opus-4-6","max_completion_tokens":%d,"messages":[{"role":"user","content":"hello"}]}`, value)), &chat))
			responses, err := ChatCompletionsToResponsesWithMinOutputTokens(&chat, 0)
			require.NoError(t, err)
			require.NotNil(t, responses.MaxOutputTokens)
			require.Equal(t, value, *responses.MaxOutputTokens)
			anthropic, err := ResponsesToAnthropicRequestWithDefaultMaxTokens(responses, 128000)
			require.NoError(t, err)
			wire, err := json.Marshal(anthropic)
			require.NoError(t, err)
			require.True(t, gjson.GetBytes(wire, "max_tokens").Exists())
			require.Equal(t, int64(value), gjson.GetBytes(wire, "max_tokens").Int())
			var roundTrip AnthropicRequest
			require.NoError(t, json.Unmarshal(wire, &roundTrip))
			back, err := AnthropicToResponsesWithMinOutputTokens(&roundTrip, 0)
			require.NoError(t, err)
			require.NotNil(t, back.MaxOutputTokens)
			require.Equal(t, value, *back.MaxOutputTokens)
			backChat, err := AnthropicToChatCompletionsRequestWithMinOutputTokens(&roundTrip, 0)
			require.NoError(t, err)
			require.NotNil(t, backChat.MaxCompletionTokens)
			require.Equal(t, value, *backChat.MaxCompletionTokens)
		})
	}
}

func TestResponsesReasoningFitsClientOutputAllowance(t *testing.T) {
	for _, limit := range []int{0, 8, 1024, 1025, 4096, 128000} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			req := &ResponsesRequest{Model: "claude-sonnet-4-6", Input: json.RawMessage(`"hello"`), MaxOutputTokens: &limit, Reasoning: &ResponsesReasoning{Effort: "high"}}
			got, err := ResponsesToAnthropicRequest(req)
			require.NoError(t, err)
			require.Equal(t, limit, got.MaxTokens)
			if limit <= 1024 {
				require.Nil(t, got.Thinking, "skip incompatible optional thinking instead of raising the output allowance")
			} else {
				require.NotNil(t, got.Thinking)
				require.Less(t, got.Thinking.BudgetTokens, limit)
			}
		})
	}
}
