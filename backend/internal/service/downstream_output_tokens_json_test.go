//go:build unit

package service

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDownstreamCollectorCountsBufferedJSONSemanticOutput(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"anthropic", `{"id":"ignored","type":"message","content":[{"type":"thinking","thinking":"thought","signature":"opaque"},{"type":"text","text":"answer"},{"type":"tool_use","id":"ignored","name":"lookup","input":{"city":"Paris"}},{"type":"redacted_thinking","data":"opaque"}],"usage":{"output_tokens":0}}`, `thoughtanswer{"city":"Paris"}`},
		{"chat", `{"object":"chat.completion","choices":[{"message":{"role":"assistant","content":"answer","reasoning_content":"thought","tool_calls":[{"id":"ignored","function":{"name":"lookup","arguments":"{\"city\":\"Paris\"}"}}]}}],"usage":{"completion_tokens":0}}`, `answerthought{"city":"Paris"}`},
		{"chat content parts", `{"choices":[{"message":{"content":[{"type":"text","text":"answer"},{"type":"image_url","image_url":{"url":"data:image/png;base64,opaque"}}]}}]}`, "answer"},
		{"responses", `{"object":"response","output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"thought"}],"encrypted_content":"opaque"},{"type":"message","content":[{"type":"output_text","text":"answer"}]},{"type":"function_call","name":"lookup","arguments":"{\"city\":\"Paris\"}"},{"type":"custom_tool_call","input":"script"}],"usage":{"output_tokens":0}}`, `thoughtanswer{"city":"Paris"}script`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewDownstreamOutputTokenCollector("claude-sonnet-4-5")
			mid := len(tc.body) / 2
			collector.ObserveWritten([]byte(tc.body[:mid]))
			require.Empty(t, collector.VisibleText(), "incomplete JSON must not be billed as a delivered response")
			collector.ObserveWritten([]byte(tc.body[mid:]))
			require.Equal(t, tc.want, collector.VisibleText())
			require.Positive(t, collector.TokenCount())
			require.Equal(t, collector.TokenCount(), collector.TokenCount(), "reading the snapshot must not count twice")
		})
	}
}

func TestDownstreamCollectorSkipsBufferedErrorsAndMetadata(t *testing.T) {
	for _, body := range []string{
		`{"error":{"message":"failure"},"choices":[{"message":{"content":"untrusted error response"}}]}`,
		`{"type":"error","message":"failure","content":[{"type":"text","text":"failure"}]}`,
		`{"id":"metadata","model":"claude-sonnet","usage":{"output_tokens":999},"content":[{"type":"redacted_thinking","data":"opaque"}]}`,
	} {
		collector := NewDownstreamOutputTokenCollector("claude-sonnet-4-5")
		collector.ObserveWritten([]byte(body))
		require.Empty(t, collector.VisibleText())
		require.Zero(t, collector.TokenCount())
	}
}

func TestAttachedDownstreamCollectorCountsBufferedJSONBeforeCancellation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Request = httptest.NewRequest("POST", "/v1/messages", nil).WithContext(ctx)
	collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-sonnet-4-5")
	defer restore()
	c.JSON(200, gin.H{"type": "message", "content": []gin.H{{"type": "text", "text": "visible answer"}}})
	result := &ForwardResult{Outcome: &ResponseOutcome{StreamCompleted: true}}
	ApplyDeliveredOutputTokens(result, collector)
	require.Positive(t, recordedOutputTokens(result))
	cancel()
	_, _ = c.Writer.WriteString(`{"content":[{"type":"text","text":"after cancellation"}]}`)
	require.Equal(t, "visible answer", collector.VisibleText())
}

func TestBufferedProviderOutputUsageWinsWithoutStopMetadata(t *testing.T) {
	delivered := 3
	result := &ForwardResult{
		Stream: false, Usage: ClaudeUsage{OutputTokens: 100},
		DeliveredOutputTokens: &delivered, Outcome: &ResponseOutcome{HasText: true},
	}
	require.Equal(t, 100, recordedOutputTokens(result), "a buffered JSON response does not need stream terminal metadata to trust its positive usage")
}
