//go:build unit

package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func geminiInputCountingForwardAccount(enabled bool) *Account {
	account := geminiSignalTestAccount()
	account.ID = 6021
	if enabled {
		account.Extra = map[string]any{"gemini_input_counting": map[string]any{
			"mode": "vertex_like_discount_v1", "models": []any{"gemini-4-argon"},
		}}
	}
	return account
}

func callGeminiInputCountingForward(t *testing.T, path string, stream, enabled bool, metadata string, groups ...*Group) (*ForwardResult, *httptest.ResponseRecorder) {
	t.Helper()
	payload := `{"candidates":[{"content":{"parts":[{"text":"A complete answer."}],"role":"model"},"finishReason":"STOP"}],"usageMetadata":` + metadata + `}`
	if stream {
		payload = "data: " + payload + "\n\n"
	}
	svc, stub := newGeminiVertexAPIKeyTestService()
	stub.response.Body = io.NopCloser(strings.NewReader(payload))
	if stream {
		stub.response.Header.Set("Content-Type", "text/event-stream")
	}
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/"+path, nil)
	if len(groups) > 0 {
		group := groups[0]
		c.Set("api_key", &APIKey{GroupID: &group.ID, Group: group})
		ctx, _ := WithGatewayTokenRequestPricing(context.WithValue(c.Request.Context(), ctxkey.Group, group))
		c.Request = c.Request.WithContext(ctx)
	}
	account := geminiInputCountingForwardAccount(enabled)
	var result *ForwardResult
	var err error
	if path == "chat" {
		body := []byte(fmt.Sprintf(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}],"max_tokens":100,"stream":%t,"stream_options":{"include_usage":true}}`, stream))
		result, err = svc.ForwardAsChatCompletions(context.Background(), c, account, body)
	} else {
		result, err = callGeminiTraversalPath(t, path, svc, c, account, "gemini-4-argon", stream)
	}
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 1, stub.calls, "input normalization cannot issue another inference or count request")
	return result, rec
}

func geminiInputCountingCustomerInput(t *testing.T, path string, stream bool, body string) int {
	t.Helper()
	usagePath := map[string]string{"chat": "usage.prompt_tokens", "messages": "usage.input_tokens", "native": "usageMetadata.promptTokenCount"}[path]
	if !stream {
		value := gjson.Get(body, usagePath)
		require.True(t, value.Exists(), body)
		return int(value.Int())
	}
	value, found := 0, false
	for _, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if current := gjson.Get(payload, usagePath); current.Exists() {
			value, found = int(current.Int()), true
		} else if path == "messages" && !found && gjson.Get(payload, "message.usage.input_tokens").Exists() {
			value, found = int(gjson.Get(payload, "message.usage.input_tokens").Int()), true
		}
	}
	require.True(t, found, body)
	return value
}

func TestGeminiInputCountingForwardAllAdaptersPreserveRawUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"chat", "messages", "native"} {
		for _, stream := range []bool{false, true} {
			for _, enabled := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/stream=%t/enabled=%t", path, stream, enabled), func(t *testing.T) {
					result, rec := callGeminiInputCountingForward(t, path, stream, enabled,
						`{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":9,"thoughtsTokenCount":2,"totalTokenCount":1011}`)
					require.Equal(t, 1000, result.Usage.InputTokens, "upstream evidence must remain raw")
					require.Equal(t, 11, result.Usage.OutputTokens)
					require.Equal(t, UsageTokenSourceUpstream, result.InputTokensSource)
					input := geminiInputCountingCustomerInput(t, path, stream, rec.Body.String())
					if enabled {
						require.NotNil(t, result.GeminiInputCounting)
						require.Equal(t, result.GeminiInputCounting.CandidateInputTokens, input)
						require.Positive(t, input)
						require.Less(t, input, 1000, "the opted-in customer projection must use the local ceiling")
					} else {
						require.Equal(t, 1000, input)
					}
				})
			}
		}
	}
}

func TestGeminiInputCountingAttemptCapturesFinalRequestAndIsolatesAccounts(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	account := geminiInputCountingForwardAccount(true)
	attempt := BeginGeminiUpstreamAttempt(c, account, "gemini-4-argon", true)
	request, err := http.NewRequest(http.MethodPost, "https://fixture.invalid/v1beta/models/gemini-4-argon:streamGenerateContent", strings.NewReader(geminiTransportTestNativeBody))
	require.NoError(t, err)
	request.Header.Set("x-goog-api-key", "test-key")
	attempt.ObserveRequest(request)
	attempt.ObserveUsage([]byte(`{"usageMetadata":{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":9}}`))
	result := &ForwardResult{Model: "gemini-4-argon", Usage: ClaudeUsage{InputTokens: 1000, OutputTokens: 9}}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.NotNil(t, result.GeminiInputCounting)
	require.Equal(t, account.ID, result.GeminiInputCounting.AccountID)
	require.NotNil(t, result.GeminiUsageFallback.Input)
	require.Nil(t, result.GeminiUsageFallback.Input.CountRequest, "the opted-in model must not call the unsafe count endpoint")
	require.Empty(t, result.GeminiUsageFallback.Input.CalibrationKey, "raw cl100k input must not train the legacy calibration")
	recovered := GeminiResultFromLastAttempt(c, account.ID, "gemini-4-argon", true, 0)
	require.NotNil(t, recovered)
	require.Equal(t, result.GeminiInputCounting, recovered.GeminiInputCounting)

	other := geminiInputCountingForwardAccount(false)
	other.ID++
	second := BeginGeminiUpstreamAttempt(c, other, "gemini-4-argon", true)
	second.ObserveRequest(request)
	next := &ForwardResult{Model: "gemini-4-argon", Usage: result.Usage}
	finishGeminiUpstreamAttempt(second, next, nil)
	require.Nil(t, next.GeminiInputCounting, "a failover account cannot inherit the previous account's policy")
}

