//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDownstreamOutputTokenCollectorFreezeCancelsUpstreamOnce(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	var calls atomic.Int32
	collector.bindUpstreamCancel(func() {
		calls.Add(1)
	})

	collector.Freeze()
	collector.Freeze()

	require.Equal(t, int32(1), calls.Load())
}

func TestDownstreamOutputTokenCollectorLateCancelBindingAfterFreeze(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.Freeze()
	var calls atomic.Int32

	collector.bindUpstreamCancel(func() {
		calls.Add(1)
	})

	require.Equal(t, int32(1), calls.Load())
}

func TestDownstreamOutputTokenCollectorFreezesAfterClientCancellation(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"before\"}}\n\n"))

	collector.Freeze()
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"after\"}}\n\n"))

	if got := collector.VisibleText(); got != "before" {
		t.Fatalf("visible text after freeze = %q, want %q", got, "before")
	}
}

func TestAttachedDownstreamCollectorFreezesWhenRequestContextIsCanceled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	requestContext, cancel := context.WithCancel(context.Background())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil).WithContext(requestContext)

	collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-opus-4-6")
	defer restore()
	_, err := fmt.Fprint(c.Writer, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"before\"}}\n\n")
	require.NoError(t, err)

	cancel()
	_, err = fmt.Fprint(c.Writer, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"after\"}}\n\n")
	require.NoError(t, err)
	require.True(t, collector.Frozen())
	require.Equal(t, "before", collector.VisibleText())
}

func TestAttachedDownstreamCollectorFreezeCancelsForwardContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)

	collector, restore := AttachDownstreamOutputTokenCollector(c, "claude-opus-4-6")
	defer restore()
	forwardContext := c.Request.Context()

	collector.Freeze()

	select {
	case <-forwardContext.Done():
	case <-time.After(100 * time.Millisecond):
		t.Fatal("forward context was not canceled after downstream freeze")
	}
}

func TestDownstreamCollectorUsesSSEEventNameWhenPayloadOmitsType(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("event: response.output_text.delta\ndata: {\"delta\":\"visible\"}\n\n"))
	require.Equal(t, "visible", collector.VisibleText())
}

func TestDownstreamCollectorParsesCRLFSSEFramesAcrossWrites(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("event: content_block_delta\r\ndata: {\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\r"))
	collector.ObserveWritten([]byte("\n\r\n"))
	require.Equal(t, "visible", collector.VisibleText())
}

func TestDownstreamCollectorCountsOpenAIResponsesFunctionArguments(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("event: response.function_call_arguments.delta\ndata: {\"type\":\"response.function_call_arguments.delta\",\"delta\":\"{\\\"query\\\":\\\"status\\\"}\"}\n\n"))

	require.Equal(t, `{"query":"status"}`, collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsOpenAIResponsesVisibleReasoningAndCustomToolInput(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"type\":\"response.reasoning_summary_text.delta\",\"delta\":\"checking facts\"}\n\n"))
	collector.ObserveWritten([]byte("data: {\"type\":\"response.custom_tool_call_input.delta\",\"delta\":\"patch body\"}\n\n"))

	require.Equal(t, "checking factspatch body", collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsOpenAIChatToolArguments(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"function\":{\"arguments\":\"{\\\"path\\\":\\\"README.md\\\"}\"}}]}}]}\n\n"))

	require.Equal(t, `{"path":"README.md"}`, collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsOpenAIChatVisibleReasoning(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.6-terra")
	collector.ObserveWritten([]byte("data: {\"choices\":[{\"delta\":{\"reasoning_content\":\"checking facts\"}}]}\n\n"))

	require.Equal(t, "checking facts", collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsAnthropicToolInput(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"query\\\":\\\"status\\\"}\"}}\n\n"))

	require.Equal(t, `{"query":"status"}`, collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestDownstreamCollectorCountsVisibleReasoningButNotSignature(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"checking facts\"}}\n\n"))
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"signature_delta\",\"signature\":\"opaque-signature\"}}\n\n"))

	require.Equal(t, "checking facts", collector.VisibleText())
	require.Greater(t, collector.TokenCount(), 0)
}

