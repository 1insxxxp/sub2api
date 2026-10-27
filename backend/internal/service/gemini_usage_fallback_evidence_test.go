//go:build unit

package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func geminiFallbackCount(t *testing.T, text string) int {
	t.Helper()
	codec, err := openAIInputTokensCodecForModel("gemini-test")
	require.NoError(t, err)
	n, err := codec.Count(text)
	require.NoError(t, err)
	return n
}

func TestGeminiFallbackEvidenceJoinsReceivedChunksAndSeparatesThoughts(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header)})
	attempt.ObserveUsage([]byte(`{"candidates":[{"index":0,"content":{"parts":[{"text":"hel"},{"thought":true,"text":"reason"}]}}]}`))
	attempt.ObserveUsage([]byte(`{"response":{"candidates":[{"index":0,"content":{"parts":[{"text":"lo"},{"thought":true,"text":"ing"}]}}]}}`))
	result := &ForwardResult{Model: "gemini-test", Stream: true}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.NotNil(t, result.GeminiUsageFallback)
	require.Equal(t, geminiFallbackCount(t, "hello"), result.GeminiUsageFallback.CandidateTokens)
	require.Equal(t, geminiFallbackCount(t, "reasoning"), result.GeminiUsageFallback.ThoughtTokens)
	require.Zero(t, result.GeminiUsageFallback.UsageFields, "received semantics are estimates, not provider measurements")
	require.True(t, result.GeminiUsageFallback.UpstreamAccepted)
	// Nil-result cancellation recovery must keep the exact same current attempt.
	recovered := GeminiResultFromLastAttempt(c, 701, "gemini-test", true, 0)
	require.Equal(t, result.GeminiUsageFallback, recovered.GeminiUsageFallback)
}

func TestGeminiFallbackEvidenceCountsNativeToolsButNotOpaquePayload(t *testing.T) {
	tool := map[string]any{"functionCall": map[string]any{"name": "lookup_weather", "args": map[string]any{"city": "Shanghai", "days": 3}}}
	measure := func(extra bool) *GeminiUsageFallback {
		c := newGeminiAttemptTestContext(t)
		attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
		part := map[string]any{}
		for key, value := range tool {
			part[key] = value
		}
		if extra {
			part["thoughtSignature"] = strings.Repeat("opaque-signature", 1000)
			part["inlineData"] = map[string]any{"mimeType": "image/png", "data": strings.Repeat("YWJj", 1000)}
		}
		raw, err := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{part}}}}})
		require.NoError(t, err)
		attempt.ObserveUsage(raw)
		result := &ForwardResult{Model: "gemini-test"}
		finishGeminiUpstreamAttempt(attempt, result, nil)
		require.NotNil(t, result.GeminiUsageFallback)
		return result.GeminiUsageFallback
	}
	plain, withOpaque := measure(false), measure(true)
	require.Positive(t, plain.CandidateTokens)
	require.Equal(t, plain.CandidateTokens, withOpaque.CandidateTokens)
	require.Zero(t, plain.ThoughtTokens)
	require.True(t, plain.UpstreamAccepted, "actual generated function call proves processing even without a captured header")
}

func TestGeminiFallbackEvidenceReconcilesCumulativeChunksAndCandidateBoundaries(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	for _, raw := range []string{
		`{"candidates":[{"index":0,"content":{"parts":[{"text":"hello"}]}},{"index":1,"content":{"parts":[{"text":"world"}]}}]}`,
		`{"candidates":[{"index":0,"content":{"parts":[{"text":"hello again"}]}},{"index":1,"content":{"parts":[{"text":"world again"}]}}]}`,
		`{"candidates":[{"index":0,"content":{"parts":[{"text":"hello again"}]}},{"index":1,"content":{"parts":[{"text":"world again"}]}}]}`,
	} {
		attempt.ObserveUsage([]byte(raw))
	}
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.NotNil(t, result.GeminiUsageFallback)
	require.Equal(t, geminiFallbackCount(t, "hello again")+geminiFallbackCount(t, "world again"), result.GeminiUsageFallback.CandidateTokens)
}

