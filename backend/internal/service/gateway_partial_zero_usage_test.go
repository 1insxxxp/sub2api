//go:build unit

package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayService_PartialZeroUsagePreservesDeliveredOutput(t *testing.T) {
	for _, tc := range []struct {
		name    string
		account func() *Account
	}{
		{name: "native", account: newAnthropicOAuthAccountForPartialUsageTest},
		{name: "passthrough", account: newAnthropicAPIKeyAccountForTest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			body := []byte(`{"model":"claude-opus-4-6","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
			parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformAnthropic)
			require.NoError(t, err)
			collector, restore := AttachDownstreamOutputTokenCollector(c, parsed.Model)
			defer restore()
			upstream := &anthropicHTTPUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": {"text/event-stream"}},
				Body: io.NopCloser(strings.NewReader(
					"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":0,\"output_tokens\":0}}}\n\n" +
						"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"The partial answer reached the client.\"}}\n\n")),
			}}

			result, err := newForwardPartialUsageServiceForTest(upstream).Forward(context.Background(), c, tc.account(), parsed)

			require.ErrorContains(t, err, "missing terminal event")
			require.NotNil(t, result, "delivered content must survive a missing final usage event")
			require.NotNil(t, result.Outcome)
			require.True(t, result.Outcome.HasText)
			require.False(t, result.Outcome.StreamCompleted)
			require.Zero(t, result.Usage.OutputTokens, "preserve the upstream measurement separately")
			ApplyDeliveredOutputTokens(result, collector)
			require.Greater(t, recordedOutputTokens(result), 0, "allow delivered-output billing fallback after interruption")
		})
	}
}

func TestPartialStreamUsageResultRetainsSemanticOutputWithoutUsage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		usage   *ClaudeUsage
		outcome ResponseOutcome
	}{
		{name: "text", usage: &ClaudeUsage{}, outcome: ResponseOutcome{HasText: true}},
		{name: "reasoning", usage: &ClaudeUsage{}, outcome: ResponseOutcome{HasReasoning: true}},
		{name: "tool call", usage: &ClaudeUsage{}, outcome: ResponseOutcome{HasToolCall: true}},
		{name: "media", usage: &ClaudeUsage{}, outcome: ResponseOutcome{HasMedia: true}},
		{name: "missing usage", outcome: ResponseOutcome{HasText: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			firstTokenMs := 12
			stream := &streamingResult{
				usage: tc.usage, outcome: tc.outcome, firstTokenMs: &firstTokenMs, clientDisconnect: true,
			}
			resp := &http.Response{Header: http.Header{"X-Request-Id": {"partial-zero-usage"}}}

			result := partialStreamUsageResult(nil, resp, stream, "alias", "upstream-model", "5m", time.Now(), context.Canceled)

			require.NotNil(t, result)
			require.Equal(t, ClaudeUsage{}, result.Usage)
			require.Same(t, &stream.outcome, result.Outcome)
			require.Equal(t, "partial-zero-usage", result.RequestID)
			require.Equal(t, "alias", result.Model)
			require.Equal(t, "upstream-model", result.UpstreamModel)
			require.Equal(t, "5m", result.CacheCreationTTLTarget)
			require.Equal(t, &firstTokenMs, result.FirstTokenMs)
			require.True(t, result.ClientDisconnect)
		})
	}
}

func TestPartialStreamUsageResultSkipsEmptyFailuresAndRetryAttempts(t *testing.T) {
	for _, tc := range []struct {
		name   string
		stream *streamingResult
		err    error
	}{
		{name: "missing stream", err: io.ErrUnexpectedEOF},
		{name: "missing usage and output", stream: &streamingResult{}, err: io.ErrUnexpectedEOF},
		{name: "metadata only", stream: &streamingResult{usage: &ClaudeUsage{}, outcome: ResponseOutcome{EventCount: 2}}, err: io.ErrUnexpectedEOF},
		{name: "retry with semantic output", stream: &streamingResult{usage: &ClaudeUsage{}, outcome: ResponseOutcome{HasText: true}}, err: &UpstreamFailoverError{StatusCode: 502}},
		{name: "wrapped retry with measured usage", stream: &streamingResult{usage: &ClaudeUsage{InputTokens: 12}, outcome: ResponseOutcome{HasText: true}}, err: fmt.Errorf("retry: %w", &UpstreamFailoverError{StatusCode: 502})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := partialStreamUsageResult(nil, &http.Response{Header: make(http.Header)}, tc.stream, "model", "model", "", time.Now(), tc.err)
			require.Nil(t, result, "empty failures and failover attempts must not produce a second billable result")
		})
	}
}

func TestPartialStreamUsageResultRetainsObservedUsageWithoutOutput(t *testing.T) {
	stream := &streamingResult{usage: &ClaudeUsage{InputTokens: 12, CacheReadInputTokens: 7}}
	result := partialStreamUsageResult(nil, &http.Response{Header: make(http.Header)}, stream, "model", "model", "", time.Now(), errors.New("stream interrupted"))
	require.NotNil(t, result)
	require.Equal(t, *stream.usage, result.Usage)
}