func TestGeminiInputCountingRedirectDropsPolicy(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, geminiInputCountingForwardAccount(true), "gemini-4-argon", true)
	request, err := http.NewRequest(http.MethodPost, "https://fixture.invalid/v1beta/models/gemini-4-argon:streamGenerateContent", strings.NewReader(geminiTransportTestNativeBody))
	require.NoError(t, err)
	request.Header.Set("x-goog-api-key", "test-key")
	attempt.ObserveRequest(request)
	redirected, err := http.NewRequest(http.MethodPost, "https://other.invalid/generate", nil)
	require.NoError(t, err)
	redirected.Header.Set("x-goog-api-key", "test-key")
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: redirected})
	result := &ForwardResult{}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.Nil(t, result.GeminiInputCounting, "the final request identity changed")

	attempt = BeginGeminiUpstreamAttempt(c, geminiInputCountingForwardAccount(true), "gemini-4-argon", true)
	attempt.ObserveRequest(request)
	sameOrigin, err := http.NewRequest(http.MethodPost, "https://fixture.invalid/v1beta/models/other-model:streamGenerateContent", nil)
	require.NoError(t, err)
	sameOrigin.Header.Set("x-goog-api-key", "test-key")
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: sameOrigin})
	result = &ForwardResult{}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.Nil(t, result.GeminiInputCounting, "same-origin redirects cannot transfer a model-specific policy")
}

func TestGeminiInputCountingNativeJSONFallbackNeverIncreasesEarlierFrame(t *testing.T) {
	payload := `[{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":10,"cachedContentTokenCount":0,"candidatesTokenCount":5,"totalTokenCount":15}},` +
		`{"candidates":[{"content":{"parts":[{"text":"done"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":9,"totalTokenCount":1009}}]`
	svc, stub := newGeminiVertexAPIKeyTestService()
	stub.response.Header.Set("Content-Type", "application/json")
	stub.response.Body = io.NopCloser(strings.NewReader(payload))
	c, rec := newGeminiNativeTestContext(t)
	_, err := svc.ForwardNative(context.Background(), c, geminiInputCountingForwardAccount(true),
		"gemini-4-argon", "streamGenerateContent", true, []byte(geminiTransportTestNativeBody))
	require.NoError(t, err)
	require.EqualValues(t, 10, gjson.Get(rec.Body.String(), "0.usageMetadata.promptTokenCount").Int())
	require.EqualValues(t, 15, gjson.Get(rec.Body.String(), "0.usageMetadata.totalTokenCount").Int())
	require.EqualValues(t, 32, gjson.Get(rec.Body.String(), "1.usageMetadata.promptTokenCount").Int())
}

func TestGeminiInputCountingNativeCumulativeUsageUsesFinalCacheEvidence(t *testing.T) {
	svc, stub := newGeminiVertexAPIKeyTestService()
	stub.response.Header.Set("Content-Type", "text/event-stream")
	stub.response.Body = io.NopCloser(strings.NewReader("data: " +
		`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":1000,"candidatesTokenCount":3}}` + "\n\ndata: " +
		`{"candidates":[{"content":{"parts":[{"text":" world"}]},"finishReason":"STOP"}],"usageMetadata":{"cachedContentTokenCount":0,"candidatesTokenCount":9,"thoughtsTokenCount":2,"totalTokenCount":1011}}` + "\n\n"))
	c, rec := newGeminiNativeTestContext(t)
	result, err := svc.ForwardNative(context.Background(), c, geminiInputCountingForwardAccount(true),
		"gemini-4-argon", "streamGenerateContent", true, []byte(geminiTransportTestNativeBody))
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1000, result.Usage.InputTokens)
	require.Equal(t, 11, result.Usage.OutputTokens)
	require.NotNil(t, result.GeminiInputCounting)
	input := geminiInputCountingCustomerInput(t, "native", true, rec.Body.String())
	require.Equal(t, result.GeminiInputCounting.CandidateInputTokens, input)
	var finalTotal int
	for _, line := range strings.Split(rec.Body.String(), "\n") {
		if strings.HasPrefix(line, "data:") {
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if value := gjson.Get(payload, "usageMetadata.totalTokenCount"); value.Exists() {
				finalTotal = int(value.Int())
			}
		}
	}
	require.Equal(t, input+11, finalTotal)
}