func TestGeminiFallbackEvidenceCountsCumulativeFunctionArgumentsOnce(t *testing.T) {
	measure := func(events []string) int {
		c := newGeminiAttemptTestContext(t)
		attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
		for _, raw := range events {
			attempt.ObserveUsage([]byte(raw))
		}
		result := &ForwardResult{Model: "gemini-test"}
		finishGeminiUpstreamAttempt(attempt, result, nil)
		require.NotNil(t, result.GeminiUsageFallback)
		return result.GeminiUsageFallback.CandidateTokens
	}
	first := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup","args":"{\"city\":\"Shang"}}]}}]}`
	last := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup","args":"{\"city\":\"Shanghai\"}"}}]}}]}`
	require.Equal(t, measure([]string{last}), measure([]string{first, last, last}))
	objectFirst := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup","args":{"city":"Shang"}}}]}}]}`
	objectLast := `{"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup","args":{"city":"Shanghai"}}}]}}]}`
	require.Equal(t, measure([]string{objectLast}), measure([]string{objectFirst, objectLast, objectLast}), "structured argument snapshots replace earlier objects despite changed closing braces")
}

func TestGeminiFallbackEvidenceKeepsParallelSameNameToolsSeparate(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	attempt.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"lookup","args":{"city":"Shanghai"}}},{"functionCall":{"name":"lookup","args":{"city":"Beijing"}}}]}}]}`))
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.NotNil(t, result.GeminiUsageFallback)
	require.Equal(t, 2*geminiFallbackCount(t, "lookup")+geminiFallbackCount(t, `{"city":"Shanghai"}`)+geminiFallbackCount(t, `{"city":"Beijing"}`), result.GeminiUsageFallback.CandidateTokens)
}

func TestGeminiFallbackEvidenceMixedCumulativeFramesDoNotRepeatTools(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	for _, text := range []string{"checking", "checking", "checking again"} {
		raw, err := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{
			map[string]any{"text": text}, map[string]any{"functionCall": map[string]any{"name": "lookup", "args": map[string]any{"city": "Shanghai"}}},
		}}}}})
		require.NoError(t, err)
		attempt.ObserveUsage(raw)
	}
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.Equal(t, geminiFallbackCount(t, "checking again")+geminiFallbackCount(t, "lookup")+geminiFallbackCount(t, `{"city":"Shanghai"}`), result.GeminiUsageFallback.CandidateTokens)
}

func TestGeminiFallbackEvidenceExplicitToolIdentitySurvivesNewText(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	call := `{"candidates":[{"content":{"parts":[{"functionCall":{"id":"call-a","name":"lookup","args":{"city":"Shanghai"}}}]}}]}`
	attempt.ObserveUsage([]byte(call))
	attempt.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"text":"checking"}]}}]}`))
	attempt.ObserveUsage([]byte(call))
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.Equal(t, geminiFallbackCount(t, "checking")+geminiFallbackCount(t, "lookup")+geminiFallbackCount(t, `{"city":"Shanghai"}`), result.GeminiUsageFallback.CandidateTokens)
}

func TestGeminiFallbackEvidenceRedirectDiscardsPreparedInputCount(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	request, err := http.NewRequest(http.MethodPost, "https://original.example.invalid/v1beta/models/gemini-test:streamGenerateContent", strings.NewReader(`{"contents":[{"parts":[{"text":"hello"}]}]}`))
	require.NoError(t, err)
	request.Header.Set("x-goog-api-key", "test-key")
	attempt.ObserveRequest(request)
	finalRequest, err := http.NewRequest(http.MethodPost, "https://redirected.example.invalid/inference", nil)
	require.NoError(t, err)
	finalRequest.Header.Set("x-goog-api-key", "test-key")
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Request: finalRequest})
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.NotNil(t, result.GeminiUsageFallback.Input)
	require.Nil(t, result.GeminiUsageFallback.Input.CountRequest)
	require.Empty(t, result.GeminiUsageFallback.Input.CalibrationKey, "a different upstream identity cannot train or use the original provider's calibration")
}

