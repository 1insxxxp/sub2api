//go:build unit

package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/tiktoken-go/tokenizer"
)

func TestRawChatEvidenceMergesPartialUsageFields(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`
	wire := "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}],\"usage\":{\"prompt_tokens\":21916,\"prompt_tokens_details\":{\"cached_tokens\":700}}}\n\n" +
		"data: {\"choices\":[],\"usage\":{\"completion_tokens\":4055}}\n\n" + "data: [DONE]\n\n"
	result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(wire)), false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 21916, result.Usage.InputTokens, "later output-only usage must retain earlier prompt")
	require.Equal(t, 700, result.Usage.CacheReadInputTokens)
	require.Equal(t, 4055, result.Usage.OutputTokens)
	require.NotNil(t, result.Outcome)
	require.True(t, result.Outcome.StreamCompleted)
	require.True(t, result.Outcome.HasText)
}

func TestRawChatEvidenceCapturesCanceledDelivery(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`
	result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n")), true)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.ClientDisconnect)
	require.NotNil(t, result.Outcome)
	require.False(t, result.Outcome.HasText, "failed downstream writes are not delivered text")
	require.False(t, result.Outcome.StreamCompleted)
	require.Equal(t, DisconnectSourceClient, result.Outcome.DisconnectSource)
}

func TestRawChatEvidenceUpstreamTimeoutIsNotClientCancel(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`
	reader := &openAIChatStreamReadErrorCloser{payload: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"), err: context.DeadlineExceeded}
	result, c, err := rawChatEvidenceForward(t, body, reader, false)
	require.Error(t, err)
	require.Nil(t, c.Request.Context().Err())
	require.NotNil(t, result)
	require.False(t, result.ClientDisconnect)
	require.NotNil(t, result.Outcome)
	require.Equal(t, DisconnectSourceUpstream, result.Outcome.DisconnectSource)
	require.Equal(t, UpstreamErrorTimeout, result.Outcome.UpstreamErrorKind)
}

func TestRawChatEvidenceRoleOnlyDoesNotClaimOutput(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`
	result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader("data: {\"choices\":[{\"delta\":{\"role\":\"assistant\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")), false)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Nil(t, result.FirstTokenMs)
	require.NotNil(t, result.Outcome)
	require.False(t, result.Outcome.HasEffectiveOutput())
}

func rawChatEvidenceForward(t *testing.T, body string, reader io.ReadCloser, disconnected bool) (*OpenAIForwardResult, *gin.Context, error) {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(body))
	if disconnected {
		c.Writer = &openAIRawStreamDisconnectedWriter{ResponseWriter: c.Writer}
	}
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"text/event-stream"}, "X-Oneapi-Request-Id": []string{"provider-exact"}}, Body: reader}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	result, err := svc.forwardAsRawChatCompletions(context.Background(), c, rawChatCompletionsTestAccount(), []byte(body), "")
	return result, c, err
}

func TestRawChatEvidenceStrictPresenceAndZero(t *testing.T) {
	for _, tc := range []struct {
		raw   string
		known bool
		value int
	}{
		{`{}`, false, 0}, {`{"prompt_tokens":null}`, false, 0}, {`{"prompt_tokens":"9"}`, false, 0}, {`{"prompt_tokens":-1}`, false, 0}, {`{"prompt_tokens":1.2}`, false, 0}, {`{"prompt_tokens":2147483648}`, false, 0}, {`{"input_tokens":0,"prompt_tokens":77}`, true, 0},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			r := &OpenAIChatUsageRound{}
			r.mergeUsage(gjson.Parse(tc.raw))
			require.Equal(t, tc.known, r.InputReported)
			require.Equal(t, tc.value, r.Usage.InputTokens)
		})
	}
	r := &OpenAIChatUsageRound{}
	r.mergeUsage(gjson.Parse(`{"prompt_tokens":100,"completion_tokens":20,"prompt_tokens_details":{"cached_tokens":10}}`))
	r.mergeUsage(gjson.Parse(`{"completion_tokens":0}`))
	require.True(t, r.OutputReported)
	require.Equal(t, 0, r.Usage.OutputTokens)
	require.Equal(t, 100, r.Usage.InputTokens)
	require.Equal(t, 10, r.Usage.CacheReadInputTokens)
}

