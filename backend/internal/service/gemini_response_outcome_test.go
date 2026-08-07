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
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiNativeNonStreamingResponseOutcomeCapturesAllOutputFamilies(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/model:generateContent", nil).WithContext(ctx)
	body := `{"candidates":[{"content":{"parts":[{"text":"hello"},{"text":"thinking","thought":true},{"functionCall":{"name":"lookup","args":{}}},{"inlineData":{"mimeType":"image/png","data":"private"}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2}}`
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}}

	usage, err := svc.handleNativeNonStreamingResponse(c, resp, false)
	require.NoError(t, err)
	require.NotNil(t, usage)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasText)
	require.True(t, outcome.HasReasoning)
	require.True(t, outcome.HasToolCall)
	require.True(t, outcome.HasMedia)
	require.True(t, outcome.StreamCompleted)
}

func TestGeminiCompatNonStreamingResponseOutcomeRecognizesPureEmptyCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil).WithContext(ctx)
	body := `{"candidates":[{"content":{"parts":[]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":0}}`
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}}

	usage, err := svc.handleNonStreamingResponse(c, resp, "model")
	require.NoError(t, err)
	require.NotNil(t, usage)
	outcome := collector.Snapshot()
	require.True(t, outcome.StreamCompleted)
	require.False(t, outcome.HasEffectiveOutput())
}

func TestGeminiNativeStreamingResponseOutcomeMarksMissingFinishAsInterrupted(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ctx, collector := WithResponseOutcomeCollector(context.Background(), http.StatusOK, http.StatusOK)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/model:streamGenerateContent", nil).WithContext(ctx)
	body := "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello\"}]}}]}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
	svc := &GeminiMessagesCompatService{cfg: &config.Config{}}

	result, err := svc.handleNativeStreamingResponse(c, resp, time.Now(), false)
	require.NoError(t, err)
	require.NotNil(t, result)
	outcome := collector.Snapshot()
	require.True(t, outcome.HasText)
	require.False(t, outcome.StreamCompleted)
	require.Equal(t, UpstreamErrorProtocol, outcome.UpstreamErrorKind)
}

func TestGeminiAntigravityStreamingResponseOutcomeCapturesMediaAndCompletion(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newAntigravityTestService(&config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/model:streamGenerateContent", nil)
	body := "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"inlineData\":{\"mimeType\":\"image/png\",\"data\":\"private\"}}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":1}}}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}

	result, err := svc.handleGeminiStreamingResponse(c, resp, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.outcome.HasMedia)
	require.True(t, result.outcome.StreamCompleted)
}

func TestGeminiAntigravityBufferedResponseOutcomeCapturesText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := newAntigravityTestService(&config.Config{Gateway: config.GatewayConfig{MaxLineSize: defaultMaxLineSize}})
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1beta/models/model:generateContent", nil)
	body := "data: {\"response\":{\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":1,\"candidatesTokenCount\":1}}}\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}

	result, err := svc.handleGeminiStreamToNonStreaming(c, resp, time.Now())
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.outcome.HasText)
	require.True(t, result.outcome.StreamCompleted)
}
