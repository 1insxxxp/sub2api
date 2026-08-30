//go:build unit

package service

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// TestOpenAIResponsesEmptyCompletedFailsOver verifies that a Responses stream
// ending with an empty response.completed (no output, no usage, no error) is
// turned into a failover error instead of a successful empty reply (issue
// #5009).
func TestOpenAIResponsesEmptyCompletedFailsOver(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_empty\",\"object\":\"response\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_empty\",\"object\":\"response\",\"status\":\"completed\"}}\n\n",
		)),
	}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	c, recorder := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.144.1")
	account := newOpenAIImageGenerationControlTestAccount()
	account.Extra = map[string]any{"openai_passthrough": true}

	body := []byte(`{
		"model":"gpt-5.6-sol",
		"stream":true,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]
	}`)

	_, err := svc.Forward(context.Background(), c, account, body)
	require.Error(t, err)
	var failoverErr *UpstreamFailoverError
	require.True(t, errors.As(err, &failoverErr), "empty completed must produce UpstreamFailoverError, got: %v", err)
	require.Equal(t, http.StatusBadGateway, failoverErr.StatusCode)
	require.Empty(t, recorder.Body.String(), "no empty success stream may reach the client")
}

// TestOpenAIResponsesEmptyCompletedWithOutputSucceeds ensures streams with real
// semantic output are untouched.
func TestOpenAIResponsesEmptyCompletedWithOutputSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_ok\",\"object\":\"response\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.output_text.delta\",\"delta\":\"hello\"}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_ok\",\"object\":\"response\",\"status\":\"completed\",\"usage\":{\"input_tokens\":10,\"output_tokens\":5,\"total_tokens\":15}}}\n\n",
		)),
	}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	c, recorder := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.144.1")
	account := newOpenAIImageGenerationControlTestAccount()
	account.Extra = map[string]any{"openai_passthrough": true}

	body := []byte(`{
		"model":"gpt-5.6-sol",
		"stream":true,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]
	}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Contains(t, recorder.Body.String(), "hello")
	require.NotNil(t, result.Usage)
	require.Equal(t, 10, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
}

// TestOpenAIResponsesEmptyCompletedWithUsageSucceeds ensures a completed event
// carrying usage is not mistaken for a silent refusal even without output.
func TestOpenAIResponsesEmptyCompletedWithUsageSucceeds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_usage\",\"object\":\"response\",\"status\":\"in_progress\"}}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_usage\",\"object\":\"response\",\"status\":\"completed\",\"usage\":{\"input_tokens\":3,\"output_tokens\":0,\"total_tokens\":3}}}\n\n",
		)),
	}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	c, _ := newOpenAIImageGenerationControlTestContext(true, "codex_cli_rs/0.144.1")
	account := newOpenAIImageGenerationControlTestAccount()
	account.Extra = map[string]any{"openai_passthrough": true}

	body := []byte(`{
		"model":"gpt-5.6-sol",
		"stream":true,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]
	}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Usage)
	require.Equal(t, 3, result.Usage.InputTokens)
}

func TestOpenAIResponsesStreamingAddsOutputTokenDetailsForGrokCompatUsage(t *testing.T) {
	gin.SetMode(gin.TestMode)

	upstream := &httpUpstreamRecorder{resp: &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body: io.NopCloser(strings.NewReader(
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\"ok\"}\n\n" +
				"data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_grok_usage\",\"object\":\"response\",\"model\":\"grok-4.6\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"id\":\"msg_1\",\"role\":\"assistant\",\"status\":\"completed\",\"content\":[{\"type\":\"output_text\",\"text\":\"ok\"}]}],\"usage\":{\"input_tokens\":14840,\"output_tokens\":795,\"total_tokens\":15635,\"input_tokens_details\":{\"cached_tokens\":4544},\"reasoning_tokens\":769}}}\n\n",
		)),
	}}
	svc := newOpenAIImageGenerationControlTestService(upstream)
	c, recorder := newOpenAIImageGenerationControlTestContext(true, "grok-shell/1.0.0 (macos; aarch64)")
	account := newOpenAIImageGenerationControlTestAccount()
	account.Extra = map[string]any{"openai_responses_supported": true}

	body := []byte(`{
		"model":"grok-4.6",
		"stream":true,
		"input":[{"type":"message","role":"user","content":[{"type":"input_text","text":"continue"}]}]
	}`)

	result, err := svc.Forward(context.Background(), c, account, body)
	require.NoError(t, err)
	require.NotNil(t, result)

	completed := firstOpenAISSEDataPayloadWithType(recorder.Body.String(), "response.completed")
	require.NotEmpty(t, completed)
	require.True(t, gjson.Get(completed, "response.usage.output_tokens_details").Exists())
	require.Equal(t, int64(769), gjson.Get(completed, "response.usage.output_tokens_details.reasoning_tokens").Int())
}

func firstOpenAISSEDataPayloadWithType(body string, eventType string) string {
	for _, line := range strings.Split(body, "\n") {
		data, ok := extractOpenAISSEDataLine(strings.TrimSpace(line))
		if !ok || gjson.Get(data, "type").String() != eventType {
			continue
		}
		return data
	}
	return ""
}

func TestOpenAIResponsesCompletedEventIsEmpty(t *testing.T) {
	cases := []struct {
		name  string
		data  string
		usage *OpenAIUsage
		want  bool
	}{
		{
			name: "bare completed",
			data: `{"type":"response.completed"}`,
			want: true,
		},
		{
			name: "completed with empty output array",
			data: `{"type":"response.completed","response":{"id":"r1","status":"completed","output":[]}}`,
			want: true,
		},
		{
			name: "completed with usage",
			data: `{"type":"response.completed","response":{"id":"r1","status":"completed","usage":{"input_tokens":1,"output_tokens":1}}}`,
			want: false,
		},
		{
			name: "completed with error",
			data: `{"type":"response.completed","response":{"id":"r1","status":"completed","error":{"code":"x"}}}`,
			want: false,
		},
		{
			name: "completed with output item",
			data: `{"type":"response.completed","response":{"id":"r1","status":"completed","output":[{"type":"message","id":"msg_1"}]}}`,
			want: false,
		},
		{
			name: "accumulated usage",
			data: `{"type":"response.completed"}`,
			usage: &OpenAIUsage{
				InputTokens:  7,
				OutputTokens: 2,
			},
			want: false,
		},
		{
			name: "invalid json",
			data: `{"type":`,
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, openAIResponsesCompletedEventIsEmpty([]byte(tc.data), tc.usage))
		})
	}
}
