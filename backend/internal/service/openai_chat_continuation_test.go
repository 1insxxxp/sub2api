package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestRawChatContinuationDeadlineCancelsInitialHeaderWait(t *testing.T) {
	serverDone := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		close(serverDone)
	}))
	defer server.Close()
	requestCtx, cancel := withRawChatContinuationDeadline(context.Background(), domain.AntiTruncationPolicy{MaxDurationSeconds: 1}, time.Now())
	defer cancel()
	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, server.URL, nil)
	require.NoError(t, err)
	started := time.Now()
	_, err = http.DefaultClient.Do(req)
	require.Error(t, err)
	require.ErrorIs(t, requestCtx.Err(), context.DeadlineExceeded)
	require.Less(t, time.Since(started), 2*time.Second)
	select {
	case <-serverDone:
	case <-time.After(time.Second):
		t.Fatal("deadline did not cancel stalled initial response headers")
	}
}

func TestBuildRawChatContinuationBodyAppendsAssistantAndPrompt(t *testing.T) {
	body := []byte(`{"model":"public","stream":true,"max_tokens":100,"messages":[{"role":"user","content":"question"}]}`)
	got, err := buildRawChatContinuationBody(body, "partial answer", 42)
	require.NoError(t, err)
	require.Equal(t, "public", gjson.GetBytes(got, "model").String())
	require.Equal(t, int64(42), gjson.GetBytes(got, "max_tokens").Int())
	require.Equal(t, 3, len(gjson.GetBytes(got, "messages").Array()))
	require.Equal(t, "assistant", gjson.GetBytes(got, "messages.1.role").String())
	require.Equal(t, "partial answer", gjson.GetBytes(got, "messages.1.content").String())
	require.Equal(t, "Continue the response from where you left off. Do not repeat any earlier text.", gjson.GetBytes(got, "messages.2.content").String())
}

func TestStreamRawChatCompletionsContinuesLengthRoundWithoutIntermediateTerminal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var continuationBody []byte
	round := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if round == 1 {
			continuationBody, _ = io.ReadAll(r.Body)
		}
		round++
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", fmt.Sprintf("round-%d", round))
		if round == 1 {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n"))
			_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":4,\"completion_tokens\":3,\"total_tokens\":7}}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\" continued\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"choices\":[],\"usage\":{\"prompt_tokens\":8,\"completion_tokens\":4,\"total_tokens\":12}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer upstream.Close()

	initialBody := []byte(`{"model":"public","stream":true,"max_tokens":10,"messages":[{"role":"user","content":"question"}]}`)
	initialReq, err := http.NewRequest(http.MethodPost, upstream.URL, bytes.NewReader(initialBody))
	require.NoError(t, err)
	initialResp, err := http.DefaultClient.Do(initialReq)
	require.NoError(t, err)
	defer initialResp.Body.Close()

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(initialBody))
	policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2, MaxOutputTokens: 10, MaxDurationSeconds: 30}
	service := &OpenAIGatewayService{}
	result, err := service.streamRawChatCompletions(c, initialResp, &Account{ID: 1, Platform: PlatformOpenAI}, "public", "public", "upstream", nil, nil, time.Now(), len(initialBody), policy, func(ctx context.Context, assistant string, remaining int) (*http.Response, error) {
		body, buildErr := buildRawChatContinuationBody(initialBody, assistant, remaining)
		if buildErr != nil {
			return nil, buildErr
		}
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, bytes.NewReader(body))
		if reqErr != nil {
			return nil, reqErr
		}
		return http.DefaultClient.Do(req)
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Equal(t, 7, result.Usage.OutputTokens)
	require.Equal(t, 12, result.Usage.InputTokens)
	require.Equal(t, "partial", gjson.GetBytes(continuationBody, "messages.1.content").String())
	require.Equal(t, int64(7), gjson.GetBytes(continuationBody, "max_tokens").Int())
	clientStream := recorder.Body.String()
	require.Contains(t, clientStream, "partial")
	require.Contains(t, clientStream, " continued")
	require.NotContains(t, clientStream, `"finish_reason":"length"`)
	require.Contains(t, clientStream, `"delta":{"content":"partial"},"finish_reason":null`)
	require.Equal(t, 1, strings.Count(clientStream, "data: [DONE]"))
	require.Contains(t, clientStream, `"prompt_tokens":12`)
	require.Contains(t, clientStream, `"completion_tokens":7`)
	require.Contains(t, clientStream, `"total_tokens":19`)
	require.Contains(t, clientStream, "data: [DONE]\n\n")
}

