//go:build unit

package repository

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func continuationCategoryOutboxCommand() *service.UsageBillingCommand {
	source := int64(17)
	log := &service.UsageLog{
		RequestID: "combined-continuation", APIKeyID: 2, UserID: 1, AccountID: 3, GroupID: &source,
		CreatedAt: time.Date(2026, time.November, 3, 0, 0, 0, 0, time.UTC),
		Outcome: &service.ResponseOutcome{CollectorVersion: service.ResponseOutcomeCollectorVersion, ContinuationUsage: []service.ContinuationUsageAudit{
			{RequestID: "first-round", Model: "gemini-4-argon", InputTokens: 120000, OutputTokens: 100, CustomerInputTokens: 110000, CustomerOutputTokens: 50,
				InputTokensSource: service.UsageTokenSourceUpstream, OutputTokensSource: service.UsageTokenSourceUpstream},
		}},
	}
	return &service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: log.APIKeyID, UserID: log.UserID, AccountID: log.AccountID, UsageLog: log}
}

func TestContinuationCategoryOutboxCombinedKeepsVersion5(t *testing.T) {
	cmd := continuationCategoryOutboxCommand()
	entry := int64(91)
	cmd.UsageLog.CategoryDefaultGroupID = &entry
	raw, err := encodeUsagePersistencePayload(cmd, 0)
	require.NoError(t, err)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	require.EqualValues(t, 5, payload["version"], "category audit must not downgrade continuation evidence to v4")
	decoded, err := decodeUsagePersistencePayload(raw, cmd.RequestID, cmd.APIKeyID)
	require.NoError(t, err)
	require.Equal(t, cmd.UsageLog.GroupID, decoded.GroupID)
	require.Equal(t, &entry, decoded.CategoryDefaultGroupID)
	require.Equal(t, cmd.UsageLog.Outcome.ContinuationUsage, decoded.Outcome.ContinuationUsage)
}

func TestContinuationCategoryOutboxVersion5DoesNotRequireCategory(t *testing.T) {
	cmd := continuationCategoryOutboxCommand()
	subscription := int64(43)
	cmd.UsageLog.SubscriptionID = &subscription
	raw, err := encodeUsagePersistencePayload(cmd, 0)
	require.NoError(t, err)
	decoded, err := decodeUsagePersistencePayload(raw, cmd.RequestID, cmd.APIKeyID)
	require.NoError(t, err, "ordinary account continuations must not require a category-default entry")
	require.Nil(t, decoded.CategoryDefaultGroupID)
	require.Equal(t, &subscription, decoded.SubscriptionID)
	require.Equal(t, cmd.UsageLog.Outcome.ContinuationUsage, decoded.Outcome.ContinuationUsage)

	// Optional identity must still be validated when present on a v5 snapshot.
	var payload map[string]any
	require.NoError(t, json.Unmarshal(raw, &payload))
	usage := payload["usage"].(map[string]any)
	usage["category_default_group_id"] = *cmd.UsageLog.GroupID
	invalid, err := json.Marshal(payload)
	require.NoError(t, err)
	_, err = decodeUsagePersistencePayload(invalid, cmd.RequestID, cmd.APIKeyID)
	require.Error(t, err)
}

func TestContinuationCategoryOutboxVersion4RejectsV5Metadata(t *testing.T) {
	cmd := continuationCategoryOutboxCommand()
	entry := int64(91)
	cmd.UsageLog.CategoryDefaultGroupID = &entry
	cmd.UsageLog.Outcome.ContinuationUsage = nil
	raw, err := encodeUsagePersistencePayload(cmd, 0)
	require.NoError(t, err)
	_, err = decodeUsagePersistencePayload(raw, cmd.RequestID, cmd.APIKeyID)
	require.NoError(t, err, "fixture must be an otherwise valid category-default snapshot")
	for _, key := range []string{"continuation_usage", "CONTINUATION_USAGE"} {
		for _, value := range []any{nil, []any{}} {
			var payload map[string]any
			require.NoError(t, json.Unmarshal(raw, &payload))
			require.EqualValues(t, 4, payload["version"])
			outcome := payload["usage"].(map[string]any)["outcome"].(map[string]any)
			outcome[key] = value
			invalid, err := json.Marshal(payload)
			require.NoError(t, err)
			_, err = decodeUsagePersistencePayload(invalid, cmd.RequestID, cmd.APIKeyID)
			require.Error(t, err, "v4 cannot silently carry null or empty v5 evidence")
		}
	}
}
