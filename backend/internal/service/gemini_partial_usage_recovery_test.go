//go:build unit

package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGeminiPartialUsageRecovery_StreamFailureRetainsObservedResult(t *testing.T) {
	const partialUsage = `data: {"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":10,"candidatesTokenCount":5}}` + "\n\n"
	for _, path := range []string{"chat", "messages", "native"} {
		for _, tc := range []struct {
			name          string
			sse           string
			readErr       error
			input, output int
		}{
			{"read_error_with_usage", partialUsage, io.ErrUnexpectedEOF, 10, 5},
			{"trailing_null_preserves_usage", partialUsage + "data: {\"usageMetadata\":null}\n\n", io.ErrUnexpectedEOF, 10, 5},
			{"trailing_empty_preserves_usage", partialUsage + "data: {\"usageMetadata\":{}}\n\n", io.ErrUnexpectedEOF, 10, 5},
			{"read_error_without_usage", geminiAbortTextChunk, io.ErrUnexpectedEOF, 0, 0},
			{"eof_with_usage", partialUsage, io.EOF, 10, 5},
			{"eof_without_usage", geminiAbortTextChunk, io.EOF, 0, 0},
		} {
			t.Run(path+"/"+tc.name, func(t *testing.T) {
				svc, c, rec := newGeminiUsageRecoveryService(t, tc.sse, tc.readErr)
				result, err := callGeminiTraversalPath(t, path, svc, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", true)
				require.Error(t, err)
				var failover *UpstreamFailoverError
				require.False(t, errors.As(err, &failover), "delivered content must never be replayed on another account")
				require.NotNil(t, result)
				require.Equal(t, tc.input, result.Usage.InputTokens)
				require.Equal(t, tc.output, result.Usage.OutputTokens)
				require.Equal(t, "usage-recovery-request", result.RequestID)
				require.NotNil(t, result.Outcome)
				require.True(t, result.Outcome.HasText)
				require.False(t, result.Outcome.StreamCompleted)
				require.Equal(t, DisconnectSourceUpstream, result.Outcome.DisconnectSource)
				require.Contains(t, rec.Body.String(), "hello")
				_, marked := c.Get(OpsStreamErrorKey)
				require.True(t, marked, "an in-band failure on HTTP 200 must be visible to Ops")
				events := upstreamErrorEventsFromContext(t, c)
				require.Len(t, events, 1)
				require.Equal(t, int64(811), events[0].AccountID)
				require.Equal(t, 1, events[0].Attempt)
				require.Equal(t, "usage-recovery-request", events[0].UpstreamRequestID)
				state := "unreported"
				if tc.input > 0 {
					state = "observed"
				}
				require.Equal(t, state, events[0].UsageState)
				require.Equal(t, int64(tc.input), events[0].InputTokens)
				require.Equal(t, int64(tc.output), events[0].OutputTokens)
			})
		}
	}
}

func TestGeminiPartialUsageRecovery_MetadataOnlyAbortPreservesUsageWithoutEffectiveOutput(t *testing.T) {
	const metadataOnly = `data: {"usageMetadata":{"promptTokenCount":123,"candidatesTokenCount":0}}` + "\n\n"
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			svc, c, _ := newGeminiAbortingStreamService(t, "/v1/"+path, nil, metadataOnly, true)
			result, _ := callGeminiTraversalPath(t, path, svc, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", true)
			require.NotNil(t, result)
			require.Equal(t, 123, result.Usage.InputTokens, "observed usage remains available for reconciliation")
			require.True(t, result.ClientDisconnect)
			require.NotNil(t, result.Outcome)
			require.False(t, result.Outcome.HasEffectiveOutput(), "metadata and heartbeat are never effective output")
			require.False(t, result.Outcome.StreamCompleted)
			require.Equal(t, DisconnectSourceClient, result.Outcome.DisconnectSource)
			events := upstreamErrorEventsFromContext(t, c)
			require.Len(t, events, 1)
			require.Equal(t, "client_cancel", events[0].Kind)
			require.True(t, events[0].ClientCanceled)
			require.Equal(t, "observed", events[0].UsageState)
			require.Equal(t, int64(123), events[0].InputTokens)
		})
	}
}

func TestGeminiPartialUsageRecovery_UncommittedChatFailureRemainsUnsettledFailover(t *testing.T) {
	const metadataOnly = `data: {"usageMetadata":{"promptTokenCount":123,"candidatesTokenCount":0}}` + "\n\n"
	svc, c, rec := newGeminiUsageRecoveryService(t, metadataOnly, io.EOF)
	result, err := callGeminiTraversalPath(t, "chat", svc, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", true)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Nil(t, result, "attempt diagnostics must not become a final usage settlement")
	require.Empty(t, rec.Body.String())
	events := upstreamErrorEventsFromContext(t, c)
	require.Len(t, events, 1)
	require.Equal(t, "failover", events[0].Kind)
	require.Equal(t, "observed", events[0].UsageState)
	require.Equal(t, int64(123), events[0].InputTokens)
}

func TestGeminiPartialUsageRecovery_CompletedRequestDoesNotAddDiagnosticEvent(t *testing.T) {
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			svc, c, _ := newGeminiUsageRecoveryService(t, geminiAbortTextChunk+geminiAbortFinalChunk, io.EOF)
			result, err := callGeminiTraversalPath(t, path, svc, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", true)
			require.NoError(t, err)
			require.True(t, result.Outcome.StreamCompleted)
			require.Equal(t, 10, result.Usage.InputTokens)
			require.Empty(t, upstreamErrorEventsFromContext(t, c))
			require.Empty(t, GetOpsStreamErrors(c))
		})
	}
}

