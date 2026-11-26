package service

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type delayedGeminiFrames struct {
	frames []string
	index  int
}

func (r *delayedGeminiFrames) Read(p []byte) (int, error) {
	if r.index == len(r.frames) {
		return 0, io.EOF
	}
	if r.index > 0 {
		time.Sleep(30 * time.Millisecond)
	}
	n := copy(p, r.frames[r.index])
	r.frames[r.index] = r.frames[r.index][n:]
	if r.frames[r.index] == "" {
		r.index++
	}
	return n, nil
}

func TestGeminiStreamPhaseTiming(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "native"} {
		t.Run(protocol, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(&delayedGeminiFrames{frames: []string{
				"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"reason\",\"thought\":true}]}}]}\n\n",
				"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"answer\"}]},\"finishReason\":\"STOP\"}]}\n\n",
			}})}
			svc := &GeminiMessagesCompatService{}
			start := time.Now()
			var outcome ResponseOutcome
			if protocol == "chat" {
				result, err := svc.handleChatCompletionsStreamingResponseFromGemini(context.Background(), c, resp, start, "gemini-test", nil, false, false, nil, ModelFirstOutputTimeoutPolicy{})
				require.NoError(t, err)
				require.NotNil(t, result)
				outcome = result.outcome
			} else {
				result, err := svc.handleNativeStreamingResponseWithContext(context.Background(), c, resp, start, "gemini-test", false, nil, "", nil, ModelFirstOutputTimeoutPolicy{})
				require.NoError(t, err)
				require.NotNil(t, result)
				outcome = result.outcome
			}
			require.NotNil(t, outcome.UpstreamFirstEventMs)
			require.NotNil(t, outcome.UpstreamFirstReasoningMs)
			require.NotNil(t, outcome.UpstreamFirstTextMs)
			require.NotNil(t, outcome.DownstreamFirstContentMs)
			require.GreaterOrEqual(t, *outcome.UpstreamFirstTextMs-*outcome.UpstreamFirstReasoningMs, 20)
			require.Less(t, *outcome.DownstreamFirstContentMs, *outcome.UpstreamFirstTextMs)
			require.True(t, strings.Contains(rec.Body.String(), "answer"))
		})
	}
}

func TestGeminiStreamPhaseTimingFailedWritesAndAbsentText(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, protocol := range []string{"chat", "native"} {
		t.Run(protocol, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
			c.Writer = &failWriteResponseWriter{ResponseWriter: c.Writer}
			body := "data: \n\ndata: [DONE]\n\ndata: {\"candidates\":[{\"content\":{\"parts\":[{\"functionCall\":{\"name\":\"test\",\"args\":{}}}]},\"finishReason\":\"STOP\"}]}\n\n"
			resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(strings.NewReader(body))}
			var outcome ResponseOutcome
			if protocol == "chat" {
				result, err := (&GeminiMessagesCompatService{}).handleChatCompletionsStreamingResponseFromGemini(context.Background(), c, resp, time.Now(), "gemini-test", nil, false, false, nil, ModelFirstOutputTimeoutPolicy{})
				require.NoError(t, err)
				require.NotNil(t, result)
				outcome = result.outcome
			} else {
				result, err := (&GeminiMessagesCompatService{}).handleNativeStreamingResponseWithContext(context.Background(), c, resp, time.Now(), "gemini-test", false, nil, "", nil, ModelFirstOutputTimeoutPolicy{})
				require.NoError(t, err)
				require.NotNil(t, result)
				outcome = result.outcome
			}
			require.NotNil(t, outcome.UpstreamFirstEventMs)
			require.Nil(t, outcome.UpstreamFirstReasoningMs)
			require.Nil(t, outcome.UpstreamFirstTextMs)
			require.Nil(t, outcome.DownstreamFirstContentMs)
		})
	}
}

func TestGeminiStreamPhaseTimingIgnoresInvalidAndEmptyPayloads(t *testing.T) {
	timing := geminiStreamPhaseTiming{start: time.Now()}
	for _, raw := range []string{"", "[DONE]", "{} invalid"} {
		timing.observeUpstream([]byte(raw))
	}
	require.Nil(t, timing.firstEvent)
	timing.observeUpstream([]byte(`{"candidates":[{"content":{"parts":[{"text":"  "}]}}]}`))
	require.NotNil(t, timing.firstEvent)
	require.Nil(t, timing.firstReasoning)
	require.Nil(t, timing.firstText)
	require.Nil(t, timing.downstreamContent)
}

func TestGeminiNativeStreamPhaseTimingWaitsForEventTerminator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/event-stream"}}, Body: io.NopCloser(&delayedGeminiFrames{frames: []string{
		"data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"answer\"}]}}]}\n",
		"\n",
		"data: {\"candidates\":[{\"finishReason\":\"STOP\"}]}\n\n",
	}})}
	result, err := (&GeminiMessagesCompatService{}).handleNativeStreamingResponseWithContext(context.Background(), c, resp, time.Now(), "gemini-test", false, nil, "", nil, ModelFirstOutputTimeoutPolicy{})
	require.NoError(t, err)
	require.NotNil(t, result.outcome.UpstreamFirstTextMs)
	require.NotNil(t, result.outcome.DownstreamFirstContentMs)
	require.GreaterOrEqual(t, *result.outcome.DownstreamFirstContentMs-*result.outcome.UpstreamFirstTextMs, 20)
}
