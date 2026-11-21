//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSettledProtectedSingleCallRetainsRawUsage(t *testing.T) {
	group := &Group{ID: 4, RateMultiplier: 1}
	key := &APIKey{ID: 2, GroupID: &group.ID, Group: group}
	user := &User{ID: 1}
	account := &Account{ID: 3, Type: AccountTypeAPIKey}
	protection := &InputTokenProtection{Limit: 18}

	check := func(t *testing.T, log *UsageLog, rawInput int) ContinuationUsageAudit {
		t.Helper()
		require.NotNil(t, log)
		require.Zero(t, log.CacheReadTokens)
		require.Zero(t, log.InputTokens)
		require.Equal(t, 18, log.CacheCreationTokens)
		require.NotNil(t, log.Outcome)
		require.Len(t, log.Outcome.ContinuationUsage, 1)
		audit := log.Outcome.ContinuationUsage[0]
		require.Equal(t, rawInput, audit.InputTokens)
		require.Equal(t, 41, audit.CacheReadTokens)
		require.Equal(t, 24, audit.CacheCreationTokens)
		require.Equal(t, 8, audit.OutputTokens)
		require.Equal(t, log.InputTokens, audit.CustomerInputTokens)
		require.Equal(t, log.OutputTokens, audit.CustomerOutputTokens)
		require.Equal(t, UsageTokenSourceUpstream, audit.InputTokensSource)
		require.Equal(t, UsageTokenSourceUpstream, audit.OutputTokensSource)
		require.Equal(t, "group_input_token_protection", audit.InputTokenPolicy)
		require.Equal(t, protection.Limit, audit.InputTokenPolicyCount)
		require.Zero(t, log.AntiTruncationRounds)
		require.Zero(t, log.Outcome.CollectorVersion)
		require.Equal(t, UsageRequestUnknown, ClassifyUsageRequestStatus(log.Outcome))
		public, err := json.Marshal(log.Outcome)
		require.NoError(t, err)
		require.NotContains(t, string(public), "cache_read_tokens")
		require.NotContains(t, string(public), "continuation_usage")
		return audit
	}

	t.Run("messages", func(t *testing.T) {
		logs := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newGatewayRecordUsageServiceForTest(logs, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
		svc.usageBillingRepo = &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
		result := &ForwardResult{
			RequestID: "one-message", Model: "claude-sonnet-4", InputTokenProtection: protection,
			InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceUpstream,
			Usage: ClaudeUsage{InputTokens: 5135, CacheReadInputTokens: 41, CacheCreationInputTokens: 24, CacheCreation5mTokens: 7, CacheCreation1hTokens: 17, OutputTokens: 8},
		}
		require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{
			Result: result, APIKey: key, Account: account, User: user, ForceCacheBilling: true,
		}))
		audit := check(t, logs.lastLog, 5135)
		require.False(t, audit.InputIncludesCache)
		require.Equal(t, 7, audit.CacheCreation5mTokens)
		require.Equal(t, 17, audit.CacheCreation1hTokens)
	})

	t.Run("openai", func(t *testing.T) {
		logs := &openAIRecordUsageLogRepoStub{inserted: true}
		svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, &openAIRecordUsageBillingRepoStub{}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
		result := &OpenAIForwardResult{
			RequestID: "one-response", Model: "gpt-5.1", InputTokenProtection: protection,
			InputTokensSource: UsageTokenSourceUpstream, OutputTokensSource: UsageTokenSourceUpstream,
			Usage: OpenAIUsage{InputTokens: 5200, CacheReadInputTokens: 41, CacheCreationInputTokens: 24, CacheCreation30mInputTokens: 9, CacheCreation12hInputTokens: 15, ImageInputTokens: 38, ImageCacheReadTokens: 25, OutputTokens: 8},
		}
		require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: result, APIKey: key, Account: account, User: user}))
		audit := check(t, logs.lastLog, 5200)
		require.True(t, audit.InputIncludesCache)
		require.Equal(t, 9, audit.CacheCreation30mTokens)
		require.Equal(t, 15, audit.CacheCreation12hTokens)
		require.Equal(t, 38, audit.ImageInputTokens)
		require.Equal(t, 25, audit.ImageCacheReadTokens)
	})
}