func TestGeminiFallbackEvidenceBoundsReceivedSemanticMemory(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	large := strings.Repeat("hello ", (4<<20)/6+10)
	raw, err := json.Marshal(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": large}}}}}})
	require.NoError(t, err)
	attempt.ObserveUsage(raw)
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.True(t, result.GeminiUsageFallback.OutputTruncated)
	require.Positive(t, result.GeminiUsageFallback.CandidateTokens)
	require.Less(t, result.GeminiUsageFallback.CandidateTokens, geminiFallbackCount(t, large), "omitted content is not invented or extrapolated into a charge")
}

func TestGeminiFallbackEvidenceRetainsMeasuredComponentsAndReleasesCountRequest(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	request, err := http.NewRequest(http.MethodPost, "https://ledger.example.invalid/v1beta/models/gemini-test:streamGenerateContent", strings.NewReader(`{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}`))
	require.NoError(t, err)
	request.Header.Set("x-goog-api-key", "test-key")
	attempt.ObserveRequest(request)
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header)})
	attempt.ObserveUsage([]byte(`{"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0,"thoughtsTokenCount":5},"candidates":[{"content":{"parts":[{"text":"fallback content"}]}}]}`))
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	require.NotNil(t, result.GeminiUsageFallback)
	require.Equal(t, uint8(7), result.GeminiUsageFallback.UsageFields)
	require.Zero(t, result.GeminiUsageFallback.MeasuredCandidates, "explicit upstream zero remains a measured component")
	require.Equal(t, 5, result.GeminiUsageFallback.MeasuredThoughts)
	require.NotNil(t, result.GeminiUsageFallback.Input)
	require.Nil(t, result.GeminiUsageFallback.Input.CountRequest, "measured prompt removes the asynchronous body snapshot")
	require.True(t, result.MeasuredUsageComplete)
}

func TestGeminiFallbackEvidenceReleasesOutputBuffersAfterAuthoritativeCandidateCount(t *testing.T) {
	for _, candidates := range []int{0, 30} {
		c := newGeminiAttemptTestContext(t)
		attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
		attempt.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"text":"earlier answer"},{"thought":true,"text":"earlier thought"}]}}]}`))
		raw, err := json.Marshal(map[string]any{"usageMetadata": map[string]int{"candidatesTokenCount": candidates, "thoughtsTokenCount": 4}})
		require.NoError(t, err)
		attempt.ObserveUsage(raw)
		attempt.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"text":"unused later answer"},{"thought":true,"text":"unused later thought"}]}}]}`))
		result := &ForwardResult{Model: "gemini-test"}
		finishGeminiUpstreamAttempt(attempt, result, nil)
		require.Zero(t, result.GeminiUsageFallback.CandidateTokens, "authoritative output must not run an unused tokenizer")
		require.Zero(t, result.GeminiUsageFallback.ThoughtTokens)
		require.Equal(t, candidates, result.GeminiUsageFallback.MeasuredCandidates)
		require.Equal(t, 4, result.GeminiUsageFallback.MeasuredThoughts)
		require.Nil(t, attempt.fallbackCandidates, "generated text must be released once its estimate is unnecessary")
	}
}

func TestGeminiFallbackEvidenceDoesNotReusePriorAttemptOrAcceptMalformedContent(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	first := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	first.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"text":"old output"}]}}]}`))
	first.EndClientCanceled()
	second := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	second.ObserveResponse(&http.Response{StatusCode: http.StatusServiceUnavailable, Header: make(http.Header)})
	second.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"text":"truncated"}]}}]`))
	result := &ForwardResult{Model: "gemini-test"}
	finishGeminiUpstreamAttempt(second, result, nil)
	require.NotNil(t, result.GeminiUsageFallback)
	require.Zero(t, result.GeminiUsageFallback.CandidateTokens)
	require.Zero(t, result.GeminiUsageFallback.ThoughtTokens)
	require.False(t, result.GeminiUsageFallback.UpstreamAccepted)
}

func TestGeminiFallbackEvidenceSurvivesDiagnosticStatusRewriteOnAcceptedStream(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 701, Platform: PlatformGemini}, "gemini-test", true)
	attempt.ObserveResponse(&http.Response{StatusCode: http.StatusOK, Header: make(http.Header)})
	attempt.ObserveUsage([]byte(`{"candidates":[{"content":{"parts":[{"text":"received before failed downstream write"}]}}]}`))
	attempt.End(errors.New("write interrupted before request context cancellation"))
	require.Equal(t, http.StatusBadGateway, attempt.event.UpstreamStatusCode)
	result := GeminiResultFromLastAttempt(c, 701, "gemini-test", true, 0)
	require.NotNil(t, result, "the caller has independently verified downstream cancellation; rewritten diagnostic HTTP status must not erase accepted inference")
	require.True(t, result.GeminiUsageFallback.UpstreamAccepted)
	require.Positive(t, result.GeminiUsageFallback.CandidateTokens)
}
