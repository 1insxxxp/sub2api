package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponseOutcomeCollectorObserveJSONPayload(t *testing.T) {
	tests := []struct {
		name     string
		protocol ResponseOutcomeProtocol
		payload  string
		assert   func(*testing.T, ResponseOutcome)
	}{
		{
			name:     "anthropic text",
			protocol: ResponseOutcomeProtocolAnthropic,
			payload:  `{"content":[{"type":"text","text":"hello"}],"stop_reason":"end_turn"}`,
			assert:   func(t *testing.T, got ResponseOutcome) { require.True(t, got.HasText) },
		},
		{
			name:     "anthropic tool",
			protocol: ResponseOutcomeProtocolAnthropic,
			payload:  `{"content":[{"type":"tool_use","id":"toolu_1","input":{"secret":"value"}}]}`,
			assert:   func(t *testing.T, got ResponseOutcome) { require.True(t, got.HasToolCall) },
		},
		{
			name:     "openai function call",
			protocol: ResponseOutcomeProtocolOpenAI,
			payload:  `{"choices":[{"message":{"content":null,"tool_calls":[{"function":{"arguments":"private"}}]}}]}`,
			assert:   func(t *testing.T, got ResponseOutcome) { require.True(t, got.HasToolCall) },
		},
		{
			name:     "gemini media",
			protocol: ResponseOutcomeProtocolGemini,
			payload:  `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"private"}}]}}]}`,
			assert:   func(t *testing.T, got ResponseOutcome) { require.True(t, got.HasMedia) },
		},
		{
			name:     "empty payload",
			protocol: ResponseOutcomeProtocolOpenAI,
			payload:  `{"choices":[{"message":{"content":"   "},"finish_reason":"stop"}]}`,
			assert:   func(t *testing.T, got ResponseOutcome) { require.False(t, got.HasEffectiveOutput()) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewResponseOutcomeCollector(200, 200)
			require.NoError(t, collector.ObserveJSONPayload(tt.protocol, []byte(tt.payload)))
			tt.assert(t, collector.Snapshot())
		})
	}
}

func TestResponseOutcomeCollectorObserveJSONPayloadRejectsInvalidJSON(t *testing.T) {
	collector := NewResponseOutcomeCollector(200, 200)
	require.Error(t, collector.ObserveJSONPayload(ResponseOutcomeProtocolOpenAI, []byte(`{"choices":`)))
	require.False(t, collector.Snapshot().HasEffectiveOutput())
}

func TestResponseOutcomeCollectorAnthropicSSERequiresTerminalFrame(t *testing.T) {
	collector := NewResponseOutcomeCollector(200, 200)
	collector.ObserveAnthropicSSEData(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)

	beforeStop := collector.Snapshot()
	require.Equal(t, "end_turn", beforeStop.FinishReason)
	require.False(t, beforeStop.StreamCompleted)

	collector.ObserveAnthropicSSEData(`{"type":"message_stop"}`)
	require.True(t, collector.Snapshot().StreamCompleted)
}

func TestResponseOutcomeCollectorTerminalFramesPreserveFinishReason(t *testing.T) {
	tests := []struct {
		name    string
		observe func(*ResponseOutcomeCollector, string)
		data    string
		stop    string
		want    string
	}{
		{
			name:    "anthropic token limit",
			observe: (*ResponseOutcomeCollector).ObserveAnthropicSSEData,
			data:    `{"type":"message_delta","delta":{"stop_reason":"max_tokens"}}`,
			stop:    `{"type":"message_stop"}`,
			want:    "max_tokens",
		},
		{
			name:    "anthropic natural completion",
			observe: (*ResponseOutcomeCollector).ObserveAnthropicSSEData,
			data:    `{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`,
			stop:    `{"type":"message_stop"}`,
			want:    "end_turn",
		},
		{
			name:    "openai token limit",
			observe: (*ResponseOutcomeCollector).ObserveOpenAISSEData,
			data:    `{"choices":[{"delta":{},"finish_reason":"length"}]}`,
			stop:    "[DONE]",
			want:    "length",
		},
		{
			name:    "openai content filter",
			observe: (*ResponseOutcomeCollector).ObserveOpenAISSEData,
			data:    `{"choices":[{"delta":{},"finish_reason":"content_filter"}]}`,
			stop:    "[DONE]",
			want:    "content_filter",
		},
		{
			name:    "gemini token limit",
			observe: (*ResponseOutcomeCollector).ObserveGeminiSSEData,
			data:    `{"candidates":[{"finishReason":"MAX_TOKENS"}]}`,
			stop:    "[DONE]",
			want:    "max_tokens",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewResponseOutcomeCollector(200, 200)
			tt.observe(collector, tt.data)
			require.Equal(t, tt.want, collector.Snapshot().FinishReason)
			tt.observe(collector, tt.stop)
			outcome := collector.Snapshot()
			require.Equal(t, tt.want, outcome.FinishReason)
			require.True(t, outcome.StreamCompleted)
		})
	}
}

func TestResponseOutcomeCollectorTerminalFramesWithoutFinishReasonUseFallback(t *testing.T) {
	for _, tt := range []struct {
		name    string
		observe func(*ResponseOutcomeCollector, string)
		stop    string
		want    string
	}{
		{"anthropic", (*ResponseOutcomeCollector).ObserveAnthropicSSEData, `{"type":"message_stop"}`, "message_stop"},
		{"openai", (*ResponseOutcomeCollector).ObserveOpenAISSEData, "[DONE]", "done"},
		{"gemini", (*ResponseOutcomeCollector).ObserveGeminiSSEData, "[DONE]", "done"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			collector := NewResponseOutcomeCollector(200, 200)
			tt.observe(collector, tt.stop)
			outcome := collector.Snapshot()
			require.Equal(t, tt.want, outcome.FinishReason)
			require.True(t, outcome.StreamCompleted)
		})
	}
}

func TestResponseOutcomeCollectorLaterResponseCanUpdateFinishReason(t *testing.T) {
	collector := NewResponseOutcomeCollector(200, 200)
	collector.ObserveAnthropicSSEData(`{"type":"message_delta","delta":{"stop_reason":"max_tokens"}}`)
	collector.ObserveAnthropicSSEData(`{"type":"message_stop"}`)
	collector.ObserveAnthropicSSEData(`{"type":"message_delta","delta":{"stop_reason":"end_turn"}}`)
	collector.ObserveAnthropicSSEData(`{"type":"message_stop"}`)
	require.Equal(t, "end_turn", collector.Snapshot().FinishReason)

	collector.MarkCompleted("length")
	require.Equal(t, "length", collector.Snapshot().FinishReason)
}