func TestRawChatEvidenceInputOmitsOpaqueMedia(t *testing.T) {
	prepare := func(n int) *GeminiInputUsageFallback {
		body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":[{"type":"text","text":"Describe this image"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("abc", n) + `"}}]}]}`
		req, _ := http.NewRequest(http.MethodPost, "https://provider.example/v1/chat/completions", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer test")
		return prepareOpenAIChatInputUsageFallback(req, 7, "gemini-4-argon")
	}
	small, large := prepare(1), prepare(50000)
	require.True(t, small.Known)
	require.Equal(t, small.Tokens, large.Tokens)
	require.Empty(t, small.CalibrationKey)
	require.Empty(t, large.CalibrationKey)
	require.Nil(t, large.CountRequest)
}

func TestRawChatEvidenceToolAndReasoningOutput(t *testing.T) {
	c := &openAIChatSemanticOutput{}
	c.observe(gjson.Parse(`{"choices":[{"index":0,"delta":{"content":"Hi ","reasoning_content":"Think","tool_calls":[{"index":0,"function":{"name":"lookup","arguments":"{\"x\":"}}]}}]}`))
	c.observe(gjson.Parse(`{"choices":[{"index":0,"delta":{"content":"there","tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`))
	c.observe(gjson.Parse(`{"choices":[{"index":0,"message":{"content":"Hi there","reasoning_content":"Think","tool_calls":[{"function":{"name":"lookup","arguments":"{\"x\":1}"}}]}}]}`))
	codec, err := tokenizer.Get(tokenizer.O200kBase)
	require.NoError(t, err)
	want := 0
	for _, v := range []string{"Hi there", "Think", "lookup", `{"x":1}`} {
		n, err := codec.Count(v)
		require.NoError(t, err)
		want += n
	}
	require.Equal(t, want, c.count())
}

func TestRawChatEvidenceNonStreamingExplicitZeroAndFailedWrite(t *testing.T) {
	for _, disconnected := range []bool{false, true} {
		t.Run(strconv.FormatBool(disconnected), func(t *testing.T) {
			result, _, err := rawChatEvidenceForward(t, `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}]}`, io.NopCloser(strings.NewReader(`{"model":"gemini-4-argon","choices":[{"message":{"content":"answer"},"finish_reason":"stop"}],"usage":{"prompt_tokens":0,"completion_tokens":0}}`)), disconnected)
			require.NoError(t, err)
			require.NotNil(t, result.ChatUsage)
			require.Len(t, result.ChatUsage.Rounds, 1)
			r := result.ChatUsage.Rounds[0]
			require.True(t, r.InputReported)
			require.True(t, r.OutputReported)
			require.Zero(t, result.Usage.InputTokens)
			require.Zero(t, result.Usage.OutputTokens)
			require.Equal(t, !disconnected, result.Outcome.HasText)
			require.Equal(t, !disconnected, result.Outcome.StreamCompleted)
			require.Equal(t, disconnected, result.ClientDisconnect)
		})
	}
}

func TestRawChatEvidenceIdentityUsesActualRequestAndRejectsRedirect(t *testing.T) {
	ctx, capture := newOpenAIChatUsageCapture(context.Background())
	_ = ctx
	req, _ := http.NewRequest(http.MethodPost, "https://actual.example/v1/chat/completions", strings.NewReader(`{"model":"exact-model","messages":[{"role":"user","content":"hi"}]}`))
	req.Header.Set("Authorization", "Bearer actual-key")
	capture.begin(req, &Account{ID: 77})
	capture.response(&http.Response{StatusCode: 200, Header: http.Header{"X-Oneapi-Request-Id": []string{"actual-id"}}, Request: req})
	require.Equal(t, GeminiUsageLedgerIdentity(req.URL.String(), "actual-key"), capture.current.IdentityHash)
	require.Equal(t, "exact-model", capture.current.UpstreamModel)
	require.Equal(t, "actual-id", capture.current.UpstreamRequestID)
	require.True(t, capture.current.InputFallback.Known)
	foreign := req.Clone(req.Context())
	foreign.URL, _ = url.Parse("https://foreign.example/v1/chat/completions")
	capture.response(&http.Response{StatusCode: 200, Header: http.Header{"X-Oneapi-Request-Id": []string{"foreign-id"}}, Request: foreign})
	require.Empty(t, capture.current.IdentityHash)
	require.Empty(t, capture.current.UpstreamRequestID)
	require.Empty(t, capture.current.InputFallback.CalibrationKey)
}

func TestRawChatEvidenceRoundIsolation(t *testing.T) {
	_, capture := newOpenAIChatUsageCapture(context.Background())
	request := func(model string) *http.Request {
		r, _ := http.NewRequest(http.MethodPost, "https://actual.example/v1/chat/completions", strings.NewReader(`{"model":"`+model+`","messages":[{"role":"user","content":"hi"}]}`))
		r.Header.Set("Authorization", "Bearer key")
		return r
	}
	for i := 0; i < 2; i++ {
		req := request("m")
		capture.begin(req, &Account{ID: 77})
		capture.response(&http.Response{StatusCode: 200, Request: req, Header: http.Header{"X-Oneapi-Request-Id": []string{fmt.Sprint(i)}}})
		capture.observe(fmt.Sprintf(`{"usage":{"prompt_tokens":%d,"completion_tokens":%d},"choices":[{"delta":{"content":"answer"}}]}`, 100+i, 20+i))
	}
	evidence := capture.snapshot()
	require.Len(t, evidence.Rounds, 2)
	require.Equal(t, 100, evidence.Rounds[0].Usage.InputTokens)
	require.Equal(t, 101, evidence.Rounds[1].Usage.InputTokens)
	require.NotZero(t, evidence.Rounds[0].ReceivedOutputTokens)
	require.NotZero(t, evidence.Rounds[1].ReceivedOutputTokens)
}

func TestRawChatEvidenceReadFailureRetainsAcceptedAttempt(t *testing.T) {
	result, _, err := rawChatEvidenceForward(t, `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}]}`, &openAIChatStreamReadErrorCloser{err: io.ErrUnexpectedEOF}, false)
	require.Error(t, err)
	require.NotNil(t, result)
	require.NotNil(t, result.Outcome)
	require.Equal(t, DisconnectSourceUpstream, result.Outcome.DisconnectSource)
	require.False(t, result.Outcome.StreamCompleted)
	require.Len(t, result.ChatUsage.Rounds, 1)
	require.True(t, result.ChatUsage.Rounds[0].UpstreamAccepted)
	require.False(t, result.ChatUsage.Rounds[0].InputReported)
}

func TestRawChatEvidenceEmbeddedErrorIsNotSuccess(t *testing.T) {
	for _, stream := range []bool{false, true} {
		t.Run(strconv.FormatBool(stream), func(t *testing.T) {
			body := fmt.Sprintf(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}],"stream":%t}`, stream)
			payload := `{"error":{"message":"upstream rejected request","type":"api_error"}}`
			if stream {
				payload = "data: " + payload + "\n\ndata: [DONE]\n\n"
			}
			result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(payload)), false)
			require.Error(t, err)
			if result != nil {
				require.NotNil(t, result.Outcome)
				require.False(t, result.Outcome.StreamCompleted)
				require.Equal(t, DisconnectSourceUpstream, result.Outcome.DisconnectSource)
			}
		})
	}
}

func TestRawChatEvidenceCanceledBeforeResponseRetainsActualAttempt(t *testing.T) {
	for _, canceled := range []bool{true, false} {
		t.Run(strconv.FormatBool(canceled), func(t *testing.T) {
			body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if canceled {
				cancel()
			}
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
			upstream := &httpUpstreamRecorder{err: context.Canceled}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			result, err := svc.forwardAsRawChatCompletions(ctx, c, rawChatCompletionsTestAccount(), body, "")
			require.Error(t, err)
			if !canceled {
				require.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			require.True(t, result.ClientDisconnect)
			require.Len(t, result.ChatUsage.Rounds, 1)
			require.False(t, result.ChatUsage.Rounds[0].UpstreamAccepted)
			require.False(t, result.Outcome.HasEffectiveOutput())
			require.False(t, result.Outcome.StreamCompleted)
			require.Equal(t, DisconnectSourceClient, result.Outcome.DisconnectSource)
		})
	}
}

func TestRawChatEvidenceReasoningVariantIsDeliveredAndSeparated(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`
	wire := "data: {\"choices\":[{\"delta\":{\"reasoning\":\"think carefully\"}}]}\n\ndata: [DONE]\n\n"
	result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(wire)), false)
	require.NoError(t, err)
	require.True(t, result.Outcome.HasReasoning)
	require.False(t, result.Outcome.HasText)
	require.NotNil(t, result.FirstTokenMs)
	round := result.ChatUsage.Rounds[0]
	require.NotZero(t, round.ReceivedReasoningTokens)
	require.Equal(t, round.ReceivedReasoningTokens, round.ReceivedOutputTokens)
}

func TestRawChatEvidenceOnlyCompleteTrailingUsageIsTerminal(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`
	text := "data: {\"choices\":[{\"delta\":{\"content\":\"answer\"}}]}\n\n"
	prompt := "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100}}\n\n"
	completion := "data: {\"choices\":[],\"usage\":{\"completion_tokens\":20}}\n\n"
	both := "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20}}\n\n"
	for _, tc := range []struct {
		name, wire string
		completed  bool
	}{
		{"partial before output", prompt + text, false},
		{"complete before output", both + text, false},
		{"partial after output", text + prompt, false},
		{"complete trailing", text + both, true},
		{"partial merged trailing", prompt + text + completion, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(tc.wire)), false)
			require.NotNil(t, result)
			if tc.completed {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
			}
			require.Equal(t, tc.completed, result.Outcome.StreamCompleted)
		})
	}
}

func TestRawChatEvidenceCalibrationUsesSemanticLength(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}]}`
	makeFallback := func(raw string) *GeminiInputUsageFallback {
		req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1/chat/completions", strings.NewReader(raw))
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer test-key")
		return prepareOpenAIChatInputUsageFallback(req, 42, "gemini-4-argon")
	}
	plain := makeFallback(body)
	formatted := makeFallback(strings.Repeat(" ", 8192) + body)
	require.True(t, plain.Known)
	require.Equal(t, plain.Tokens, formatted.Tokens)
	require.NotEmpty(t, plain.CalibrationKey)
	require.Equal(t, plain.CalibrationKey, formatted.CalibrationKey)
}

func TestRawChatEvidenceReadFailureRetainsAlreadyReceivedUsage(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hi"}]}`
	reader := &openAIChatStreamReadErrorCloser{payload: []byte(`{"choices":[{"message":{"content":"answer"}}],"usage":{"prompt_tokens":21916,"completion_tokens":4055}}`), err: io.ErrUnexpectedEOF}
	result, _, err := rawChatEvidenceForward(t, body, reader, false)
	require.Error(t, err)
	require.NotNil(t, result)
	require.Equal(t, 21916, result.Usage.InputTokens)
	require.Equal(t, 4055, result.Usage.OutputTokens)
	require.False(t, result.Outcome.HasText)
	require.False(t, result.Outcome.StreamCompleted)
	require.Equal(t, DisconnectSourceUpstream, result.Outcome.DisconnectSource)
}

func TestRawChatEvidenceContinuationRoundIsolationAndFailure(t *testing.T) {
	for _, failed := range []bool{false, true} {
		t.Run(strconv.FormatBool(failed), func(t *testing.T) {
			body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
			first := "data: {\"choices\":[{\"delta\":{\"content\":\"partial\"},\"finish_reason\":\"length\"}],\"usage\":{\"prompt_tokens\":100,\"completion_tokens\":20}}\n\ndata: [DONE]\n\n"
			second := "data: {\"choices\":[{\"delta\":{\"content\":\"rest\"},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":150,\"completion_tokens\":30}}\n\ndata: [DONE]\n\n"
			status := 200
			if failed {
				status = 502
				second = `{"error":{"message":"unavailable"}}`
			}
			upstream := &httpUpstreamRecorder{responses: []*http.Response{
				{StatusCode: 200, Header: http.Header{"X-Oneapi-Request-Id": []string{"round-1"}}, Body: io.NopCloser(strings.NewReader(first))},
				{StatusCode: status, Header: http.Header{"X-Oneapi-Request-Id": []string{"round-2"}}, Body: io.NopCloser(strings.NewReader(second))},
			}}
			svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
			ctx := WithRawChatAntiTruncationPolicy(context.Background(), domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 1, MaxDurationSeconds: 30})
			result, err := svc.forwardAsRawChatCompletions(ctx, c, rawChatCompletionsTestAccount(), body, "")
			require.NotNil(t, result)
			require.Len(t, result.ChatUsage.Rounds, 2)
			require.Equal(t, "round-1", result.ChatUsage.Rounds[0].UpstreamRequestID)
			require.Equal(t, "round-2", result.ChatUsage.Rounds[1].UpstreamRequestID)
			require.Equal(t, 100, result.ChatUsage.Rounds[0].Usage.InputTokens)
			require.True(t, result.Outcome.HasText)
			if failed {
				require.Error(t, err)
				require.False(t, result.ChatUsage.Rounds[1].UpstreamAccepted)
				require.False(t, result.Outcome.StreamCompleted)
				require.Equal(t, DisconnectSourceUpstream, result.Outcome.DisconnectSource)
			} else {
				require.NoError(t, err)
				require.Equal(t, 150, result.ChatUsage.Rounds[1].Usage.InputTokens)
				require.Equal(t, 250, result.Usage.InputTokens)
				require.True(t, result.Outcome.StreamCompleted)
			}
		})
	}
}

func TestRawChatEvidenceIdentityUsesActualBearerCredential(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://upstream.example/v1/chat/completions", strings.NewReader(`{"model":"m","messages":[]}`))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer selected-api-key")
	req.Header.Set("x-goog-api-key", "unrelated-header-override")
	_, capture := newOpenAIChatUsageCapture(context.Background())
	capture.begin(req, &Account{ID: 42})
	require.Equal(t, GeminiUsageLedgerIdentity(req.URL.String(), "selected-api-key"), capture.current.IdentityHash)
}

func TestRawChatEvidenceSyncCanceledContextCannotClaimDeliveredAnswer(t *testing.T) {
	body := []byte(`{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}]}`)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	upstream := &httpUpstreamRecorder{resp: &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"answer"}}],"usage":{"prompt_tokens":20,"completion_tokens":5}}`))}}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream}
	result, _ := svc.forwardAsRawChatCompletions(ctx, c, rawChatCompletionsTestAccount(), body, "")
	require.NotNil(t, result)
	require.True(t, result.ClientDisconnect)
	require.False(t, result.Outcome.StreamCompleted)
	require.False(t, result.Outcome.HasText)
	require.Equal(t, 20, result.Usage.InputTokens)
	require.Equal(t, 5, result.Usage.OutputTokens)
}

type rawChatBridgeThenCancelUpstream struct {
	cancel context.CancelFunc
	calls  int
}

func (u *rawChatBridgeThenCancelUpstream) Do(_ *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	u.calls++
	if u.calls == 1 {
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"model":"grok-build-0.1","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"A diagram."}]}],"usage":{"input_tokens":11,"output_tokens":7}}`))}, nil
	}
	u.cancel()
	return nil, context.Canceled
}
func (u *rawChatBridgeThenCancelUpstream) DoWithTLS(r *http.Request, p string, a int64, n int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(r, p, a, n)
}
func TestRawChatEvidenceHeaderCancelPreservesMeasuredBridgeUsage(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	body := []byte(`{"model":"grok-composer-2.5-fast","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,QUJD"}}]}]}`)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body)).WithContext(ctx)
	account := healthyGrokOAuthGatewayTestAccount(55, "access-token")
	repo := &grokQuotaAccountRepo{mockAccountRepoForPlatform: &mockAccountRepoForPlatform{accountsByID: map[int64]*Account{55: account}}}
	upstream := &rawChatBridgeThenCancelUpstream{cancel: cancel}
	svc := &OpenAIGatewayService{cfg: rawChatCompletionsTestConfig(), httpUpstream: upstream, grokTokenProvider: NewGrokTokenProvider(repo, nil), accountRepo: repo}
	result, err := svc.forwardAsRawChatCompletions(ctx, c, account, body, "")
	require.ErrorIs(t, err, context.Canceled)
	require.Equal(t, 2, upstream.calls)
	require.NotNil(t, result)
	require.True(t, result.ClientDisconnect)
	require.Equal(t, 11, result.ChatUsage.ExtraUsage.InputTokens)
	require.Equal(t, 7, result.ChatUsage.ExtraUsage.OutputTokens)
	svc.FinalizeOpenAIChatUsage(context.Background(), &OpenAIRecordUsageInput{Account: account, Result: result})
	require.Equal(t, 11, result.Usage.InputTokens)
	require.Equal(t, 7, result.Usage.OutputTokens)
}

