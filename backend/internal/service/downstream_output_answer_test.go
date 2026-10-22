//go:build unit

package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDownstreamCollectorTerminalRequiresCompleteSuccessfulFrame(t *testing.T) {
	for _, tc := range []struct{ name, frame string }{
		{"chat finish", "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"},
		{"messages", "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"},
		{"native", "data: {\"candidates\":[{\"finishReason\":\"STOP\"}]}\n\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewDownstreamOutputTokenCollector("gemini-2.5-flash")
			collector.ObserveWritten([]byte(strings.TrimSuffix(tc.frame, "\n")))
			require.False(t, collector.HasDeliveredTerminal(), "the SSE frame has not yet reached its dispatch boundary")
			collector.ObserveWritten([]byte("\n"))
			require.True(t, collector.HasDeliveredTerminal())
			collector.Freeze()
			require.True(t, collector.HasDeliveredTerminal(), "a disconnect after delivery does not undo the terminal")

			failed := NewDownstreamOutputTokenCollector("gemini-2.5-flash")
			failed.ObserveWritten([]byte(strings.TrimSuffix(tc.frame, "\n")))
			failed.Freeze()
			failed.ObserveWritten([]byte("\n"))
			require.False(t, failed.HasDeliveredTerminal(), "a rejected frame tail is not a delivered terminal")
		})
	}
}

func TestDownstreamCollectorErrorDoneIsNotSuccessfulTerminal(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gemini-2.5-flash")
	collector.ObserveWritten([]byte("data: {\"error\":{\"type\":\"upstream_error\",\"code\":\"incomplete_stream\"}}\n\n"))
	collector.ObserveWritten([]byte("data: [DONE]\n\n"))
	require.False(t, collector.HasDeliveredTerminal(), "a failed stream's DONE is not successful completion")
}

func TestDownstreamCollectorDeliveredAnswerDistinguishesReasoning(t *testing.T) {
	for _, tc := range []struct {
		name, reasoning, answer string
	}{
		{"chat", `data: {"choices":[{"delta":{"reasoning_content":"thinking"}}]}`, `data: {"choices":[{"delta":{"content":"answer"}}]}`},
		{"messages", `data: {"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"thinking"}}`, `data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"answer"}}`},
		{"native", `data: {"candidates":[{"content":{"parts":[{"text":"thinking","thought":true}]}}]}`, `data: {"candidates":[{"content":{"parts":[{"text":"answer"}]}}]}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewDownstreamOutputTokenCollector("gemini-2.5-flash")
			collector.ObserveWritten([]byte(tc.reasoning + "\n\n"))
			require.False(t, collector.HasDeliveredText())
			require.Positive(t, collector.TokenCount(), "reasoning remains in the existing token count")
			collector.ObserveWritten([]byte(tc.answer + "\n\n"))
			require.True(t, collector.HasDeliveredText())
			require.Equal(t, "thinkinganswer", collector.VisibleText(), "the existing token input is unchanged")

			frozen := NewDownstreamOutputTokenCollector("gemini-2.5-flash")
			frozen.ObserveWritten([]byte(tc.reasoning + "\n\n"))
			frozen.Freeze()
			frozen.ObserveWritten([]byte(tc.answer + "\n\n"))
			require.False(t, frozen.HasDeliveredText(), "writes after cancellation cannot authorize a charge")
		})
	}
}

func TestDownstreamCollectorBufferedDeliveredAnswer(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		answer     bool
	}{
		{"chat reasoning", `{"choices":[{"message":{"reasoning_content":"thinking"}}]}`, false},
		{"chat answer", `{"choices":[{"message":{"content":"answer"}}]}`, true},
		{"messages reasoning", `{"content":[{"type":"thinking","thinking":"thinking"}]}`, false},
		{"messages answer", `{"content":[{"type":"text","text":"answer"}]}`, true},
		{"native reasoning", `{"candidates":[{"content":{"parts":[{"text":"thinking","thought":true}]}}]}`, false},
		{"native answer", `{"candidates":[{"content":{"parts":[{"text":"answer"}]}}]}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewDownstreamOutputTokenCollector("gemini-2.5-flash")
			collector.ObserveWritten([]byte(tc.body))
			require.Equal(t, tc.answer, collector.HasDeliveredText())
			require.Positive(t, collector.TokenCount())
		})
	}
}
