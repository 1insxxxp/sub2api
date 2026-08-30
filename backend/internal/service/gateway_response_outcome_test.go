//go:build unit

package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/apicompat"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGatewayStreamingResponseCapturesAnthropicTextOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)

	body := "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.outcome.HasText)
	require.True(t, result.outcome.StreamCompleted)
}

func TestGatewayStreamingResponseMarksMissingTerminalAsUpstreamInterruption(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(bytes.NewBufferString(
			"event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":1}}}\n\n",
		)),
	}

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	require.Error(t, err)
	require.NotNil(t, result)
	require.False(t, result.outcome.StreamCompleted)
	require.Equal(t, DisconnectSourceUpstream, result.outcome.DisconnectSource)
	require.Equal(t, UpstreamErrorProtocol, result.outcome.UpstreamErrorKind)
}

func TestGatewayStreamingResponseCapturesAnthropicToolAndReasoningOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := "event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"tool_use\",\"id\":\"toolu_1\",\"name\":\"Read\",\"input\":{}}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"thinking_delta\",\"thinking\":\"private\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.outcome.HasToolCall)
	require.True(t, result.outcome.HasReasoning)
	require.True(t, result.outcome.StreamCompleted)
	require.False(t, result.outcome.HasText)
}

func TestGatewayStreamingResponseMarksClientDisconnectOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newMinimalGatewayService()
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer}
	body := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}

	result, err := svc.handleStreamingResponse(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model", "model", false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.clientDisconnect)
	require.Equal(t, DisconnectSourceClient, result.outcome.DisconnectSource)
	require.True(t, result.outcome.StreamCompleted)
}

func TestGatewayNonStreamingResponseCapturesAnthropicToolOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := []byte(`{"type":"message","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"private":"value"}}],"stop_reason":"tool_use","usage":{"input_tokens":4,"output_tokens":2}}`)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(body))}
	svc := &GatewayService{cfg: &config.Config{}, rateLimitService: &RateLimitService{}}
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)

	usage, err := svc.handleNonStreamingResponse(ctx, resp, c, &Account{ID: 1}, "model", "model")
	require.NoError(t, err)
	require.NotNil(t, usage)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasToolCall)
	require.True(t, outcome.StreamCompleted)
}

func TestGatewayAnthropicPassthroughStreamingCapturesTextOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(bytes.NewBufferString(body))}

	result, err := svc.handleStreamingResponseAnthropicAPIKeyPassthrough(context.Background(), resp, c, &Account{ID: 1}, time.Now(), "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.outcome.HasText)
	require.True(t, result.outcome.StreamCompleted)
}

func TestGatewayAnthropicPassthroughNonStreamingCapturesToolOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &GatewayService{cfg: &config.Config{}}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	body := []byte(`{"type":"message","content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"private":"value"}}],"stop_reason":"tool_use","usage":{"input_tokens":4,"output_tokens":2}}`)
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(body))}
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)

	usage, err := svc.handleNonStreamingResponseAnthropicAPIKeyPassthrough(ctx, resp, c, &Account{ID: 1})
	require.NoError(t, err)
	require.NotNil(t, usage)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasToolCall)
	require.True(t, outcome.StreamCompleted)
}

func TestGatewayChatCompletionsBridgeCapturesAnthropicOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[],"model":"model","usage":{"input_tokens":1}}}`,
			``,
			`event: content_block_delta`,
			`data: {"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}`,
			``,
			`event: message_stop`,
			`data: {"type":"message_stop"}`,
			``,
		}, "\n"))),
	}

	result, err := (&GatewayService{}).handleCCStreamingFromAnthropic(resp, c, "model", "model", "", nil, time.Now(), true)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Outcome)
	require.True(t, result.Outcome.HasText)
	require.True(t, result.Outcome.StreamCompleted)
}

func TestGatewayResponsesBridgeMarksAnthropicMissingTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(strings.Join([]string{
			`event: message_start`,
			`data: {"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"model":"model","usage":{"input_tokens":1}}}`,
			``,
		}, "\n"))),
	}

	result, err := (&GatewayService{}).handleResponsesBufferedStreamingResponse(resp, c, "model", "model", "", nil, time.Now(), apicompat.ResponsesClientToolMapping{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Outcome)
	require.True(t, result.Outcome.HasText)
	require.False(t, result.Outcome.StreamCompleted)
	require.Equal(t, UpstreamErrorProtocol, result.Outcome.UpstreamErrorKind)
}

func TestGatewayAntigravityClaudeCapturesConvertedOutcome(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newAntigravityTestService(&config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":1}}}\n\n",
		)),
	}

	result, err := svc.handleClaudeStreamingResponse(c, resp, time.Now(), "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.outcome.HasText)
	require.True(t, result.outcome.StreamCompleted)
}
