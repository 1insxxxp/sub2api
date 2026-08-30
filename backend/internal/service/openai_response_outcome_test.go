//go:build unit

package service

import (
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

func TestOpenAIStreamingResponseOutcomeCapturesTextReasoningAndFunctionCall(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		``,
		`data: {"type":"response.reasoning_summary_text.delta","delta":"thinking"}`,
		``,
		`data: {"type":"response.output_item.added","item":{"type":"function_call","name":"lookup"}}`,
		``,
		`data: {"type":"response.completed","response":{"status":"completed","usage":{"input_tokens":1,"output_tokens":2}}}`,
		``,
	}, "\n")
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &OpenAIGatewayService{cfg: &config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}}}

	result, err := svc.handleStreamingResponse(ctx, resp, c, &Account{ID: 1}, time.Now(), "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasText)
	require.True(t, outcome.HasReasoning)
	require.True(t, outcome.HasToolCall)
	require.True(t, outcome.StreamCompleted)
}

func TestOpenAINonStreamingResponseOutcomeRecognizesPureEmptyCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	body := `{"id":"resp_1","status":"completed","output":[],"usage":{"input_tokens":5,"output_tokens":0}}`
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	result, err := svc.handleNonStreamingResponse(ctx, resp, c, &Account{ID: 1}, "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	outcome := collector.Snapshot()
	require.True(t, outcome.StreamCompleted)
	require.False(t, outcome.HasEffectiveOutput())
}

func TestOpenAINonStreamingSSEBridgePreservesOutcomeAndCodexTurnState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	c.Request.Header.Set("session_id", "sess-sse-bridge")
	c.Set("api_key", &APIKey{ID: 7})
	body := strings.Join([]string{
		`data: {"type":"response.output_text.delta","delta":"hello"}`,
		``,
		`data: {"type":"response.completed","response":{"id":"resp_sse_bridge","status":"completed","output":[],"usage":{"input_tokens":1,"output_tokens":1}}}`,
		``,
	}, "\n")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":       []string{"text/event-stream"},
			"X-Codex-Turn-State": []string{"turn-state-sse-bridge"},
		},
		Body: io.NopCloser(strings.NewReader(body)),
	}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	result, err := svc.handleNonStreamingResponse(ctx, resp, c, &Account{ID: 42}, "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "turn-state-sse-bridge", rec.Header().Get("X-Codex-Turn-State"))
	outcome := collector.Snapshot()
	require.True(t, outcome.HasText)
	require.True(t, outcome.StreamCompleted)
	raw, ok := svc.openaiCodexTurnStateOrigins.Load("7\x00sess-sse-bridge")
	require.True(t, ok)
	origin, ok := raw.(openAICodexTurnStateOrigin)
	require.True(t, ok)
	require.Equal(t, int64(42), origin.accountID)
}

func TestOpenAIImagesNonStreamingResponseOutcomeRecognizesMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil).WithContext(ctx)
	body := `{"created":1710000007,"data":[{"b64_json":"private-image-data"}]}`
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	_, _, _, err := svc.handleOpenAIImagesNonStreamingResponse(resp, c)
	require.NoError(t, err)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasMedia)
	require.True(t, outcome.StreamCompleted)
}

func TestOpenAIImagesStreamingResponseOutcomeRecognizesCompletedMedia(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/images/generations", nil).WithContext(ctx)
	body := "data: {\"type\":\"image_generation.completed\",\"b64_json\":\"private-image-data\"}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	_, _, _, _, err := svc.handleOpenAIImagesStreamingResponse(resp, c, time.Now())
	require.NoError(t, err)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasMedia)
	require.True(t, outcome.StreamCompleted)
}

func TestOpenAIPassthroughStreamingResponseOutcomeCapturesText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	body := "data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
		"data: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	result, err := svc.handleStreamingResponsePassthrough(ctx, resp, c, &Account{ID: 1}, time.Now(), "model", "model")
	require.NoError(t, err)
	require.NotNil(t, result)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasText)
	require.True(t, outcome.StreamCompleted)
}

func TestOpenAIResponsesChatFallbackResponseOutcomeCapturesChatDelta(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, _ := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", nil).WithContext(ctx)
	body := "data: {\"id\":\"chatcmpl_1\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello\"},\"finish_reason\":null}]}\n\n" +
		"data: {\"id\":\"chatcmpl_1\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1}}\n\n" +
		"data: [DONE]\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &OpenAIGatewayService{cfg: &config.Config{}}

	result, err := svc.streamChatCompletionsAsResponses(c, resp, "model", nil, nil, false, map[string]apicompat.NamespacedToolName{}, "model", "model", nil, nil, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Outcome)
	require.True(t, result.Outcome.HasText)
	require.True(t, result.Outcome.StreamCompleted)
}
