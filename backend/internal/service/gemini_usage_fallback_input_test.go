//go:build unit

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

const geminiFallbackInputURL = "https://count.example.com/proxy/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse"
const geminiFallbackInputBody = `{"systemInstruction":{"parts":[{"text":"Be precise."}]},"contents":[{"role":"user","parts":[{"text":"Explain streaming."}]}]}`

func newGeminiFallbackInputRequest(t *testing.T, endpoint, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	req.Header.Set("x-goog-api-key", "same-test-key")
	return req
}

func TestGeminiFallbackInput_SnapshotsFinalSemanticBodyWithoutConsumingInference(t *testing.T) {
	req := newGeminiFallbackInputRequest(t, geminiFallbackInputURL, geminiFallbackInputBody)
	fallback := prepareGeminiInputUsageFallback(req, 5957, geminiInputUsageRegressionModel)
	require.NotNil(t, fallback)
	require.True(t, fallback.Known)
	require.Positive(t, fallback.Tokens)
	require.NotEmpty(t, fallback.CalibrationKey)
	require.NotContains(t, fallback.CalibrationKey, "same-test-key")
	require.NotContains(t, fallback.CalibrationKey, "Be precise")
	require.NotNil(t, fallback.CountRequest)
	require.Equal(t, "https://count.example.com/proxy/v1beta/models/gemini-2.5-flash:countTokens", fallback.CountRequest.URL)
	var counted map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(fallback.CountRequest.Body, &counted))
	var nested map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(counted["generateContentRequest"], &nested))
	require.JSONEq(t, `"models/gemini-2.5-flash"`, string(nested["model"]))
	require.JSONEq(t, `{"parts":[{"text":"Be precise."}]}`, string(nested["systemInstruction"]))
	raw, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, geminiFallbackInputBody, string(raw), "count preparation cannot consume the inference body")
}

func TestGeminiFallbackInput_ToolsCallsResultsAndSchemaAffectEstimate(t *testing.T) {
	base := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, geminiFallbackInputBody), 5957, geminiInputUsageRegressionModel)
	for _, field := range []string{
		`"tools":[{"functionDeclarations":[{"name":"lookup","description":"` + strings.Repeat("Detailed search tool. ", 80) + `","parameters":{"type":"OBJECT","properties":{"query":{"type":"STRING"}}}}]}]`,
		`"generationConfig":{"responseSchema":{"type":"OBJECT","description":"` + strings.Repeat("Structured response schema. ", 80) + `"}}`,
		`"contents":[{"role":"model","parts":[{"functionCall":{"name":"lookup","args":{"query":"` + strings.Repeat("structured argument ", 80) + `"}}}]}]`,
		`"contents":[{"role":"user","parts":[{"functionResponse":{"name":"lookup","response":{"answer":"` + strings.Repeat("structured result ", 80) + `"}}}]}]`,
	} {
		fallback := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, "{"+field+"}"), 5957, geminiInputUsageRegressionModel)
		require.True(t, fallback.Known)
		require.Greater(t, fallback.Tokens, base.Tokens*5, "structured semantic input cannot disappear from fallback")
	}
}

func TestGeminiFallbackInput_OpaqueMediaAndSignaturesAreNotTextTokens(t *testing.T) {
	base := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, geminiFallbackInputBody), 5957, geminiInputUsageRegressionModel)
	media := strings.Replace(geminiFallbackInputBody, `"Explain streaming."`, `"Explain streaming.","thoughtSignature":"`+strings.Repeat("opaque", 1000)+`","inlineData":{"mimeType":"image/png","data":"`+strings.Repeat("AAAA", 10000)+`"},"fileData":{"fileUri":"https://files.example/`+strings.Repeat("asset", 1000)+`","mimeType":"image/png"}`, 1)
	fallback := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, media), 5957, geminiInputUsageRegressionModel)
	require.True(t, fallback.Known)
	require.Equal(t, base.Tokens, fallback.Tokens)
	require.Empty(t, fallback.CalibrationKey, "multimodal input is not a text calibration sample")
	wrapped := `{"model":"gemini-2.5-flash","project":"opaque-project","request":` + geminiFallbackInputBody + `}`
	codeAssist := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, "https://cloudcode-pa.googleapis.com/v1internal:streamGenerateContent?alt=sse", wrapped), 5957, geminiInputUsageRegressionModel)
	require.True(t, codeAssist.Known)
	require.Equal(t, base.Tokens, codeAssist.Tokens)
	require.Nil(t, codeAssist.CountRequest, "Code Assist must not be silently changed to AI Studio")
}

