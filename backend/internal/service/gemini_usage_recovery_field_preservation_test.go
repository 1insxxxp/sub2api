//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGeminiUsageRecoveryPreservesReportedFieldsWhenLedgerFillsOnlyMissingUsage(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		fields                    uint8
		usage                     ClaudeUsage
		inputSource, outputSource UsageTokenSource
		want                      ClaudeUsage
	}{
		{"explicit zero input and cache", 1 | 8, ClaudeUsage{}, UsageTokenSourceUpstream, UsageTokenSourceUnreported, ClaudeUsage{OutputTokens: 15}},
		{"explicit zero input without cache field", 1, ClaudeUsage{}, UsageTokenSourceUpstream, UsageTokenSourceUnreported, ClaudeUsage{OutputTokens: 15}},
		{"explicit zero output and measured cache", 2 | 8, ClaudeUsage{CacheReadInputTokens: 50}, UsageTokenSourceUnreported, UsageTokenSourceUpstream, ClaudeUsage{InputTokens: 950, CacheReadInputTokens: 50}},
		{"explicit zero cache with missing prompt", 2 | 8, ClaudeUsage{}, UsageTokenSourceUnreported, UsageTokenSourceUpstream, ClaudeUsage{InputTokens: 1000}},
		{"known gross prompt with missing cache", 1, ClaudeUsage{InputTokens: 600}, UsageTokenSourceUpstream, UsageTokenSourceUnreported, ClaudeUsage{InputTokens: 400, CacheReadInputTokens: 200, OutputTokens: 15}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, upstream := newGeminiUsageRecoveryFixture()
			input.Result.Usage = tc.usage
			input.Result.InputTokensSource, input.Result.OutputTokensSource = tc.inputSource, tc.outputSource
			input.Result.GeminiUsageFallback = &GeminiUsageFallback{UsageFields: tc.fields}
			upstream.resp = geminiUsageRecoveryResponse(200, `{"success":true,"data":[{"type":2,"request_id":"provider-request-1","model_name":"gemini-2.5-flash","prompt_tokens":1000,"completion_tokens":15,"other":"{\"cache_tokens\":200,\"cache_input_includes_cache\":true}"}]}`)
			recovered, err := (&GatewayService{cfg: &config.Config{}, httpUpstream: upstream}).RecoverGeminiUsage(context.Background(), input)
			require.NoError(t, err)
			require.True(t, recovered)
			require.Equal(t, tc.want, input.Result.Usage)
			require.True(t, input.Result.MeasuredUsageComplete)
		})
	}
}

func TestGeminiUsageRecoveryReservesShortTaskDeadlineForBilling(t *testing.T) {
	input, upstream := newGeminiUsageRecoveryFixture()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	recovered, err := (&GatewayService{cfg: &config.Config{}, httpUpstream: upstream}).RecoverGeminiUsage(ctx, input)
	require.ErrorIs(t, err, ErrGeminiUsageRecoveryUnavailable)
	require.False(t, recovered)
	require.Empty(t, upstream.requests)
	require.NoError(t, ctx.Err(), "leave the usage task alive for local fallback and billing")
}

func TestGeminiUsageRecoveryCountReservesShortTaskDeadlineForBilling(t *testing.T) {
	svc, input, upstream := newGeminiCountFallbackFixture(t, `{"totalTokens":21916}`)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, counted, err := svc.CountGeminiInputFallback(ctx, input)
	require.ErrorIs(t, err, ErrGeminiInputCountUnavailable)
	require.False(t, counted)
	require.Empty(t, upstream.requests)
	require.Nil(t, input.Result.GeminiUsageFallback.Input.CountRequest)
	require.NoError(t, ctx.Err(), "counting must reserve the same parent deadline as ledger recovery")
}
