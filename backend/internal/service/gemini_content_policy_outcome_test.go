//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiOutcomeHasContentPolicyFailure(t *testing.T) {
	require.False(t, GeminiOutcomeHasContentPolicyFailure(nil))
	for _, tc := range []struct {
		finishReason string
		want         bool
	}{
		{finishReason: "SAFETY", want: true},
		{finishReason: "recitation", want: true},
		{finishReason: " LANGUAGE ", want: true},
		{finishReason: "BLOCKLIST", want: true},
		{finishReason: "PROHIBITED_CONTENT", want: true},
		{finishReason: "SPII", want: true},
		{finishReason: "STOP"},
		{finishReason: "MAX_TOKENS"},
		{finishReason: "MALFORMED_FUNCTION_CALL"},
		{finishReason: "OTHER"},
		{finishReason: "unknown-provider-reason"},
		{},
	} {
		t.Run(tc.finishReason, func(t *testing.T) {
			collector := NewResponseOutcomeCollector(200, 200)
			collector.MarkCompleted(tc.finishReason)
			outcome := collector.Snapshot()
			require.Equal(t, tc.want, GeminiOutcomeHasContentPolicyFailure(&outcome))
		})
	}
}
