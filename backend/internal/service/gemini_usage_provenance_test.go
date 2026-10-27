//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGeminiUsageProvenancePreservesFieldPresenceAndSelectedOutput(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		events                    []string
		completed                 bool
		delivered                 *int
		inputSource, outputSource string
		input, output             int
	}{
		{name: "cancel without usage", delivered: provenanceInt(7), inputSource: "unreported", outputSource: "unreported"},
		{name: "only output reported", events: []string{`{"usageMetadata":{"candidatesTokenCount":9}}`}, completed: true, inputSource: "unreported", outputSource: "upstream", output: 9},
		{name: "measured zero", events: []string{`{"usageMetadata":{"promptTokenCount":0,"candidatesTokenCount":0}}`}, completed: true, inputSource: "upstream", outputSource: "upstream"},
		{name: "fully cached input", events: []string{`{"usageMetadata":{"promptTokenCount":100,"cachedContentTokenCount":100,"candidatesTokenCount":9}}`}, completed: true, inputSource: "upstream", outputSource: "upstream", output: 9},
		{name: "cache only is not measured input", events: []string{`{"usageMetadata":{"cachedContentTokenCount":100}}`}, delivered: provenanceInt(7), inputSource: "unreported", outputSource: "unreported"},
		{name: "partial updates retain input", events: []string{`{"usageMetadata":{"promptTokenCount":100}}`, `{"usageMetadata":{"candidatesTokenCount":9}}`, `{"usageMetadata":{"promptTokenCount":null}}`}, completed: true, inputSource: "upstream", outputSource: "upstream", input: 100, output: 9},
		{name: "interrupted provider output remains exact", events: []string{`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":9}}`}, delivered: provenanceInt(7), inputSource: "upstream", outputSource: "upstream", input: 100, output: 9},
		{name: "equal delivered count does not replace measurement", events: []string{`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":7}}`}, delivered: provenanceInt(7), inputSource: "upstream", outputSource: "upstream", input: 100, output: 7},
		{name: "completed provider output retained", events: []string{`{"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":9}}`}, completed: true, delivered: provenanceInt(7), inputSource: "upstream", outputSource: "upstream", input: 100, output: 9},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := newGeminiAttemptTestContext(t)
			attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 3, Platform: PlatformGemini}, "gemini-test", true)
			var acc geminiUsageAccumulator
			result := &ForwardResult{RequestID: "usage-provenance", Model: "gemini-test", Stream: true, DeliveredOutputTokens: tc.delivered, Outcome: &ResponseOutcome{StreamCompleted: tc.completed, CollectorVersion: 1}}
			for _, event := range tc.events {
				attempt.ObserveUsage([]byte(event))
				if u := acc.Observe([]byte(event)); u != nil {
					result.Usage = *u
				}
			}
			finishGeminiUpstreamAttempt(attempt, result, nil)
			log := (&GatewayService{}).buildRecordUsageLog(context.Background(), &recordUsageCoreInput{}, result, &APIKey{ID: 1}, &User{ID: 2}, &Account{ID: 3, Platform: PlatformGemini}, nil, result.Model, 1, 1, 1, BillingTypeBalance, false, nil, &recordUsageOpts{})
			data, err := json.Marshal(log)
			require.NoError(t, err)
			var fields map[string]any
			require.NoError(t, json.Unmarshal(data, &fields))
			require.Equal(t, tc.inputSource, fields["InputTokensSource"])
			require.Equal(t, tc.outputSource, fields["OutputTokensSource"])
			require.Equal(t, tc.input, log.InputTokens)
			require.Equal(t, tc.output, log.OutputTokens)
		})
	}
}

func TestGeminiUsageProvenanceDoesNotCreateResponseEvidenceForSync(t *testing.T) {
	c := newGeminiAttemptTestContext(t)
	attempt := BeginGeminiUpstreamAttempt(c, &Account{ID: 3, Platform: PlatformGemini}, "gemini-test", false)
	attempt.ObserveUsage([]byte(`{"usageMetadata":{"promptTokenCount":12,"candidatesTokenCount":3}}`))
	result := &ForwardResult{RequestID: "sync-provenance", Model: "gemini-test", Usage: ClaudeUsage{InputTokens: 12, OutputTokens: 3}}
	finishGeminiUpstreamAttempt(attempt, result, nil)
	log := (&GatewayService{}).buildRecordUsageLog(context.Background(), &recordUsageCoreInput{}, result, &APIKey{ID: 1}, &User{ID: 2}, &Account{ID: 3, Platform: PlatformGemini}, nil, result.Model, 1, 1, 1, BillingTypeBalance, false, nil, &recordUsageOpts{})
	data, err := json.Marshal(log)
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(data, &fields))
	require.Equal(t, "upstream", fields["InputTokensSource"])
	require.Equal(t, "upstream", fields["OutputTokensSource"])
	require.Nil(t, log.Outcome, "source metadata must not invent an empty response")
}

func provenanceInt(n int) *int { return &n }