func TestGeminiFallbackInput_CalibrationKeySeparatesIdentityModelAndShape(t *testing.T) {
	base := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, geminiFallbackInputBody), 5957, geminiInputUsageRegressionModel)
	for _, tc := range []struct {
		accountID                  int64
		model, endpoint, body, key string
	}{
		{5958, geminiInputUsageRegressionModel, geminiFallbackInputURL, geminiFallbackInputBody, "same-test-key"},
		{5957, "gemini-4-argon", geminiFallbackInputURL, geminiFallbackInputBody, "same-test-key"},
		{5957, geminiInputUsageRegressionModel, strings.Replace(geminiFallbackInputURL, "count.example.com", "different.example.com", 1), geminiFallbackInputBody, "same-test-key"},
		{5957, geminiInputUsageRegressionModel, geminiFallbackInputURL, geminiFallbackInputBody, "rotated-key"},
		{5957, geminiInputUsageRegressionModel, geminiFallbackInputURL, `{"contents":[{"parts":[{"text":"请准确解释网络请求流式响应的工作原理及错误恢复机制。"}]}]}`, "same-test-key"},
		{5957, geminiInputUsageRegressionModel, geminiFallbackInputURL, `{"tools":[{"functionDeclarations":[{"name":"lookup"}]}],"contents":[{"parts":[{"text":"Explain streaming."}]}]}`, "same-test-key"},
	} {
		req := newGeminiFallbackInputRequest(t, tc.endpoint, tc.body)
		req.Header.Set("x-goog-api-key", tc.key)
		fallback := prepareGeminiInputUsageFallback(req, tc.accountID, tc.model)
		require.NotEmpty(t, fallback.CalibrationKey)
		require.NotEqual(t, base.CalibrationKey, fallback.CalibrationKey)
	}
	cached := strings.Replace(geminiFallbackInputBody, `"contents":`, `"cachedContent":"cachedContents/provider-cache","contents":`, 1)
	fallback := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, cached), 5957, geminiInputUsageRegressionModel)
	require.Empty(t, fallback.CalibrationKey)
	require.True(t, fallback.LocalTokensExcludeCachedContent)
	require.False(t, base.LocalTokensExcludeCachedContent)
}

func TestGeminiFallbackInput_VertexCountUsesItsOwnProtocol(t *testing.T) {
	endpoint := "https://us-central1-aiplatform.googleapis.com/v1/projects/p/locations/us-central1/publishers/google/models/gemini-2.5-flash:streamGenerateContent?alt=sse"
	fallback := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, endpoint, geminiFallbackInputBody), 5957, geminiInputUsageRegressionModel)
	require.NotNil(t, fallback.CountRequest)
	require.Equal(t, strings.TrimSuffix(endpoint, ":streamGenerateContent?alt=sse")+":countTokens", fallback.CountRequest.URL)
	require.JSONEq(t, geminiFallbackInputBody, string(fallback.CountRequest.Body))
	cached := strings.Replace(geminiFallbackInputBody, `"contents":`, `"cachedContent":"projects/p/cachedContents/c","contents":`, 1)
	fallback = prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, endpoint, cached), 5957, geminiInputUsageRegressionModel)
	require.Nil(t, fallback.CountRequest, "unsupported Vertex cached-content counting must not drop the cache reference")
}

func TestGeminiFallbackInput_RejectsMissingOversizeAndInvalidBody(t *testing.T) {
	for _, body := range []string{`{broken}`, `[]`, strings.Repeat(" ", 4<<20) + `{}`} {
		fallback := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, body), 5957, geminiInputUsageRegressionModel)
		require.NotNil(t, fallback)
		require.False(t, fallback.Known)
		require.Nil(t, fallback.CountRequest)
	}
	req := newGeminiFallbackInputRequest(t, geminiFallbackInputURL, geminiFallbackInputBody)
	req.GetBody = nil
	require.False(t, prepareGeminiInputUsageFallback(req, 5957, geminiInputUsageRegressionModel).Known)
}