func TestGeminiInputCountingNativeJSONFallback(t *testing.T) {
	for _, array := range []bool{false, true} {
		t.Run(fmt.Sprintf("array=%t", array), func(t *testing.T) {
			payload := `{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":9,"totalTokenCount":1009}}`
			if array {
				payload = "[\n" + payload + "\n]"
			}
			svc, stub := newGeminiVertexAPIKeyTestService()
			svc.settingService = &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":60,"switch_seconds":60,"hard_cap_seconds":300}}`}}
			stub.response.Header.Set("Content-Type", "application/json")
			stub.response.Body = io.NopCloser(strings.NewReader(payload))
			c, rec := newGeminiNativeTestContext(t)
			result, err := svc.ForwardNative(context.Background(), c, geminiInputCountingForwardAccount(true),
				"gemini-4-argon", "streamGenerateContent", true, []byte(geminiTransportTestNativeBody))
			require.NoError(t, err)
			require.NotNil(t, result)
			require.NotNil(t, result.GeminiInputCounting)
			require.Equal(t, 1000, result.Usage.InputTokens)
			path := "usageMetadata.promptTokenCount"
			if array {
				path = "0." + path
			}
			require.Equal(t, result.GeminiInputCounting.CandidateInputTokens, int(gjson.Get(rec.Body.String(), path).Int()))
		})
	}
}

func TestGeminiInputCountingUnexpectedMediaDisablesProjection(t *testing.T) {
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			svc, stub := newGeminiVertexAPIKeyTestService()
			stub.response.Body = io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"parts":[{"text":"hello"},{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":9}}`))
			c, rec := newGeminiNativeTestContext(t)
			result, err := callGeminiTraversalPath(t, path, svc, c, geminiInputCountingForwardAccount(true), "gemini-4-argon", false)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Nil(t, result.GeminiInputCounting)
			require.Equal(t, 1000, geminiInputCountingCustomerInput(t, path, false, rec.Body.String()))
		})
	}
}

func TestGeminiInputCountingNativeLateEvidenceRestoresClientUsage(t *testing.T) {
	for _, tc := range []struct {
		name, terminal  string
		input, customer int
	}{
		{"cache", `{"candidates":[{"content":{"parts":[{"text":"done"}]},"finishReason":"STOP"}],"usageMetadata":{"cachedContentTokenCount":200,"candidatesTokenCount":9,"totalTokenCount":1009}}`, 800, 1000},
		{"explicit zero", `{"candidates":[{"content":{"parts":[{"text":"done"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":0,"cachedContentTokenCount":0,"candidatesTokenCount":9,"totalTokenCount":9}}`, 0, 0},
		{"media", `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}}]},"finishReason":"STOP"}],"usageMetadata":{"candidatesTokenCount":9,"totalTokenCount":1009}}`, 1000, 1000},
		{"media without usage", `{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"image/png","data":"aGVsbG8="}}]},"finishReason":"STOP"}]}`, 1000, 1000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, stub := newGeminiVertexAPIKeyTestService()
			stub.response.Header.Set("Content-Type", "text/event-stream")
			stub.response.Body = io.NopCloser(strings.NewReader("data: " +
				`{"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":1000,"cachedContentTokenCount":0,"candidatesTokenCount":3}}` + "\n\ndata: " + tc.terminal + "\n\n"))
			c, rec := newGeminiNativeTestContext(t)
			result, err := svc.ForwardNative(context.Background(), c, geminiInputCountingForwardAccount(true),
				"gemini-4-argon", "streamGenerateContent", true, []byte(geminiTransportTestNativeBody))
			require.NoError(t, err)
			require.Equal(t, tc.input, result.Usage.InputTokens)
			require.Equal(t, tc.customer, geminiInputCountingCustomerInput(t, "native", true, rec.Body.String()))
		})
	}
}

func TestGeminiInputCountingForwardExplicitZeroAndCacheAreUnchanged(t *testing.T) {
	for _, metadata := range []string{
		`{"promptTokenCount":0,"cachedContentTokenCount":0,"candidatesTokenCount":9}`,
		`{"promptTokenCount":1000,"candidatesTokenCount":9}`,
		`{"promptTokenCount":1000,"cachedContentTokenCount":200,"candidatesTokenCount":9}`,
	} {
		for _, path := range []string{"chat", "messages", "native"} {
			for _, stream := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%t/%s", path, stream, metadata), func(t *testing.T) {
					_, rec := callGeminiInputCountingForward(t, path, stream, true, metadata)
					_, baseline := callGeminiInputCountingForward(t, path, stream, false, metadata)
					require.Equal(t, geminiInputCountingCustomerInput(t, path, stream, baseline.Body.String()),
						geminiInputCountingCustomerInput(t, path, stream, rec.Body.String()))
				})
			}
		}
	}
}
