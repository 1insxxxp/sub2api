//go:build unit

package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const inputCountingTestModel = "[量]gemini-4-argon"
const inputCountingTestBody = `{"contents":[{"role":"user","parts":[{"text":"请仅回复OK，不要解释。"}]}],"generationConfig":{"temperature":0,"maxOutputTokens":256}}`

func inputCountingTestAccount() *Account {
	return &Account{ID: 6021, Platform: PlatformGemini, Type: AccountTypeAPIKey, Extra: map[string]any{
		"gemini_input_counting": map[string]any{"mode": "vertex_like_discount_v1", "models": []any{inputCountingTestModel}},
	}}
}

func inputCountingTestRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, "https://fixture.example/v1beta/models/%5B%E9%87%8F%5Dgemini-4-argon:generateContent", bytes.NewReader([]byte(body)))
	require.NoError(t, err)
	req.Header.Set("x-goog-api-key", "fixture-secret")
	return req
}

func TestGeminiInputCounting_ExactOptInAndScalarEvidence(t *testing.T) {
	a := inputCountingTestAccount()
	req := inputCountingTestRequest(t, inputCountingTestBody)
	e := prepareGeminiInputCountingPolicy(req, a, inputCountingTestModel)
	require.NotNil(t, e)
	require.Equal(t, int64(6021), e.AccountID)
	require.Equal(t, "vertex_like_discount_v1", e.Policy)
	require.Equal(t, 39, e.CandidateInputTokens)
	raw, err := io.ReadAll(req.Body)
	require.NoError(t, err)
	require.Equal(t, inputCountingTestBody, string(raw))
	scalar, err := json.Marshal(e)
	require.NoError(t, err)
	require.NotContains(t, string(scalar), "fixture-secret")
	require.NotContains(t, string(scalar), "fixture.example")
	require.NotContains(t, string(scalar), "不要解释")
	for _, model := range []string{"gemini-4-argon", "[量]gemini-4-argon-high", "[量]gemini-4-argon "} {
		require.Nil(t, prepareGeminiInputCountingPolicy(req, a, model))
	}
	for _, mutate := range []func(*Account){func(a *Account) { a.Extra = nil }, func(a *Account) { a.Type = AccountTypeOAuth }, func(a *Account) { a.Platform = PlatformOpenAI }, func(a *Account) { a.ID = 0 }} {
		a := inputCountingTestAccount()
		mutate(a)
		require.Nil(t, prepareGeminiInputCountingPolicy(req, a, inputCountingTestModel))
	}
}

func TestGeminiInputCounting_ConfigProjection(t *testing.T) {
	valid := map[string]any{"mode": "vertex_like_discount_v1", "models": []any{inputCountingTestModel}, "url": "https://private.example", "key": "secret"}
	p := GeminiInputCountingConfigProjection(valid)
	require.Equal(t, map[string]any{"mode": "vertex_like_discount_v1", "models": []string{inputCountingTestModel}}, p)
	for _, bad := range []any{nil, "bad", map[string]any{}, map[string]any{"mode": "future", "models": []string{inputCountingTestModel}}, map[string]any{"mode": "vertex_like_discount_v1", "models": []any{}}, map[string]any{"mode": "vertex_like_discount_v1", "models": []any{inputCountingTestModel, 42}}, map[string]any{"mode": "vertex_like_discount_v1", "models": []string{" "}}} {
		require.Nil(t, GeminiInputCountingConfigProjection(bad))
	}
}

func TestGeminiInputCounting_StrictTextEligibility(t *testing.T) {
	base := `{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`
	bad := []string{`{broken}`, `[]`, `{"contents":[]}`, `{"contents":null}`, `{"contents":[{"parts":[{"text":12}]}]}`, `{"contents":[{"parts":[{}]}]}`, `{"contents":[{"role":"unknown","parts":[{"text":"hello"}]}]}`, `{"contents":[{"parts":[{"text":"hello","text":"different"}]}]}`, strings.Repeat(" ", 4<<20) + base}
	for _, field := range []string{`"tools":[]`, `"toolConfig":{}`, `"cachedContent":"cachedContents/x"`, `"cached_content":"x"`, `"generationConfig":{"responseSchema":{}}`, `"generationConfig":{"responseJsonSchema":{}}`, `"generationConfig":{"responseModalities":["IMAGE"]}`, `"unknown_future_field":true`} {
		bad = append(bad, strings.TrimSuffix(base, "}")+","+field+"}")
	}
	for _, part := range []string{`"inlineData":{}`, `"fileData":{}`, `"thoughtSignature":"opaque"`, `"thought":true`, `"functionCall":{}`, `"functionResponse":{}`, `"futureMedia":true`} {
		bad = append(bad, strings.Replace(base, `"text":"hello"`, `"text":"hello",`+part, 1))
	}
	for _, body := range bad {
		require.Nil(t, prepareGeminiInputCountingPolicy(inputCountingTestRequest(t, body), inputCountingTestAccount(), inputCountingTestModel), body[:min(len(body), 160)])
	}
	req := inputCountingTestRequest(t, base)
	req.GetBody = nil
	require.Nil(t, prepareGeminiInputCountingPolicy(req, inputCountingTestAccount(), inputCountingTestModel))
	req = inputCountingTestRequest(t, base)
	req.Method = http.MethodGet
	require.Nil(t, prepareGeminiInputCountingPolicy(req, inputCountingTestAccount(), inputCountingTestModel))
}

