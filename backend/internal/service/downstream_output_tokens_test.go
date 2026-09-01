//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tiktoken-go/tokenizer"
)

func TestDownstreamOutputTokenCollectorCountsOnlyDeliveredVisibleText(t *testing.T) {
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	require.NoError(t, err)

	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"first visible \"}}\n\n"))
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"second visible \"}}]}\n\n"))
	collector.ObserveWritten([]byte("event: response.output_text.delta\n" +
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"third visible\"}\n\n"))
	collector.ObserveWritten([]byte("event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"hidden reasoning\"}}\n\n"))
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"{\\\"hidden\\\":true}\"}}]}}]}\n\n"))

	wantTokens, _, err := codec.Encode("first visible second visible third visible")
	require.NoError(t, err)
	want := len(wantTokens)
	require.Equal(t, want, collector.TokenCount())
}

func TestDownstreamOutputTokenCollectorBuffersSplitSSEFrames(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial"))
	collector.ObserveWritten([]byte(" frame\"}}]}\n\n"))

	require.Positive(t, collector.TokenCount())
}

func TestDownstreamOutputTokenCollectorCountsOpenAIEventLineDelta(t *testing.T) {
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	require.NoError(t, err)

	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("event: response.reasoning_summary_text.delta\n" +
		"data: {\"delta\":\"hidden reasoning\"}\n\n"))
	collector.ObserveWritten([]byte("event: response.output_text.delta\n" +
		"data: {\"delta\":\"visible response\"}\n\n"))

	want, _, err := codec.Encode("visible response")
	require.NoError(t, err)
	require.Equal(t, len(want), collector.TokenCount())
}

func TestAttachDeliveredOutputTokenCollectorChargesOnlySuccessfulWrites(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-opus-4-6")
	defer restore()
	_, err := fmt.Fprint(c.Writer, "event: content_block_delta\n"+
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\n\n")
	require.NoError(t, err)

	result := &ForwardResult{Stream: true, Usage: ClaudeUsage{OutputTokens: 321}}
	ApplyDeliveredOutputTokenBilling(result, collector)
	require.Equal(t, 321, result.UpstreamOutputTokens)
	require.Equal(t, collector.TokenCount(), result.Usage.OutputTokens)
	require.Equal(t, 321, recordedUpstreamOutputTokens(result))

	// Usage submission can be reached from an error/cleanup path after a
	// successful stream. Preserve the provider-reported amount on re-entry.
	ApplyDeliveredOutputTokenBilling(result, collector)
	require.Equal(t, 321, result.UpstreamOutputTokens)
	require.Equal(t, collector.TokenCount(), result.Usage.OutputTokens)
}

func TestApplyDeliveredOpenAIOutputTokenBilling(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"visible response\"}}]}\n\n"))

	result := &OpenAIForwardResult{Stream: true, Usage: OpenAIUsage{OutputTokens: 456}}
	ApplyDeliveredOpenAIOutputTokenBilling(result, collector)

	require.Equal(t, 456, result.UpstreamOutputTokens)
	require.Equal(t, collector.TokenCount(), result.Usage.OutputTokens)
	require.Equal(t, 456, recordedOpenAIUpstreamOutputTokens(result))

	ApplyDeliveredOpenAIOutputTokenBilling(result, collector)
	require.Equal(t, 456, result.UpstreamOutputTokens)
}

func TestApplyDeliveredOutputTokenBillingLeavesImageResultsUntouched(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"visible response\"}}]}\n\n"))

	result := &ForwardResult{
		Stream:     true,
		ImageCount: 1,
		Usage:      ClaudeUsage{OutputTokens: 456, ImageOutputTokens: 7},
	}
	ApplyDeliveredOutputTokenBilling(result, collector)

	require.Equal(t, 456, result.Usage.OutputTokens)
	require.Zero(t, result.UpstreamOutputTokens)
	require.False(t, result.DownstreamOutputTokenBilling)
}

func TestDeliveredOutputTokenBillingPersistsVisibleTokensToUsageLog(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("event: content_block_delta\n" +
		"data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"The user received only this sentence.\"}}\n\n"))

	result := &ForwardResult{
		RequestID: "visible-output-token-test",
		Model:     "claude-opus-4-6",
		Stream:    true,
		Usage: ClaudeUsage{
			InputTokens:  100,
			OutputTokens: 17841,
		},
	}
	ApplyDeliveredOutputTokenBilling(result, collector)

	svc := &GatewayService{}
	groupID := int64(1)
	usageLog := svc.buildRecordUsageLog(
		context.Background(),
		&recordUsageCoreInput{},
		result,
		&APIKey{ID: 1, GroupID: &groupID, Group: &Group{ID: groupID}},
		&User{ID: 2},
		&Account{ID: 3},
		nil,
		result.Model,
		1,
		1,
		1,
		BillingTypeBalance,
		false,
		&CostBreakdown{},
		&recordUsageOpts{},
	)

	require.Equal(t, collector.TokenCount(), usageLog.OutputTokens)
	require.Less(t, usageLog.OutputTokens, 17841)
	require.Equal(t, 17841, usageLog.UpstreamOutputTokens)
}
