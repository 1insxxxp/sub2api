//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestRawChatOpsCancellationBeforeResponse(t *testing.T) {
	body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"private prompt"}],"stream":true}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: &httpUpstreamRecorder{err: context.Canceled}}
	result, err := svc.forwardAsRawChatCompletions(ctx, c, rawChatCompletionsTestAccount(), body, "")
	require.Error(t, err)
	require.NotNil(t, result)
	events := geminiUpstreamAttemptEvents(c)
	require.Len(t, events, 1)
	require.True(t, events[0].IsClientCancelDiagnostic())
	require.Positive(t, events[0].Attempt)
	require.Equal(t, "unreported", events[0].UsageState)
	require.Zero(t, events[0].UpstreamStatusCode)
	require.Empty(t, GetOpsStreamErrors(c), "cancellation must not become an SLA failure")
}

func TestRawChatOpsFailedWriteSeparatesReceivedAndDelivered(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"private prompt"}],"stream":true}`
	wire := "data: {\"choices\":[{\"delta\":{\"content\":\"private answer\"}}]}\n\n"
	result, c, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(wire)), true)
	require.NoError(t, err)
	require.True(t, result.ClientDisconnect)
	require.NoError(t, c.Request.Context().Err(), "write failure can precede context cancellation")
	events := geminiUpstreamAttemptEvents(c)
	require.Len(t, events, 1)
	require.True(t, events[0].IsClientCancelDiagnostic())
	require.Equal(t, "provider-exact", events[0].UpstreamRequestID)
	require.NotNil(t, events[0].DownstreamDelivery)
	require.False(t, events[0].DownstreamDelivery.AnswerTextDelivered)
	raw, err := json.Marshal(events[0])
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(raw, &fields))
	require.NotNil(t, fields["estimated_received_output_tokens"])
	require.Positive(t, fields["estimated_received_output_tokens"])
	require.NotContains(t, string(raw), "private answer")
	require.NotContains(t, string(raw), "private prompt")
	require.Empty(t, GetOpsStreamErrors(c))
}

func TestRawChatOpsVisibleStreamFailureIsRecordedOnce(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}],"stream":true}`
	wire := "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}],\"usage\":{\"prompt_tokens\":0}}\n\n" +
		"data: {\"error\":{\"message\":\"sensitive provider detail\"},\"usage\":{\"completion_tokens\":0}}\n\n"
	result, c, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(wire)), false)
	require.Error(t, err)
	require.NotNil(t, result)
	require.Equal(t, http.StatusOK, c.Writer.Status())
	events := geminiUpstreamAttemptEvents(c)
	require.Len(t, events, 1)
	require.False(t, events[0].ClientCanceled)
	require.Equal(t, "inference", events[0].Stage)
	require.Equal(t, "observed", events[0].UsageState)
	require.Equal(t, uint8(3), events[0].UsageFields)
	require.Equal(t, http.StatusOK, events[0].UpstreamStatusCode, "do not invent an upstream HTTP 502")
	require.NotNil(t, events[0].DownstreamDelivery)
	require.True(t, events[0].DownstreamDelivery.AnswerTextDelivered)
	markers := GetOpsStreamErrors(c)
	require.Len(t, markers, 1)
	require.True(t, markers[0].CountTowardsSLA)
	require.Equal(t, http.StatusBadGateway, markers[0].IntendedStatus)
}

func TestRawChatOpsSyncErrorAndHTTPFailoverRetainOneEvent(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusBadGateway} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}]}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: status, Header: http.Header{"X-Oneapi-Request-Id": []string{"direct-id"}}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"provider failed"}}`))}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
			require.Error(t, err)
			if status == http.StatusBadGateway {
				require.Nil(t, result)
			}
			events := geminiUpstreamAttemptEvents(c)
			require.Len(t, events, 1)
			require.Equal(t, "inference", events[0].Stage)
			require.Positive(t, events[0].Attempt)
			require.Equal(t, status, events[0].UpstreamStatusCode)
			require.Equal(t, "direct-id", events[0].UpstreamRequestID)
			require.Empty(t, GetOpsStreamErrors(c), "ordinary HTTP errors use the existing error/failover logger")
		})
	}
}

func TestRawChatOpsSuccessfulResponseAddsNoDiagnostic(t *testing.T) {
	result, c, err := rawChatEvidenceForward(t, `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}],"stream":true}`, io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\ndata: [DONE]\n\n")), false)
	require.NoError(t, err)
	require.True(t, result.Outcome.StreamCompleted)
	require.Empty(t, geminiUpstreamAttemptEvents(c))
	require.Empty(t, GetOpsStreamErrors(c))
}

type rawChatOpsNativeFailureForwarder struct {
	canceled bool
	cancel   context.CancelFunc
	calls    int
}

func (f *rawChatOpsNativeFailureForwarder) ForwardAsChatCompletions(_ context.Context, c *gin.Context, account *Account, _ []byte) (*ForwardResult, error) {
	f.calls++
	attempt := BeginGeminiUpstreamAttempt(c, account, "gemini-native-selected", false)
	if f.canceled {
		f.cancel()
		attempt.EndClientCanceled()
		return nil, context.Canceled
	}
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusInternalServerError, Header: http.Header{"X-Oneapi-Request-Id": []string{"native-request"}}})
	err := &UpstreamFailoverError{StatusCode: http.StatusInternalServerError}
	attempt.End(err)
	return nil, err
}

func TestRawChatOpsNativeHandoffDoesNotOverwriteFinalFailure(t *testing.T) {
	for _, canceled := range []bool{false, true} {
		name := "native_500"
		if canceled {
			name = "native_canceled"
		}
		t.Run(name, func(t *testing.T) {
			body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}]}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
			fallback := &rawChatOpsNativeFailureForwarder{canceled: canceled, cancel: cancel}
			upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"contents is required"}}`))}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, geminiCompatForwarder: fallback}
			result, err := svc.forwardAsRawChatCompletions(ctx, c, rawChatCompletionsTestAccount(), body, "")
			require.Error(t, err)
			require.Nil(t, result)
			require.Equal(t, 1, fallback.calls)
			events := geminiUpstreamAttemptEvents(c)
			require.Len(t, events, 1, "the abandoned raw HTTP 400 must not become the final native failure")
			require.Equal(t, "gemini-native-selected", events[0].Model)
			require.Positive(t, events[0].Attempt)
			if canceled {
				require.True(t, events[0].IsClientCancelDiagnostic())
			} else {
				require.Equal(t, http.StatusInternalServerError, events[0].UpstreamStatusCode)
				require.Equal(t, "native-request", events[0].UpstreamRequestID)
			}
			require.Empty(t, GetOpsStreamErrors(c))
			capture := openAIChatCaptureFromContext(c.Request.Context())
			require.NotNil(t, capture)
			require.Nil(t, capture.opsContext, "a protocol handoff must release the pooled Gin reference")
		})
	}
}

func TestRawChatOpsMissingNativeForwarderKeepsOriginalDiagnostic(t *testing.T) {
	body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}]}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: http.StatusBadRequest, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"error":{"message":"contents is required"}}`))}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), body, "")
	require.ErrorContains(t, err, "not configured")
	require.Nil(t, result)
	events := geminiUpstreamAttemptEvents(c)
	require.Len(t, events, 1)
	require.Equal(t, http.StatusBadRequest, events[0].UpstreamStatusCode)
	require.Equal(t, "gemini-4-argon", events[0].Model)
}
