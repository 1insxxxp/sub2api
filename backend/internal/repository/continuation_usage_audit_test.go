//go:build unit

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestContinuationUsageAuditSnapshotRoundTrip(t *testing.T) {
	raw := []byte(`{"version":5,"usage":{"request_id":"continued","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-11-03T00:00:00Z","outcome":{"collector_version":1,"continuation_usage":[{"request_id":"round1","ledger_request_id":"ledger1","model":"gemini-4-argon","input_tokens":120000,"output_tokens":500,"customer_input_tokens":110000,"customer_output_tokens":500,"input_tokens_source":"upstream","output_tokens_source":"upstream","input_token_policy":"vertex_like_discount_v1","input_token_policy_count":110000},{"request_id":"round2","ledger_request_id":"ledger2","model":"gemini-4-argon","input_tokens":120500,"output_tokens":100,"customer_input_tokens":100000,"customer_output_tokens":50,"input_tokens_source":"upstream","output_tokens_source":"upstream"}]}}}`)
	log, err := decodeUsagePersistencePayload(raw, "continued", 2)
	require.NoError(t, err)
	public, err := json.Marshal(log.Outcome)
	require.NoError(t, err)
	require.NotContains(t, string(public), "round1")
	cmd := &service.UsageBillingCommand{RequestID: log.RequestID, APIKeyID: log.APIKeyID, UserID: log.UserID, AccountID: log.AccountID, UsageLog: log}
	encoded, err := encodeUsagePersistencePayload(cmd, 0)
	require.NoError(t, err)
	var want, got map[string]any
	require.NoError(t, json.Unmarshal(raw, &want))
	require.NoError(t, json.Unmarshal(encoded, &got))
	require.EqualValues(t, 5, got["version"])
	require.Equal(t, want["usage"].(map[string]any)["outcome"].(map[string]any)["continuation_usage"], got["usage"].(map[string]any)["outcome"].(map[string]any)["continuation_usage"])
}

func TestContinuationUsageAuditPersistsAfterOutcomeUpsert(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := newUsageLogRepositoryWithSQL(nil, db)
	audit := []service.ContinuationUsageAudit{{RequestID: "first", Model: "gpt-5", InputTokensSource: service.UsageTokenSourceUpstream, OutputTokensSource: service.UsageTokenSourceUpstream, InputTokens: 10, OutputTokens: 2, CustomerInputTokens: 10, CustomerOutputTokens: 2}}
	log := &service.UsageLog{RequestID: "continued", APIKeyID: 2, UserID: 1, AccountID: 3, Outcome: &service.ResponseOutcome{CollectorVersion: 1, ContinuationUsage: audit}}
	mock.ExpectExec(`INSERT INTO usage_response_outcomes`).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(`UPDATE usage_response_outcomes SET continuation_usage=\$3::jsonb`).WithArgs("continued", int64(2), sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
	require.NoError(t, repo.upsertResponseOutcome(context.Background(), log))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestContinuationUsageAuditRejectsOlderVersionsAndUnboundedEvidence(t *testing.T) {
	for _, version := range []int{1, 2, 3, 4} {
		for _, value := range []string{`null`, `[]`} {
			raw := []byte(fmt.Sprintf(`{"version":%d,"usage":{"request_id":"continued","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-11-03T00:00:00Z","outcome":{"continuation_usage":%s}}}`, version, value))
			_, err := decodeUsagePersistencePayload(raw, "continued", 2)
			require.Error(t, err)
		}
	}
	for _, extra := range []string{`,"prompt":"private text"`, `,"api_key":"secret"`, `,"input_tokens":-1`} {
		raw := []byte(`{"version":5,"usage":{"request_id":"continued","api_key_id":2,"user_id":1,"account_id":3,"created_at":"2026-11-03T00:00:00Z","outcome":{"continuation_usage":[{"request_id":"first","model":"gpt-5","input_tokens_source":"upstream","output_tokens_source":"upstream"` + extra + `}]}}}`)
		_, err := decodeUsagePersistencePayload(raw, "continued", 2)
		require.Error(t, err)
	}
	rounds := make([]service.ContinuationUsageAudit, 5)
	require.Error(t, service.ValidateContinuationUsageAudit(rounds))
}