func newGeminiCountFallbackFixture(t *testing.T, response string) (*GatewayService, *RecordUsageInput, *httpUpstreamRecorder) {
	t.Helper()
	fallback := prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, geminiFallbackInputURL, geminiFallbackInputBody), 5957, geminiInputUsageRegressionModel)
	upstream := &httpUpstreamRecorder{resp: geminiUsageRecoveryResponse(200, response)}
	input := &RecordUsageInput{Account: &Account{ID: 5957, Type: AccountTypeAPIKey, Platform: PlatformGemini, Credentials: map[string]any{"base_url": "https://count.example.com/proxy", "api_key": "same-test-key"}}, Result: &ForwardResult{GeminiUsageFallback: &GeminiUsageFallback{Input: fallback}}}
	return &GatewayService{cfg: &config.Config{}, httpUpstream: upstream}, input, upstream
}

func TestGeminiFallbackInput_RemoteCountIsBoundedSingleRequestAndClearsSnapshot(t *testing.T) {
	for _, measured := range []int{0, 21916} {
		raw, _ := json.Marshal(map[string]int{"totalTokens": measured})
		svc, input, upstream := newGeminiCountFallbackFixture(t, string(raw))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		tokens, ok, err := svc.CountGeminiInputFallback(ctx, input)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, measured, tokens)
		require.Nil(t, input.Result.GeminiUsageFallback.Input.CountRequest)
		require.Len(t, upstream.requests, 1)
		require.Equal(t, http.MethodPost, upstream.lastReq.Method)
		require.True(t, strings.HasSuffix(upstream.lastReq.URL.String(), ":countTokens"))
		require.True(t, HTTPUpstreamRedirectsDisabled(upstream.lastReq.Context()))
		require.Equal(t, "same-test-key", upstream.lastReq.Header.Get("x-goog-api-key"))
		_, hasDeadline := upstream.lastReq.Context().Deadline()
		require.True(t, hasDeadline)
		require.Zero(t, input.Result.Usage.InputTokens, "caller decides estimation policy; this helper does not bill or mark actual usage")
	}
}

func TestGeminiFallbackInput_RemoteDefaultOriginAndIdentityGuards(t *testing.T) {
	for _, change := range []string{"key", "origin", "account", "oauth", "service_account", "missing_proxy"} {
		t.Run(change, func(t *testing.T) {
			svc, input, upstream := newGeminiCountFallbackFixture(t, `{"totalTokens":21916}`)
			switch change {
			case "key":
				input.Account.Credentials["api_key"] = "rotated"
			case "origin":
				input.Account.Credentials["base_url"] = "https://elsewhere.example"
			case "account":
				input.Account.ID = 999
			case "oauth":
				input.Account.Type = AccountTypeOAuth
			case "service_account":
				input.Account.Type = AccountTypeServiceAccount
			case "missing_proxy":
				id := int64(9)
				input.Account.ProxyID = &id
			}
			_, ok, _ := svc.CountGeminiInputFallback(context.Background(), input)
			require.False(t, ok)
			require.Empty(t, upstream.requests)
			require.Nil(t, input.Result.GeminiUsageFallback.Input.CountRequest)
		})
	}
	svc, input, upstream := newGeminiCountFallbackFixture(t, `{"totalTokens":21916}`)
	input.Account.Credentials["base_url"] = ""
	input.Result.GeminiUsageFallback.Input = prepareGeminiInputUsageFallback(newGeminiFallbackInputRequest(t, strings.Replace(geminiFallbackInputURL, "https://count.example.com/proxy", "https://generativelanguage.googleapis.com", 1), geminiFallbackInputBody), 5957, geminiInputUsageRegressionModel)
	_, ok, err := svc.CountGeminiInputFallback(context.Background(), input)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, upstream.requests, 1)
}