func TestCompletedStreamRecordsProviderOutputTokens(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\n\n"))
	result := &ForwardResult{Usage: ClaudeUsage{OutputTokens: 321}}

	ApplyDeliveredOutputTokens(result, collector)

	require.Equal(t, 321, result.Usage.OutputTokens)
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Equal(t, collector.TokenCount(), *result.DeliveredOutputTokens)
	require.Equal(t, 321, recordedOutputTokens(result))
	require.Equal(t, 321, customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens))
}

func TestApplyDeliveredOutputTokensMarksFrozenCollectorAsClientDisconnect(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\n\n"))
	collector.Freeze()
	result := &ForwardResult{Usage: ClaudeUsage{OutputTokens: 321}}

	ApplyDeliveredOutputTokens(result, collector)

	require.True(t, result.ClientDisconnect)
	require.NotNil(t, result.DeliveredOutputTokens)
}

func TestApplyDeliveredOpenAIOutputTokensMarksFrozenCollectorAsClientDisconnect(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("gpt-5.1")
	collector.ObserveWritten([]byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"visible\"}\n\n"))
	collector.Freeze()
	result := &OpenAIForwardResult{Usage: OpenAIUsage{OutputTokens: 321}}

	ApplyDeliveredOpenAIOutputTokens(result, collector)

	require.True(t, result.ClientDisconnect)
	require.NotNil(t, result.DeliveredOutputTokens)
}

func TestUsageLogSeparatesDeliveredAndProviderOutputTokens(t *testing.T) {
	delivered := 7
	result := &ForwardResult{
		RequestID:             "delivered-output-test",
		Model:                 "claude-opus-4-6",
		Stream:                true,
		ClientDisconnect:      true,
		DeliveredOutputTokens: &delivered,
		Usage:                 ClaudeUsage{InputTokens: 10, OutputTokens: 321},
	}
	groupID := int64(1)
	usageLog := (&GatewayService{}).buildRecordUsageLog(
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

	require.Equal(t, 7, usageLog.OutputTokens)
	require.Equal(t, 321, usageLog.UpstreamOutputTokens)
}

func TestCompletedUsageLogUsesProviderOutputTokens(t *testing.T) {
	delivered := 7
	result := &ForwardResult{
		RequestID:             "completed-output-test",
		Model:                 "claude-opus-4-6",
		Stream:                true,
		DeliveredOutputTokens: &delivered,
		Usage:                 ClaudeUsage{InputTokens: 10, OutputTokens: 321},
	}
	groupID := int64(1)
	usageLog := (&GatewayService{}).buildRecordUsageLog(
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

	require.Equal(t, 321, usageLog.OutputTokens)
	require.Equal(t, 321, usageLog.UpstreamOutputTokens)
}

func TestCompletedOpenAIStreamRecordsProviderOutputTokens(t *testing.T) {
	delivered := 7
	result := &OpenAIForwardResult{
		DeliveredOutputTokens: &delivered,
		Usage:                 OpenAIUsage{OutputTokens: 321},
	}

	require.Equal(t, 321, recordedOpenAIOutputTokens(result))
	require.Equal(t, 321, customerBillableOutputTokens(result.ClientDisconnect, result.DeliveredOutputTokens, result.Usage.OutputTokens))
}

func TestCustomerBillableOutputTokensUsesDeliveredOnlyAfterDisconnect(t *testing.T) {
	delivered := 1935

	require.Equal(t, 1935, customerBillableOutputTokens(true, &delivered, 13811))
	require.Equal(t, 13811, customerBillableOutputTokens(false, &delivered, 13811))
	require.Equal(t, 13811, customerBillableOutputTokens(true, nil, 13811))
}