func TestRawChatEvidenceErrorPreservesPositiveMeasuredAcceptance(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}]}`
	for _, tc := range []struct {
		name, usage string
		accepted    bool
		output      int
	}{
		{"measured completion", `{"completion_tokens":7}`, true, 7},
		{"measured reasoning", `{"completion_tokens_details":{"reasoning_tokens":3}}`, true, 3},
		{"explicit all zero", `{"prompt_tokens":0,"completion_tokens":0,"completion_tokens_details":{"reasoning_tokens":0}}`, false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload := `{"error":{"message":"generation interrupted"},"usage":` + tc.usage + `}`
			result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(payload)), false)
			require.Error(t, err)
			require.NotNil(t, result)
			require.Len(t, result.ChatUsage.Rounds, 1)
			round := result.ChatUsage.Rounds[0]
			require.Equal(t, tc.accepted, round.UpstreamAccepted)
			(&OpenAIGatewayService{}).FinalizeOpenAIChatUsage(context.Background(), &OpenAIRecordUsageInput{Result: result})
			if tc.accepted {
				require.Positive(t, result.Usage.InputTokens, "positive measured generation must retain missing input fallback")
				require.Equal(t, round.InputFallback.Tokens, result.Usage.InputTokens)
				require.Equal(t, UsageTokenSourceEstimated, result.InputTokensSource)
			} else {
				require.Zero(t, result.Usage.InputTokens)
			}
			require.Equal(t, tc.output, result.Usage.OutputTokens)
			require.False(t, result.Outcome.StreamCompleted)
		})
	}
}

func TestRawChatEvidenceMeasuredErrorBeforeStreamOutputStillFailsOver(t *testing.T) {
	body := `{"model":"gemini-4-argon","messages":[{"role":"user","content":"hello"}],"stream":true}`
	wire := "data: {\"error\":{\"message\":\"interrupted\"},\"usage\":{\"completion_tokens\":7}}\n\n"
	result, _, err := rawChatEvidenceForward(t, body, io.NopCloser(strings.NewReader(wire)), false)
	var failover *UpstreamFailoverError
	require.ErrorAs(t, err, &failover)
	require.Nil(t, result, "failed transparent retry must not create another billed request")
}
