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

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountContinuationForwardRepairsSafeUnexpectedEOF(t *testing.T) {
	c, recorder := accountContinuationTestContext()
	body := []byte(`{"model":"long-text","stream":true,"messages":[{"role":"user","content":"write"}]}`)
	calls := 0
	result, err := runForwardWithAccountContinuation(c.Request.Context(), c, accountContinuationTestAccount(true), body, AccountContinuationChat, "long-text", true,
		func(_ context.Context, _ []byte) (*ForwardResult, error) {
			calls++
			if calls == 1 {
				_, _ = fmt.Fprint(c.Writer, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"alpha\"},\"finish_reason\":null}]}\n\n")
				return &ForwardResult{Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 5}}, io.ErrUnexpectedEOF
			}
			_, _ = fmt.Fprint(c.Writer, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"beta\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
			return &ForwardResult{Usage: ClaudeUsage{InputTokens: 110, OutputTokens: 5}}, nil
		})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, 210, result.Usage.InputTokens)
	require.Contains(t, recorder.Body.String(), "alpha")
	require.Contains(t, recorder.Body.String(), "beta")
}

func TestAccountContinuationForwardFailureCannotRestartAccountFailover(t *testing.T) {
	c, recorder := accountContinuationTestContext()
	body := []byte(`{"model":"long-text","stream":true,"messages":[{"role":"user","content":"write"}]}`)
	calls := 0
	result, err := runForwardWithAccountContinuation(c.Request.Context(), c, accountContinuationTestAccount(true), body, AccountContinuationChat, "long-text", true,
		func(ctx context.Context, request []byte) (*ForwardResult, error) {
			calls++
			if calls == 2 {
				return nil, &UpstreamFailoverError{StatusCode: 503}
			}
			_, _ = fmt.Fprint(c.Writer, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"alpha\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n")
			return &ForwardResult{Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 5}}, nil
		})
	require.Error(t, err)
	var failover *UpstreamFailoverError
	require.False(t, errors.As(err, &failover), "must settle previous delivered output instead of retrying another account")
	require.NotNil(t, result)
	require.Contains(t, recorder.Body.String(), "alpha")
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Positive(t, *result.DeliveredOutputTokens)
	require.NotNil(t, result.Outcome)
	require.Equal(t, DisconnectSourceUpstream, result.Outcome.DisconnectSource)
}

func TestAccountContinuationForwardCancellationNeverStartsNextRound(t *testing.T) {
	c, _ := accountContinuationTestContext()
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	body := []byte(`{"model":"long-text","stream":true,"messages":[{"role":"user","content":"write"}]}`)
	calls := 0
	result, err := runForwardWithAccountContinuation(ctx, c, accountContinuationTestAccount(true), body, AccountContinuationChat, "long-text", true,
		func(_ context.Context, _ []byte) (*ForwardResult, error) {
			calls++
			_, _ = fmt.Fprint(c.Writer, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"alpha\"},\"finish_reason\":\"length\"}]}\n\ndata: [DONE]\n\n")
			cancel()
			return &ForwardResult{Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 5},
				Outcome: &ResponseOutcome{CollectorVersion: ResponseOutcomeCollectorVersion, HasText: true, StreamCompleted: true}}, nil
		})
	require.Error(t, err)
	require.Equal(t, 1, calls)
	require.True(t, result.ClientDisconnect)
	require.Len(t, result.ContinuationResults, 1, "interrupted first rounds need the same delivered-only settlement as later rounds")
	require.True(t, result.ContinuationResults[0].ClientDisconnect)
	require.False(t, result.ContinuationResults[0].Outcome.StreamCompleted, "a canceled leaf cannot be priced as a completed response")
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Equal(t, *result.DeliveredOutputTokens, recordedOutputTokens(result), "do not charge undelivered provider output")
}

func TestAccountContinuationBufferedFailureDoesNotInventDelivery(t *testing.T) {
	c, rec := accountContinuationTestContext()
	body := []byte(`{"model":"long-text","messages":[{"role":"user","content":"write"}]}`)
	calls := 0
	result, err := runForwardWithAccountContinuation(c.Request.Context(), c, accountContinuationTestAccount(true), body, AccountContinuationChat, "long-text", false,
		func(_ context.Context, _ []byte) (*ForwardResult, error) {
			calls++
			if calls == 2 {
				return nil, &UpstreamFailoverError{StatusCode: 503}
			}
			c.JSON(200, map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "alpha"}, "finish_reason": "length"}}})
			return &ForwardResult{Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 5}}, nil
		})
	require.Error(t, err)
	require.NotNil(t, result)
	require.Zero(t, rec.Body.Len(), "no successful buffered response exists after the failed continuation")
	require.False(t, rec.Flushed)
	require.False(t, c.Writer.Written(), "leave headers writable so the handler can return an error instead of an empty HTTP 200")
	require.NotNil(t, result.DeliveredOutputTokens)
	require.Zero(t, *result.DeliveredOutputTokens, "buffering is not downstream delivery")
	require.Len(t, result.ContinuationResults, 1, "retain upstream evidence even when no buffered response was delivered")
}