func TestRawChatContinuationAllowedRejectsUnsafeRequestShapes(t *testing.T) {
	for name, body := range map[string]string{
		"multiple choices":  `{"n":2,"messages":[{"role":"user","content":"x"}]}`,
		"json response":     `{"response_format":{"type":"json_object"},"messages":[{"role":"user","content":"x"}]}`,
		"reasoning history": `{"messages":[{"role":"assistant","reasoning_content":"thought"}]}`,
		"reasoning part":    `{"messages":[{"role":"assistant","content":[{"type":"reasoning","text":"thought"}]}]}`,
	} {
		t.Run(name, func(t *testing.T) { require.False(t, rawChatContinuationAllowed([]byte(body))) })
	}
	require.True(t, rawChatContinuationAllowed([]byte(`{"messages":[{"role":"user","content":"hello"}]}`)))
}

func TestStreamRawChatContinuationStopsForMultiChoiceAndReasoningDeltas(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for name, body := range map[string]string{
		"multi choice output": "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"a\"},\"finish_reason\":null},{\"index\":1,\"delta\":{\"content\":\"b\"},\"finish_reason\":null}]}\n\n" + "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"length\"}]}\n\n" + "data: [DONE]\n\n",
		"reasoning output":    "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer\",\"reasoning_content\":\"thought\"},\"finish_reason\":\"length\"}]}\n\n" + "data: [DONE]\n\n",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x","messages":[{"role":"user","content":"hello"}]}`))
			calls := 0
			resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}
			_, err := (&OpenAIGatewayService{}).streamRawChatCompletions(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "x", "x", "x", nil, nil, time.Now(), 32,
				domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1}, func(context.Context, string, int) (*http.Response, error) {
					calls++
					return nil, nil
				})
			require.NoError(t, err)
			require.Zero(t, calls)
		})
	}
}

func TestStreamRawChatContinuationTreatsZeroOutputLimitAsUnbounded(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
	first := "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n" + "data: [DONE]\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(first))}
	called := false
	_, err := (&OpenAIGatewayService{}).streamRawChatCompletions(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "x", "x", "x", nil, nil, time.Now(), 32,
		domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxOutputTokens: 0}, func(_ context.Context, _ string, remaining int) (*http.Response, error) {
			called = true
			require.Zero(t, remaining)
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" + "data: [DONE]\n\n"))}, nil
		})
	require.NoError(t, err)
	require.True(t, called)
}

func TestClampRawChatOutputTokensZeroDoesNotAddCap(t *testing.T) {
	body := []byte(`{"model":"x","messages":[{"role":"user","content":"hello"}]}`)
	got, err := clampRawChatOutputTokens(body, 0)
	require.NoError(t, err)
	require.JSONEq(t, string(body), string(got))
}

func TestStreamRawChatContinuationStopsForNormalStopToolCallAndCancellation(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		cancel  bool
		wantErr bool
	}{
		{
			name: "normal stop",
			body: "data: {\"choices\":[{\"delta\":{\"content\":\"done\"},\"finish_reason\":null}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" + "data: [DONE]\n\n",
		},
		{
			name: "tool call",
			body: "data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"call_1\"}]},\"finish_reason\":null}]}\n\n" +
				"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"tool_calls\"}]}\n\n" + "data: [DONE]\n\n",
		},
		{
			name:    "client canceled before continuation",
			body:    "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n",
			cancel:  true,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			requestCtx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x","messages":[{"role":"user","content":"hello"}]}`)).WithContext(requestCtx)
			if tt.cancel {
				cancel()
			}
			calls := 0
			resp := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"x-request-id": []string{"test"}}, Body: io.NopCloser(strings.NewReader(tt.body))}
			_, err := (&OpenAIGatewayService{}).streamRawChatCompletions(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "x", "x", "x", nil, nil, time.Now(), 32,
				domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2}, func(context.Context, string, int) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: [DONE]\n\n"))}, nil
				})
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Zero(t, calls)
		})
	}
}

func TestStreamRawChatContinuationHandlesCleanEOFAndRoundCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name          string
		first         string
		second        string
		maxRounds     int
		wantLengthEnd bool
	}{
		{
			name:      "clean EOF continues",
			first:     "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n",
			second:    "data: {\"choices\":[{\"delta\":{\"content\":\" done\"},\"finish_reason\":null}]}\n\n" + "data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n" + "data: [DONE]\n\n",
			maxRounds: 1,
		},
		{
			name:          "round cap emits last length terminal",
			first:         "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n" + "data: [DONE]\n\n",
			second:        "data: {\"choices\":[{\"delta\":{\"content\":\" more\"},\"finish_reason\":\"length\"}]}\n\n" + "data: [DONE]\n\n",
			maxRounds:     1,
			wantLengthEnd: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x","messages":[{"role":"user","content":"hello"}]}`))
			calls := 0
			resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.first))}
			result, err := (&OpenAIGatewayService{}).streamRawChatCompletions(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "x", "x", "x", nil, nil, time.Now(), 32,
				domain.AntiTruncationPolicy{Enabled: true, MaxRounds: tc.maxRounds}, func(context.Context, string, int) (*http.Response, error) {
					calls++
					return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(tc.second))}, nil
				})
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, 1, calls)
			require.Equal(t, 1, result.AntiTruncationRounds)
			if tc.wantLengthEnd {
				require.Contains(t, recorder.Body.String(), `"finish_reason":"length"`)
			} else {
				require.NotContains(t, recorder.Body.String(), `"finish_reason":"length"`)
				require.Equal(t, 1, strings.Count(recorder.Body.String(), "data: [DONE]"))
			}
		})
	}
}