type geminiUsageRecoveryDoErrorUpstream struct {
	HTTPUpstream
	waitForDeadline bool
}

func (u *geminiUsageRecoveryDoErrorUpstream) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	if u.waitForDeadline {
		<-req.Context().Done()
		return nil, req.Context().Err()
	}
	return nil, io.ErrUnexpectedEOF
}

func TestGeminiPartialUsageRecovery_DoFailureHasExactlyOneAttemptSummary(t *testing.T) {
	for _, path := range []string{"chat", "messages", "native"} {
		for _, timeout := range []bool{false, true} {
			name := "transport_error"
			if timeout {
				name = "first_output_timeout"
			}
			t.Run(path+"/"+name, func(t *testing.T) {
				svc, c, rec := newGeminiUsageRecoveryService(t, "", io.EOF)
				svc.httpUpstream = &geminiUsageRecoveryDoErrorUpstream{waitForDeadline: timeout}
				if timeout {
					svc.settingService = &SettingService{settingRepo: &modelFirstOutputSettingRepo{value: `{"enabled":true,"default":{"enabled":true,"target_seconds":1,"switch_seconds":1,"hard_cap_seconds":5}}`}}
				}
				result, err := callGeminiTraversalPath(t, path, svc, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", true)
				var failover *UpstreamFailoverError
				require.ErrorAs(t, err, &failover)
				require.Nil(t, result)
				require.Empty(t, rec.Body.String())
				events := upstreamErrorEventsFromContext(t, c)
				require.Len(t, events, 1, "a failed Do must not add an undecorated duplicate event")
				require.Equal(t, 1, events[0].Attempt)
				require.Equal(t, int64(811), events[0].AccountID)
				require.Equal(t, "unreported", events[0].UsageState)
				require.Empty(t, events[0].UpstreamRequestID, "no response means no invented provider request ID")
				if timeout {
					require.Equal(t, http.StatusGatewayTimeout, events[0].UpstreamStatusCode)
					require.Equal(t, "first_output_timeout", events[0].Reason)
				}
			})
		}
	}
}

func TestGeminiPartialUsageRecovery_UsageFieldsMergeAcrossChunks(t *testing.T) {
	const chunks = `data: {"candidates":[{"content":{"parts":[{"text":"hello"}]}}],"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":20,"candidatesTokenCount":5,"thoughtsTokenCount":2}}` + "\n\n" +
		`data: {"usageMetadata":{"promptTokenCount":120}}` + "\n\n" +
		`data: {"usageMetadata":{"candidatesTokenCount":6}}` + "\n\n"
	for _, path := range []string{"chat", "messages", "native"} {
		t.Run(path, func(t *testing.T) {
			svc, c, _ := newGeminiUsageRecoveryService(t, chunks, io.ErrUnexpectedEOF)
			result, err := callGeminiTraversalPath(t, path, svc, c, geminiVertexAPIKeyTestAccount(""), "gemini-3-flash-preview", true)
			require.Error(t, err)
			require.NotNil(t, result)
			require.Equal(t, 100, result.Usage.InputTokens)
			require.Equal(t, 20, result.Usage.CacheReadInputTokens)
			require.Equal(t, 8, result.Usage.OutputTokens)
		})
	}
}

func TestGeminiPartialUsageRecovery_ThoughtsAreNotDeliveredAnswerEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, parts string
		wantText    bool
	}{
		{"thought_only", `{"text":"private reasoning","thought":true}`, false},
		{"thought_and_answer", `{"text":"private reasoning","thought":true},{"text":"hello"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			collector := NewResponseOutcomeCollector(http.StatusOK, http.StatusOK)
			collector.ObserveGeminiSSEData(`{"candidates":[{"content":{"parts":[` + tc.parts + `]}}]}`)
			outcome := collector.Snapshot()
			require.True(t, outcome.HasReasoning)
			require.Equal(t, tc.wantText, outcome.HasText)
		})
	}
}

func newGeminiUsageRecoveryService(t *testing.T, sse string, readErr error) (*GeminiMessagesCompatService, *gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/test", nil).WithContext(context.Background())
	svc, upstream := newGeminiVertexAPIKeyTestService()
	upstream.response = &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}, "X-Goog-Request-Id": []string{"usage-recovery-request"}},
		Body:       &geminiAbortingStreamBody{data: bytes.NewReader([]byte(sse)), abortErr: readErr},
	}
	return svc, c, rec
}
