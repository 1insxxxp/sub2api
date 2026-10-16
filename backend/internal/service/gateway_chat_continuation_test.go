package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func anthropicTextStream(text, stopReason string, inputTokens, outputTokens int) string {
	return "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg\",\"type\":\"message\",\"role\":\"assistant\",\"model\":\"claude-test\",\"content\":[],\"stop_reason\":null,\"usage\":{\"input_tokens\":" + itoa(inputTokens) + ",\"output_tokens\":0}}}\n\n" +
		"event: content_block_start\ndata: {\"type\":\"content_block_start\",\"index\":0,\"content_block\":{\"type\":\"text\",\"text\":\"\"}}\n\n" +
		"event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":" + `"` + text + `"` + "}}\n\n" +
		"event: content_block_stop\ndata: {\"type\":\"content_block_stop\",\"index\":0}\n\n" +
		"event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"" + stopReason + "\"},\"usage\":{\"input_tokens\":" + itoa(inputTokens) + ",\"output_tokens\":" + itoa(outputTokens) + "}}\n\n" +
		"event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n"
}

func TestBuildAnthropicChatContinuationBodyAppendsTranscriptAndRemainingCap(t *testing.T) {
	body := []byte(`{"model":"claude-public","max_tokens":100,"messages":[{"role":"user","content":"hello"}]}`)
	got, err := buildAnthropicChatContinuationBody(body, "partial", 73)
	require.NoError(t, err)
	require.Equal(t, int64(73), gjson.GetBytes(got, "max_tokens").Int())
	require.Equal(t, "partial", gjson.GetBytes(got, "messages.1.content").String())
	require.Equal(t, "assistant", gjson.GetBytes(got, "messages.1.role").String())
	require.Equal(t, "user", gjson.GetBytes(got, "messages.2.role").String())
}

func TestBuildAnthropicChatContinuationBodyDoesNotRaisePerRoundHardLimit(t *testing.T) {
	body := []byte(`{"model":"claude-public","max_tokens":20,"messages":[{"role":"user","content":"hello"}]}`)
	got, err := buildAnthropicChatContinuationBody(body, "partial", 85)
	require.NoError(t, err)
	require.Equal(t, int64(20), gjson.GetBytes(got, "max_tokens").Int())
}

func TestAnthropicChatContinuationAllowsPlainStringContent(t *testing.T) {
	body := []byte(`{"model":"claude-public","messages":[{"role":"user","content":"hello"}]}`)
	require.True(t, anthropicChatContinuationAllowed(body))
}

func TestHandleCCStreamingFromAnthropicContinuesAndAggregatesUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-public"}`))
	initial := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"x-request-id": []string{"first"}}, Body: io.NopCloser(strings.NewReader(anthropicTextStream("partial", "max_tokens", 5, 3)))}
	called := false
	policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: 100, MaxDurationSeconds: 10}
	result, err := (&GatewayService{}).handleCCStreamingFromAnthropicWithContinuation(initial, c, "claude-public", "claude-upstream", "", nil, time.Now(), policy,
		func(_ context.Context, assistant string, remaining int) (*http.Response, error) {
			called = true
			require.Equal(t, "partial", assistant)
			require.Equal(t, 97, remaining)
			return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"x-request-id": []string{"second"}}, Body: io.NopCloser(strings.NewReader(anthropicTextStream(" complete", "end_turn", 7, 4)))}, nil
		})
	require.NoError(t, err)
	require.True(t, called)
	require.NotNil(t, result)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
	stream := recorder.Body.String()
	require.Contains(t, stream, "partial")
	require.Contains(t, stream, " complete")
	require.Equal(t, 1, strings.Count(stream, "data: [DONE]"))
}

func TestHandleCCStreamingFromAnthropicDoesNotContinueNormalStop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-public"}`))
	initial := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(anthropicTextStream("done", "end_turn", 2, 2)))}
	called := false
	_, err := (&GatewayService{}).handleCCStreamingFromAnthropicWithContinuation(initial, c, "claude-public", "claude-upstream", "", nil, time.Now(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2},
		func(context.Context, string, int) (*http.Response, error) { called = true; return nil, nil })
	require.NoError(t, err)
	require.False(t, called)
}

func TestHandleCCStreamingFromAnthropicDoesNotContinueAfterMessageStopWithoutStopReason(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-public"}`))
	initial := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(anthropicTextStream("complete", "", 2, 0)))}
	called := false
	_, err := (&GatewayService{}).handleCCStreamingFromAnthropicWithContinuation(initial, c, "claude-public", "claude-upstream", "", nil, time.Now(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2},
		func(context.Context, string, int) (*http.Response, error) { called = true; return nil, nil })
	require.NoError(t, err)
	require.False(t, called)
	require.Contains(t, recorder.Body.String(), "data: [DONE]")
}

func TestHandleCCStreamingFromAnthropicMissingUsageUsesEstimateForBudget(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-public"}`))
	stream := anthropicTextStream("123456789012", "max_tokens", 2, 0)
	stream = strings.ReplaceAll(stream, `,"usage":{"input_tokens":2,"output_tokens":0}`, "")
	require.NotContains(t, stream, `"usage"`)
	initial := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(stream))}
	called := false
	result, err := (&GatewayService{}).handleCCStreamingFromAnthropicWithContinuation(initial, c, "claude-public", "claude-upstream", "", nil, time.Now(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: 3},
		func(context.Context, string, int) (*http.Response, error) { called = true; return nil, nil })
	require.NoError(t, err)
	require.False(t, called, "12 runes estimate to 3 output tokens and exhaust the budget")
	require.Zero(t, result.AntiTruncationRounds)
}

func TestHandleCCStreamingFromAnthropicUsesTextEstimateForOutputBudgetWithoutUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("remaining budget", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-public"}`))
		initial := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(anthropicTextStream("12345678", "max_tokens", 2, 0)))}
		called := false
		result, err := (&GatewayService{}).handleCCStreamingFromAnthropicWithContinuation(initial, c, "claude-public", "claude-upstream", "", nil, time.Now(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: 3},
			func(_ context.Context, _ string, remaining int) (*http.Response, error) {
				called = true
				require.Equal(t, 1, remaining, "8 runes estimate to 2 output tokens")
				return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(anthropicTextStream("ok", "end_turn", 2, 1)))}, nil
			})
		require.NoError(t, err)
		require.True(t, called)
		require.Equal(t, 1, result.AntiTruncationRounds)
	})

	t.Run("estimated budget exhausted", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"claude-public"}`))
		initial := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(anthropicTextStream("123456789012", "max_tokens", 2, 0)))}
		called := false
		result, err := (&GatewayService{}).handleCCStreamingFromAnthropicWithContinuation(initial, c, "claude-public", "claude-upstream", "", nil, time.Now(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: 3},
			func(context.Context, string, int) (*http.Response, error) { called = true; return nil, nil })
		require.NoError(t, err)
		require.False(t, called)
		require.Zero(t, result.AntiTruncationRounds)
	})
}