func TestGeminiInputCounting_ProfileAddsWrapperAndMessages(t *testing.T) {
	body := `{"systemInstruction":{"parts":[{"text":"请仅回复OK，不要解释。"}]},"contents":[{"role":"user","parts":[{"text":"请仅回复OK，不要解释。"}]},{"role":"model","parts":[{"text":"请仅回复OK，不要解释。"}]},{"role":"user","parts":[{"text":"请仅回复OK，不要解释。"}]}]}`
	e := prepareGeminiInputCountingPolicy(inputCountingTestRequest(t, body), inputCountingTestAccount(), inputCountingTestModel)
	require.NotNil(t, e)
	require.Equal(t, 8*4+31+2*2, e.CandidateInputTokens)
}

func TestGeminiInputCounting_PythonProfileEquivalence(t *testing.T) {
	raw, err := os.ReadFile("../pkg/geminitokens/testdata/python_counts.json")
	require.NoError(t, err)
	var fixtures []struct {
		Name         string          `json:"name"`
		ContentCount int             `json:"content_count"`
		Raw          int             `json:"raw"`
		Request      json.RawMessage `json:"request"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixtures))
	for _, fixture := range fixtures {
		t.Run(fixture.Name, func(t *testing.T) {
			evidence := prepareGeminiInputCountingPolicy(inputCountingTestRequest(t, string(fixture.Request)), inputCountingTestAccount(), inputCountingTestModel)
			require.NotNil(t, evidence)
			require.Equal(t, fixture.Raw+31+2*(fixture.ContentCount-1), evidence.CandidateInputTokens)
		})
	}
}

func TestGeminiInputCounting_CustomerProjectionOnlyDiscountsMeasuredUncached(t *testing.T) {
	for _, tc := range []struct {
		name           string
		raw, candidate int
		source         UsageTokenSource
		fields         uint8
		cache          int
		want           int
		discount       bool
	}{
		{"discount", 100, 60, UsageTokenSourceUpstream, 1 | 8, 0, 60, true},
		{"no increase", 30, 60, UsageTokenSourceUpstream, 1 | 8, 0, 30, false},
		{"equal", 60, 60, UsageTokenSourceUpstream, 1 | 8, 0, 60, false},
		{"explicit zero", 0, 60, UsageTokenSourceUpstream, 1 | 8, 0, 0, false},
		{"cache unknown", 100, 60, UsageTokenSourceUpstream, 1, 0, 100, false},
		{"cache nonzero", 100, 60, UsageTokenSourceUpstream, 1 | 8, 5, 100, false},
		{"estimated input", 100, 60, UsageTokenSourceEstimated, 1 | 8, 0, 100, false},
		{"unknown input", 100, 60, UsageTokenSourceUnknown, 1 | 8, 0, 100, false},
		{"invalid candidate", 100, -1, UsageTokenSourceUpstream, 1 | 8, 0, 100, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &ForwardResult{Usage: ClaudeUsage{InputTokens: tc.raw, OutputTokens: 5, CacheReadInputTokens: tc.cache}, InputTokensSource: tc.source, GeminiUsageFallback: &GeminiUsageFallback{UsageFields: tc.fields}, GeminiInputCounting: &GeminiInputCountingEvidence{AccountID: 6021, Policy: "vertex_like_discount_v1", CandidateInputTokens: tc.candidate}}
			got, discount := geminiCustomerInputTokens(r)
			require.Equal(t, tc.want, got)
			require.Equal(t, tc.discount, discount)
			require.Equal(t, tc.raw, r.Usage.InputTokens)
			require.Equal(t, 5, r.Usage.OutputTokens)
		})
	}
	got, discount := geminiCustomerInputTokens(nil)
	require.Zero(t, got)
	require.False(t, discount)
	r := &ForwardResult{Usage: ClaudeUsage{InputTokens: 100}, InputTokensSource: UsageTokenSourceUpstream}
	got, discount = geminiCustomerInputTokens(r)
	require.Equal(t, 100, got)
	require.False(t, discount)
}

func TestGeminiInputCounting_RejectsMismatchedRequestModel(t *testing.T) {
	for _, path := range []string{"/v1beta/models/gemini-4-argon:generateContent", "/v1beta/models/[量]gemini-4-argon:countTokens"} {
		req := inputCountingTestRequest(t, inputCountingTestBody)
		req.URL.Path = path
		require.Nil(t, prepareGeminiInputCountingPolicy(req, inputCountingTestAccount(), inputCountingTestModel))
	}
	req := inputCountingTestRequest(t, inputCountingTestBody)
	req.URL = nil
	require.Nil(t, prepareGeminiInputCountingPolicy(req, inputCountingTestAccount(), inputCountingTestModel))
}

func TestGeminiInputCounting_RejectsGeneratedMedia(t *testing.T) {
	for _, mutate := range []func(*ForwardResult){
		func(r *ForwardResult) { r.Usage.ImageOutputTokens = 1 },
		func(r *ForwardResult) { r.ImageCount = 1 },
		func(r *ForwardResult) { r.ImageEndpoint = true },
		func(r *ForwardResult) { r.AudioUsage = &AudioUsage{Mode: "tts"} },
		func(r *ForwardResult) { r.Outcome = &ResponseOutcome{HasMedia: true} },
	} {
		r := &ForwardResult{Usage: ClaudeUsage{InputTokens: 100}, InputTokensSource: UsageTokenSourceUpstream, GeminiUsageFallback: &GeminiUsageFallback{UsageFields: 1 | 8}, GeminiInputCounting: &GeminiInputCountingEvidence{AccountID: 6021, Policy: GeminiInputCountingVertexLikeDiscountV1, CandidateInputTokens: 60}}
		mutate(r)
		got, discount := geminiCustomerInputTokens(r)
		require.Equal(t, 100, got)
		require.False(t, discount)
	}
}