func TestAccountContinuationForwardUsesIndependentOutcomeCollectors(t *testing.T) {
	c, _ := accountContinuationTestContext()
	body := []byte(`{"model":"long-text","stream":true,"messages":[{"role":"user","content":"write"}]}`)
	var collectors []*ResponseOutcomeCollector
	_, err := runForwardWithAccountContinuation(c.Request.Context(), c, accountContinuationTestAccount(true), body, AccountContinuationChat, "long-text", true,
		func(ctx context.Context, _ []byte) (*ForwardResult, error) {
			collector, ok := ResponseOutcomeCollectorFromContext(ctx)
			require.True(t, ok)
			collectors = append(collectors, collector)
			reason := "length"
			if len(collectors) == 2 {
				reason = "stop"
			}
			_, _ = fmt.Fprintf(c.Writer, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"alpha\"},\"finish_reason\":%q}]}\n\ndata: [DONE]\n\n", reason)
			return &ForwardResult{Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 5}}, nil
		})
	require.NoError(t, err)
	require.Len(t, collectors, 2)
	require.NotSame(t, collectors[0], collectors[1])
}

func accountContinuationTestContext() (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	return c, recorder
}

func accountContinuationTestAccount(enabled bool) *Account {
	return &Account{ID: 51, Platform: PlatformGemini, Extra: map[string]any{
		"anti_truncation": map[string]any{"version": 1, "rules": []any{
			map[string]any{"model": "long-text", "enabled": enabled, "max_rounds": 1},
		}},
	}}
}

func TestAccountContinuationForwardChatCombinesRounds(t *testing.T) {
	c, recorder := accountContinuationTestContext()
	body := []byte(`{"model":"long-text","stream":true,"messages":[{"role":"user","content":"write"}]}`)
	calls := 0
	result, err := runForwardWithAccountContinuation(c.Request.Context(), c, accountContinuationTestAccount(true), body, AccountContinuationChat, "long-text", true,
		func(ctx context.Context, request []byte) (*ForwardResult, error) {
			calls++
			_, deadline := ctx.Deadline()
			require.False(t, deadline, "continuation must not cancel normal long output")
			text, reason := "alpha", "length"
			if calls == 2 {
				require.Equal(t, "alpha", gjson.GetBytes(request, "messages.1.content").String())
				text, reason = "beta", "stop"
			}
			c.Writer.Header().Set("Content-Type", "text/event-stream")
			_, _ = fmt.Fprintf(c.Writer, "data: {\"id\":\"round-%d\",\"choices\":[{\"index\":0,\"delta\":{\"content\":%q},\"finish_reason\":null}]}\n\n", calls, text)
			_, _ = fmt.Fprintf(c.Writer, "data: {\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":%q}]}\n\ndata: [DONE]\n\n", reason)
			c.Writer.Flush()
			return &ForwardResult{RequestID: fmt.Sprintf("upstream-%d", calls), Model: "long-text", Stream: true, Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 5}}, nil
		})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, 200, result.Usage.InputTokens)
	require.Equal(t, 10, result.Usage.OutputTokens)
	require.Len(t, result.ContinuationResults, 2)
	require.Equal(t, 1, result.AntiTruncationRounds)
	require.Equal(t, 1, strings.Count(recorder.Body.String(), "[DONE]"))
	require.Contains(t, recorder.Body.String(), "alpha")
	require.Contains(t, recorder.Body.String(), "beta")
}

func TestAccountContinuationForwardGeminiBufferedCombinesBeforeJSON(t *testing.T) {
	c, recorder := accountContinuationTestContext()
	body := []byte(`{"contents":[{"role":"user","parts":[{"text":"write"}]}]}`)
	calls := 0
	result, err := runForwardWithAccountContinuation(c.Request.Context(), c, accountContinuationTestAccount(true), body, AccountContinuationGemini, "long-text", false,
		func(_ context.Context, request []byte) (*ForwardResult, error) {
			calls++
			text, reason := "alpha", "MAX_TOKENS"
			if calls == 2 {
				require.Equal(t, "alpha", gjson.GetBytes(request, "contents.1.parts.0.text").String())
				text, reason = "beta", "STOP"
			}
			c.JSON(http.StatusOK, gin.H{"candidates": []any{gin.H{"content": gin.H{"role": "model", "parts": []any{gin.H{"text": text}}}, "finishReason": reason}}, "usageMetadata": gin.H{"promptTokenCount": 100, "candidatesTokenCount": 5}})
			return &ForwardResult{Model: "long-text", RequestID: fmt.Sprintf("round-%d", calls), Usage: ClaudeUsage{InputTokens: 100, OutputTokens: 5}}, nil
		})
	require.NoError(t, err)
	require.Equal(t, 2, calls)
	require.Equal(t, 200, result.Usage.InputTokens)
	require.True(t, gjson.ValidBytes(recorder.Body.Bytes()), "buffered rounds must produce one JSON response")
	require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "candidates.0.content.parts.0.text").String(), "alpha")
	require.Contains(t, gjson.GetBytes(recorder.Body.Bytes(), "candidates.0.content.parts.0.text").String(), "beta")
}
