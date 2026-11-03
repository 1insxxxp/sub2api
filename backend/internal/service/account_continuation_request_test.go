//go:build unit

package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAccountContinuationSafeRequestShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol AccountContinuationProtocol
		body     string
		allowed  bool
	}{
		{"chat text", AccountContinuationChat, `{"model":"m","messages":[{"role":"user","content":"hello"}]}`, true},
		{"messages text", AccountContinuationMessages, `{"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]}`, true},
		{"gemini text", AccountContinuationGemini, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`, true},
		{"responses text", AccountContinuationResponses, `{"input":"hello"}`, true},
		{"chat audio output", AccountContinuationChat, `{"messages":[],"modalities":["text","audio"],"audio":{"voice":"alloy","format":"wav"}}`, false},
		{"chat text output", AccountContinuationChat, `{"messages":[],"modalities":["text"]}`, true},
		{"gemini image output", AccountContinuationGemini, `{"contents":[],"generationConfig":{"responseModalities":["TEXT","IMAGE"]}}`, false},
		{"gemini text output", AccountContinuationGemini, `{"contents":[],"generationConfig":{"responseModalities":["TEXT"]}}`, true},
		{"gemini audio config", AccountContinuationGemini, `{"contents":[],"generationConfig":{"speechConfig":{"voiceConfig":{}}}}`, false},
		{"chat tool", AccountContinuationChat, `{"messages":[],"tools":[{"type":"function"}]}`, false},
		{"chat image", AccountContinuationChat, `{"messages":[{"role":"user","content":[{"type":"image_url","image_url":"x"}]}]}`, false},
		{"messages opaque", AccountContinuationMessages, `{"messages":[{"role":"assistant","content":[{"type":"thinking","thinking":"private","signature":"s"}]}]}`, false},
		{"gemini signature", AccountContinuationGemini, `{"contents":[{"parts":[{"text":"hi","thoughtSignature":"s"}]}]}`, false},
		{"gemini multiple", AccountContinuationGemini, `{"contents":[],"generationConfig":{"candidateCount":2}}`, false},
		{"gemini json", AccountContinuationGemini, `{"contents":[],"generationConfig":{"responseMimeType":"application/json"}}`, false},
		{"responses tool", AccountContinuationResponses, `{"input":[{"type":"function_call_output","call_id":"c","output":"x"}]}`, false},
		{"responses stateful", AccountContinuationResponses, `{"input":"hi","previous_response_id":"r"}`, false},
		{"strict output", AccountContinuationResponses, `{"input":"hi","text":{"format":{"type":"json_schema"}}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.allowed, AccountContinuationRequestAllowed(tc.protocol, []byte(tc.body)))
		})
	}
}

func TestAccountContinuationBuildsNativeTranscriptWithoutInventingCaps(t *testing.T) {
	for _, tc := range []struct {
		protocol   AccountContinuationProtocol
		body, path string
	}{
		{AccountContinuationChat, `{"model":"m","messages":[{"role":"user","content":"hi"}]}`, "messages"},
		{AccountContinuationMessages, `{"model":"m","messages":[{"role":"user","content":"hi"}]}`, "messages"},
		{AccountContinuationGemini, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}]}`, "contents"},
		{AccountContinuationResponses, `{"model":"m","input":"hi"}`, "input"},
	} {
		t.Run(string(tc.protocol), func(t *testing.T) {
			got, err := BuildAccountContinuationBody(tc.protocol, []byte(tc.body), "partial answer", 0)
			require.NoError(t, err)
			require.Len(t, gjson.GetBytes(got, tc.path).Array(), 3)
			require.Contains(t, string(got), "partial answer")
			require.NotContains(t, string(got), "max_tokens")
			require.NotContains(t, string(got), "max_output_tokens")
			require.NotContains(t, string(got), "maxOutputTokens")
		})
	}
}

func TestAccountContinuationNeverRaisesClientCaps(t *testing.T) {
	for _, tc := range []struct {
		protocol   AccountContinuationProtocol
		body, path string
	}{
		{AccountContinuationChat, `{"messages":[],"max_tokens":20}`, "max_tokens"},
		{AccountContinuationChat, `{"messages":[],"max_completion_tokens":20}`, "max_completion_tokens"},
		{AccountContinuationMessages, `{"messages":[],"max_tokens":20}`, "max_tokens"},
		{AccountContinuationGemini, `{"contents":[],"generationConfig":{"maxOutputTokens":20}}`, "generationConfig.maxOutputTokens"},
		{AccountContinuationResponses, `{"input":"hi","max_output_tokens":20}`, "max_output_tokens"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			got, err := BuildAccountContinuationBody(tc.protocol, []byte(tc.body), "partial", 90)
			require.NoError(t, err)
			require.Equal(t, int64(20), gjson.GetBytes(got, tc.path).Int())
			got, err = BuildAccountContinuationBody(tc.protocol, []byte(tc.body), "partial", 7)
			require.NoError(t, err)
			require.Equal(t, int64(7), gjson.GetBytes(got, tc.path).Int())
		})
	}
}

func TestAccountContinuationBudgetStartsAfterNormalLongFirstRound(t *testing.T) {
	budget := NewAccountContinuationBudget(domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 3, MaxDurationSeconds: 10}, AccountContinuationChat, []byte(`{"messages":[]}`))
	start := time.Now().Add(time.Hour)
	require.True(t, budget.CanContinue(start, 0, 100))
	require.True(t, budget.CanContinue(start.Add(9*time.Second), 1, 100000))
	require.False(t, budget.CanContinue(start.Add(11*time.Second), 2, 100001))
	require.Zero(t, budget.RemainingOutputTokens(100000))
}

func TestAccountContinuationBudgetPreservesExplicitCumulativeLimit(t *testing.T) {
	budget := NewAccountContinuationBudget(domain.AntiTruncationPolicy{Enabled: true, MaxRounds: 3}, AccountContinuationGemini, []byte(`{"contents":[],"generationConfig":{"maxOutputTokens":20}}`))
	require.Equal(t, 7, budget.RemainingOutputTokens(13))
	require.True(t, budget.CanContinue(time.Now(), 0, 13))
	require.False(t, budget.CanContinue(time.Now(), 0, 20))
	require.False(t, budget.CanContinue(time.Now(), 3, 13))
}

func TestAccountContinuationRejectsOpaqueSystemAndExternalCache(t *testing.T) {
	for _, tc := range []struct {
		p    AccountContinuationProtocol
		body string
	}{
		{AccountContinuationMessages, `{"messages":[{"role":"user","content":"hi"}],"system":[{"type":"image","source":{"type":"base64","data":"x"}}]}`},
		{AccountContinuationMessages, `{"messages":[{"role":"user","content":"hi"}],"system":[{"type":"thinking","thinking":"x","signature":"opaque"}]}`},
		{AccountContinuationGemini, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"systemInstruction":{"parts":[{"inlineData":{"data":"x"}}]}}`},
		{AccountContinuationGemini, `{"contents":[{"role":"function","parts":[{"text":"hi"}]}]}`},
		{AccountContinuationGemini, `{"contents":[{"role":"user","parts":[{"text":"hi"}]}],"cachedContent":"cached/123"}`},
		{AccountContinuationResponses, `{"input":"hi","instructions":[{"type":"image","data":"x"}]}`},
	} {
		require.False(t, AccountContinuationRequestAllowed(tc.p, []byte(tc.body)), tc.body)
	}
}
