//go:build unit

package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

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

func TestDeliveredOutputTokensDoNotReplaceProviderUsage(t *testing.T) {
	collector := NewDownstreamOutputTokenCollector("claude-opus-4-6")
	collector.ObserveWritten([]byte("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"visible\"}}\n\n"))
	result := &ForwardResult{Usage: ClaudeUsage{OutputTokens: 321}}

	ApplyDeliveredOutputTokens(result, collector)

	require.Equal(t, 321, result.Usage.OutputTokens)
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Equal(t, collector.TokenCount(), *result.DeliveredOutputTokens)
	require.Equal(t, collector.TokenCount(), recordedOutputTokens(result))
}

func TestUsageLogSeparatesDeliveredAndProviderOutputTokens(t *testing.T) {
	delivered := 7
	result := &ForwardResult{
		RequestID:             "delivered-output-test",
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

	require.Equal(t, 7, usageLog.OutputTokens)
	require.Equal(t, 321, usageLog.UpstreamOutputTokens)
}