func TestStreamRawChatContinuationReturnsNextRoundError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x","messages":[{"role":"user","content":"hello"}]}`))
	first := "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n" + "data: [DONE]\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(first))}
	result, err := (&OpenAIGatewayService{}).streamRawChatCompletions(c, resp, &Account{ID: 1, Platform: PlatformOpenAI}, "x", "x", "x", nil, nil, time.Now(), 32,
		domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1}, func(context.Context, string, int) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusBadGateway, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"error":{"message":"upstream failed"}}`))}, nil
		})
	require.Error(t, err)
	require.NotNil(t, result)
	require.Contains(t, err.Error(), "continuation upstream request failed")
}

func TestStreamRawChatContinuationDeadlineClosesStalledCurrentAndNextRounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Run("current round", func(t *testing.T) {
		serverDone := make(chan struct{})
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":null}]}\n\n"))
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(serverDone)
		}))
		defer upstream.Close()
		initialResp, err := http.Get(upstream.URL)
		require.NoError(t, err)
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
		start := time.Now()
		_, err = (&OpenAIGatewayService{}).streamRawChatCompletions(c, initialResp, &Account{ID: 1, Platform: PlatformOpenAI}, "x", "x", "x", nil, nil, start, 32,
			domain.AntiTruncationPolicy{Enabled: true, MaxDurationSeconds: 1}, func(context.Context, string, int) (*http.Response, error) {
				t.Fatal("must not continue after a deadline")
				return nil, nil
			})
		require.Error(t, err)
		require.Less(t, time.Since(start), 2*time.Second)
		select {
		case <-serverDone:
		case <-time.After(time.Second):
			t.Fatal("deadline did not close the stalled upstream response")
		}
	})

	t.Run("next round", func(t *testing.T) {
		var requests atomic.Int32
		requests.Store(1)
		secondRoundCanceled := make(chan struct{})
		upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if requests.Add(1) == 1 {
				_, _ = w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n"))
				_, _ = w.Write([]byte("data: [DONE]\n\n"))
				return
			}
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()
			<-r.Context().Done()
			close(secondRoundCanceled)
		}))
		defer upstream.Close()
		initialResp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(
			"data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}]}\n\n" + "data: [DONE]\n\n",
		))}
		recorder := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(recorder)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x"}`))
		start := time.Now()
		result, streamErr := (&OpenAIGatewayService{}).streamRawChatCompletions(c, initialResp, &Account{ID: 1, Platform: PlatformOpenAI}, "x", "x", "x", nil, nil, start, 32,
			domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxDurationSeconds: 1}, func(ctx context.Context, _ string, _ int) (*http.Response, error) {
				req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, upstream.URL, strings.NewReader(`{}`))
				if reqErr != nil {
					return nil, reqErr
				}
				return http.DefaultClient.Do(req)
			})
		require.Less(t, time.Since(start), 2*time.Second)
		require.NotNil(t, result)
		require.Equal(t, int32(2), requests.Load())
		select {
		case <-secondRoundCanceled:
		case <-time.After(time.Second):
			t.Fatal("deadline did not cancel the stalled continuation request")
		}
		_ = streamErr
	})
}

func TestAntiTruncationPolicyContextRoundTrip(t *testing.T) {
	policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2, MaxOutputTokens: 500, MaxDurationSeconds: 20}
	ctx := WithRawChatAntiTruncationPolicy(t.Context(), policy)
	got, ok := RawChatAntiTruncationPolicyFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, policy, got)
}

func TestGroupAliasAntiTruncationPolicyIsPublicAliasScopedAndCanBeReplacedOnFallback(t *testing.T) {
	policy := domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 2}
	group := &Group{ModelAliases: []domain.ModelAlias{{Alias: "fast", UpstreamModel: "gpt-5.4", AntiTruncation: policy}}}
	ctx := WithGroupAliasRawChatAntiTruncationPolicy(t.Context(), group, "fast")
	got, ok := RawChatAntiTruncationPolicyFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, policy, got)

	ctx = WithGroupAliasRawChatAntiTruncationPolicy(ctx, &Group{}, "fast")
	_, ok = RawChatAntiTruncationPolicyFromContext(ctx)
	require.False(t, ok)
	ctx = WithGroupAliasRawChatAntiTruncationPolicy(ctx, group, "gpt-5.4")
	_, ok = RawChatAntiTruncationPolicyFromContext(ctx)
	require.False(t, ok)
}
