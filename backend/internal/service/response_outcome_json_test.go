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
