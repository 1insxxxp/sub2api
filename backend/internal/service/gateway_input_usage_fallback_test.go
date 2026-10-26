//go:build unit

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecordUsage_MissingInputTokensSettlesPreparedEstimate(t *testing.T) {
	repo := &openAIRecordUsageLogRepoStub{inserted: true}
	userRepo := &openAIRecordUsageUserRepoStub{}
	svc := newGatewayRecordUsageServiceForTest(repo, userRepo, &openAIRecordUsageSubRepoStub{})
	svc.usageBillingRepo = &gatewayUsageBillingRepoForTest{userRepo: userRepo}
	groupID := int64(104)
	svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive}, groupID, PlatformAnthropic)
	svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
	result := &ForwardResult{
		RequestID:            "missing-input-usage",
		Model:                "claude-sonnet-4",
		EstimatedInputTokens: 50000,
		Usage:                ClaudeUsage{OutputTokens: 20},
		Duration:             time.Second,
		Outcome:              &ResponseOutcome{HasText: true, DisconnectSource: DisconnectSourceClient},
	}
	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result:  result,
		APIKey:  &APIKey{ID: 101, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformAnthropic, RateMultiplier: 1}},
		User:    &User{ID: 102},
		Account: &Account{ID: 103, Platform: PlatformAnthropic},
	})
	require.NoError(t, err)
	require.NotNil(t, repo.lastLog)
	require.Equal(t, 50000, repo.lastLog.InputTokens)
	require.True(t, result.InputTokensEstimated)
	require.Positive(t, repo.lastLog.InputCost)
	require.Equal(t, 1, userRepo.deductCalls)
	require.InDelta(t, repo.lastLog.ActualCost, userRepo.lastAmount, 1e-12)
	require.NotNil(t, repo.lastLog.AccountStatsCost)
	require.InDelta(t, repo.lastLog.TotalCost, *repo.lastLog.AccountStatsCost, 1e-12,
		"upstream cost and customer cost must use the same recovered input tokens")
}

func TestRecordUsage_MissingInputEstimateRespectsForceCacheBilling(t *testing.T) {
	repo := &usageLogRepoRecordUsageStub{inserted: true}
	svc := newGatewayServiceForRecordUsageTest(repo)
	result := &ForwardResult{
		RequestID: "missing-input-force-cache", Model: "claude-sonnet-4", EstimatedInputTokens: 50000,
		Outcome: &ResponseOutcome{HasText: true, StreamCompleted: true},
	}
	err := svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: result, APIKey: &APIKey{ID: 1}, User: &User{ID: 2},
		Account: &Account{ID: 3, Platform: PlatformAnthropic}, ForceCacheBilling: true,
	})
	require.NoError(t, err)
	require.NotNil(t, repo.last)
	require.Zero(t, repo.last.InputTokens)
	require.Equal(t, 50000, repo.last.CacheReadTokens)
	require.Positive(t, repo.last.CacheReadCost)
	require.True(t, result.InputTokensEstimated)
}

func TestApplyMissingInputTokenEstimate_GenerationAndProviderPrecedence(t *testing.T) {
	delivered := 17
	zero := 0
	tests := []struct {
		name          string
		usage         ClaudeUsage
		delivered     *int
		outcome       *ResponseOutcome
		wantEstimated bool
	}{
		{name: "provider output proves generation", usage: ClaudeUsage{OutputTokens: 7}, wantEstimated: true},
		{name: "delivered output proves generation", delivered: &delivered, wantEstimated: true},
		{name: "completed explicit zero with text", outcome: &ResponseOutcome{StreamCompleted: true, HasText: true}, wantEstimated: true},
		{name: "partial reasoning proves generation", outcome: &ResponseOutcome{HasReasoning: true, DisconnectSource: DisconnectSourceClient}, wantEstimated: true},
		{name: "tool response proves generation", outcome: &ResponseOutcome{HasToolCall: true}, wantEstimated: true},
		{name: "no response is not billed"},
		{name: "header and heartbeats are not generation", outcome: &ResponseOutcome{EventCount: 5, OutputBytes: 200}},
		{name: "terminal event alone is not generation", delivered: &zero, outcome: &ResponseOutcome{StreamCompleted: true}},
		{name: "cancel before output is not billed", delivered: &zero, outcome: &ResponseOutcome{DisconnectSource: DisconnectSourceClient}},
		{name: "provider input wins", usage: ClaudeUsage{InputTokens: 31, OutputTokens: 7}},
		{name: "zero uncached input with cache read is valid", usage: ClaudeUsage{CacheReadInputTokens: 9000, OutputTokens: 7}},
		{name: "provider cache creation wins", usage: ClaudeUsage{CacheCreationInputTokens: 8000, OutputTokens: 7}},
		{name: "provider nested five minute cache wins", usage: ClaudeUsage{CacheCreation5mTokens: 8000, OutputTokens: 7}},
		{name: "provider nested one hour cache wins", usage: ClaudeUsage{CacheCreation1hTokens: 8000, OutputTokens: 7}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &ForwardResult{EstimatedInputTokens: 5000, Usage: tt.usage, DeliveredOutputTokens: tt.delivered, Outcome: tt.outcome}
			applyMissingInputTokenEstimate(context.Background(), result, &Account{ID: 1})
			require.Equal(t, tt.wantEstimated, result.InputTokensEstimated)
			if tt.wantEstimated {
				require.Equal(t, 5000, result.Usage.InputTokens)
			} else {
				require.Equal(t, tt.usage, result.Usage)
			}
			before := result.Usage
			applyMissingInputTokenEstimate(context.Background(), result, nil)
			require.Equal(t, before, result.Usage, "settlement must not add an estimate twice")
		})
	}
	applyMissingInputTokenEstimate(context.Background(), nil, nil)
}