func TestGeminiFallbackInput_RemoteRejectsNonMeasurementsWithoutRetry(t *testing.T) {
	for _, raw := range []string{`{}`, `{"totalTokens":null}`, `{"totalTokens":"12"}`, `{"totalTokens":-1}`, `{"totalTokens":1.5}`, `{"totalTokens":2147483648}`, `{"totalTokens":1,"totalTokens":2}`, `{"error":{"message":"sensitive"}}`, `{"totalTokens":10} trailing-sensitive`} {
		svc, input, upstream := newGeminiCountFallbackFixture(t, raw)
		_, ok, err := svc.CountGeminiInputFallback(context.Background(), input)
		require.False(t, ok)
		require.Error(t, err)
		require.NotContains(t, err.Error(), "sensitive")
		require.Len(t, upstream.requests, 1)
		require.Nil(t, input.Result.GeminiUsageFallback.Input.CountRequest)
	}
	svc, input, upstream := newGeminiCountFallbackFixture(t, `{"totalTokens":21916}`)
	upstream.resp.StatusCode = http.StatusFound
	_, ok, err := svc.CountGeminiInputFallback(context.Background(), input)
	require.Error(t, err)
	require.False(t, ok)
	require.Len(t, upstream.requests, 1)
}

func TestGeminiFallbackInput_RealHTTPOnlyCountsAndDoesNotFollowRedirect(t *testing.T) {
	var countCalls, redirectedCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			redirectedCalls.Add(1)
			return
		}
		countCalls.Add(1)
		require.Equal(t, "/v1beta/models/gemini-2.5-flash:countTokens", r.URL.Path)
		require.Equal(t, "Bearer same-test-key", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("x-goog-api-key"))
		if countCalls.Load() == 1 {
			_, _ = io.WriteString(w, `{"totalTokens":21916}`)
			return
		}
		http.Redirect(w, r, "/redirected", http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	svc := &GatewayService{cfg: cfg, httpUpstream: &geminiRealHTTPUsageUpstream{client: server.Client(), target: target}}
	for i := 0; i < 2; i++ {
		req := newGeminiFallbackInputRequest(t, server.URL+"/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", geminiFallbackInputBody)
		req.Header.Del("x-goog-api-key")
		req.Header.Set("Authorization", "Bearer same-test-key")
		input := &RecordUsageInput{Account: &Account{ID: 5957, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": server.URL, "api_key": "same-test-key"}}, Result: &ForwardResult{GeminiUsageFallback: &GeminiUsageFallback{Input: prepareGeminiInputUsageFallback(req, 5957, geminiInputUsageRegressionModel)}}}
		tokens, ok, err := svc.CountGeminiInputFallback(context.Background(), input)
		if i == 0 {
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, 21916, tokens)
		} else {
			require.ErrorIs(t, err, ErrGeminiInputCountUnavailable)
			require.False(t, ok)
		}
		require.Nil(t, input.Result.GeminiUsageFallback.Input.CountRequest)
	}
	require.EqualValues(t, 2, countCalls.Load())
	require.Zero(t, redirectedCalls.Load())
}

func TestGeminiFallbackInput_RealHTTPDeadlineCancelsSlowCount(t *testing.T) {
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	target, err := url.Parse(server.URL)
	require.NoError(t, err)
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.AllowInsecureHTTP = true
	svc := &GatewayService{cfg: cfg, httpUpstream: &geminiRealHTTPUsageUpstream{client: server.Client(), target: target}}
	req := newGeminiFallbackInputRequest(t, server.URL+"/v1beta/models/gemini-2.5-flash:generateContent", geminiFallbackInputBody)
	input := &RecordUsageInput{Account: &Account{ID: 5957, Platform: PlatformGemini, Type: AccountTypeAPIKey, Credentials: map[string]any{"base_url": server.URL, "api_key": "same-test-key"}}, Result: &ForwardResult{GeminiUsageFallback: &GeminiUsageFallback{Input: prepareGeminiInputUsageFallback(req, 5957, geminiInputUsageRegressionModel)}}}
	started := time.Now()
	_, ok, err := svc.CountGeminiInputFallback(context.Background(), input)
	require.ErrorIs(t, err, ErrGeminiInputCountUnavailable)
	require.False(t, ok)
	require.Less(t, time.Since(started), 4*time.Second)
	require.Nil(t, input.Result.GeminiUsageFallback.Input.CountRequest)
	select {
	case <-canceled:
	case <-time.After(time.Second):
		t.Fatal("count request was not canceled at its deadline")
	}
}
