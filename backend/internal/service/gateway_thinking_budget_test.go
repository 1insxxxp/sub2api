//go:build unit

package service

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

const basetenBudgetError = "Error from provider (Console Go): Upstream request failed: [invalid_request_error] must be greater than 1024 to reserve tokens for a final answer when Baseten reasoning is enabled"

func TestThinkingBudgetConstraintErrors(t *testing.T) {
	for _, tc := range []struct {
		message string
		want    bool
	}{
		{basetenBudgetError, true},
		{"MUST BE GREATER THAN 1024 TO RESERVE TOKENS FOR A FINAL ANSWER WHEN BASETEN REASONING IS ENABLED", true},
		{"thinking.budget_tokens must be >= 1024", true},
		{"thinking budget tokens must be greater than or equal to 1024", true},
		{"thinking.budget_tokens: Input should be greater than 1024", true},
		{"max_tokens must be greater than thinking.budget_tokens", true},
		{"reasoning quota exceeded: 1024 tokens", false},
		{"must be greater than 1024", false},
		{"reserve tokens for a final answer when Baseten reasoning is enabled", false},
		{"must be greater than 2048 to reserve tokens for a final answer when Baseten reasoning is enabled", false},
		{"", false},
	} {
		t.Run(tc.message, func(t *testing.T) {
			require.Equal(t, tc.want, isThinkingBudgetConstraintError(tc.message))
		})
	}
}

func TestRectifyThinkingBudgetFinalAnswerReserve(t *testing.T) {
	for _, tc := range []struct {
		maxTokens int
		want      int64
		budget    int64
		changed   bool
	}{
		{1024, 1024, 32000, false},
		{4096, 4096, 4095, true},
		{32000, 32000, 31999, true},
		{32001, 32001, 32000, false},
		{40000, 40000, 32000, false},
		{64000, 64000, 32000, false},
		{100000, 100000, 32000, false},
	} {
		t.Run(fmt.Sprint(tc.maxTokens), func(t *testing.T) {
			body := []byte(fmt.Sprintf(`{"model":"example","max_tokens":%d,"thinking":{"type":"enabled","budget_tokens":32000},"messages":[{"role":"user","content":"hi"}]}`, tc.maxTokens))
			require.True(t, isThinkingBudgetConstraintError(basetenBudgetError))
			got, changed := RectifyThinkingBudget(body)
			require.Equal(t, tc.changed, changed)
			require.Equal(t, tc.want, gjson.GetBytes(got, "max_tokens").Int())
			require.Equal(t, tc.budget, gjson.GetBytes(got, "thinking.budget_tokens").Int())
			require.Equal(t, "hi", gjson.GetBytes(got, "messages.0.content").String())
		})
	}
	adaptive := []byte(`{"max_tokens":1024,"thinking":{"type":"adaptive"}}`)
	got, changed := RectifyThinkingBudget(adaptive)
	require.False(t, changed)
	require.Equal(t, adaptive, got)
	legacy := []byte(`{"max_tokens":32001,"thinking":{"type":"enabled","budget_tokens":32000}}`)
	got, changed = RectifyThinkingBudget(legacy)
	require.False(t, changed)
	require.Equal(t, legacy, got)
}

func TestRectifyThinkingBudgetDoesNotInventOutputAllowance(t *testing.T) {
	for _, body := range []string{
		`{"model":"private-model","thinking":{"type":"enabled","budget_tokens":1}}`,
		`{"model":"claude-opus-4-6","thinking":{"type":"enabled","budget_tokens":1}}`,
	} {
		got, changed := RectifyThinkingBudget([]byte(body))
		require.True(t, changed)
		require.False(t, gjson.GetBytes(got, "max_tokens").Exists())
		require.Equal(t, int64(32000), gjson.GetBytes(got, "thinking.budget_tokens").Int())
	}
	for _, limit := range []string{"0", "null", "1024", "1", "\"invalid\""} {
		body := []byte(fmt.Sprintf(`{"max_tokens":%s,"thinking":{"type":"enabled","budget_tokens":1}}`, limit))
		got, changed := RectifyThinkingBudget(body)
		require.False(t, changed)
		require.Equal(t, body, got)
	}
}