func TestPrepareAnthropicInputTokenEstimate_UsesPromptAndToolsWithoutBinaryPayload(t *testing.T) {
	base := `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"Please summarize this discussion."}]}`
	withTools := `{"model":"claude-sonnet-4","system":"You are a helpful assistant.","messages":[{"role":"user","content":"Please summarize this discussion."}],"tools":[{"name":"lookup","description":"Lookup detailed records for the requested discussion","input_schema":{"type":"object","properties":{"query":{"type":"string"}}}}]}`
	result := &ForwardResult{}
	prepareAnthropicInputTokenEstimate(result, []byte(base))
	require.Positive(t, result.EstimatedInputTokens)
	baseEstimate := result.EstimatedInputTokens
	prepareAnthropicInputTokenEstimate(result, []byte(withTools))
	require.Greater(t, result.EstimatedInputTokens, baseEstimate)

	imageBody := `{"model":"claude-sonnet-4","messages":[{"role":"user","content":[{"type":"text","text":"Please summarize this discussion."},{"type":"image","source":{"type":"base64","media_type":"image/png","data":"` + strings.Repeat("ABCD", 10000) + `"}}]}]}`
	prepareAnthropicInputTokenEstimate(result, []byte(imageBody))
	require.Positive(t, result.EstimatedInputTokens)
	require.Less(t, result.EstimatedInputTokens, 200, "binary encoding must not be billed as ordinary text tokens")
	prepareAnthropicInputTokenEstimate(result, []byte(`not-json`))
	require.Zero(t, result.EstimatedInputTokens, "failed preparation must not leave a stale estimate")
	prepareAnthropicInputTokenEstimate(nil, []byte(base))
}

func TestPrepareAnthropicInputTokenEstimate_KeepsProviderInputAndCaches(t *testing.T) {
	body := []byte(`{"model":"claude-sonnet-4","messages":[{"role":"user","content":"This request is already metered by the provider."}]}`)
	for _, usage := range []ClaudeUsage{
		{InputTokens: 10}, {CacheReadInputTokens: 10}, {CacheCreationInputTokens: 10},
		{CacheCreation5mTokens: 10}, {CacheCreation1hTokens: 10},
	} {
		result := &ForwardResult{Usage: usage}
		prepareAnthropicInputTokenEstimate(result, body)
		require.Zero(t, result.EstimatedInputTokens)
		require.Equal(t, usage, result.Usage)
	}
}

func TestApplyMissingInputTokenEstimate_GeminiRequiresProviderMeasurement(t *testing.T) {
	delivered := 85073
	for _, tc := range []struct {
		name  string
		usage ClaudeUsage
	}{
		{name: "missing usage"},
		{name: "output alone is not measured input", usage: ClaudeUsage{OutputTokens: 96146}},
		{name: "measured input wins", usage: ClaudeUsage{InputTokens: 55779, OutputTokens: 96146}},
		{name: "cached input remains measured", usage: ClaudeUsage{InputTokens: 47779, CacheReadInputTokens: 8000}},
		{name: "fully cached input remains zero", usage: ClaudeUsage{CacheReadInputTokens: 55779}},
	} {
		for _, complete := range []bool{false, true} {
			name := tc.name + "/interrupted"
			if complete {
				name = tc.name + "/completed"
			}
			t.Run(name, func(t *testing.T) {
				result := &ForwardResult{
					RequestID: "gemini-measured-input", Model: "gemini-2.5-pro",
					Usage: tc.usage, EstimatedInputTokens: 94942, DeliveredOutputTokens: &delivered,
					Outcome: &ResponseOutcome{HasText: true, StreamCompleted: complete},
				}
				applyMissingInputTokenEstimate(context.Background(), result, &Account{ID: 7, Platform: PlatformGemini})
				require.Equal(t, tc.usage, result.Usage, "Gemini input/cache billing requires a provider measurement")
				require.False(t, result.InputTokensEstimated)
				applyMissingInputTokenEstimate(context.Background(), result, &Account{ID: 7, Platform: PlatformGemini})
				require.Equal(t, tc.usage, result.Usage, "settlement retries must not introduce input estimates")
			})
		}
	}
}
