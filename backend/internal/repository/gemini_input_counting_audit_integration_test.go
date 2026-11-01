//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestGeminiInputCountingAuditRetryPersistsAuditAtomicallyWithoutRebilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	rawInput, policyCount := 1037, 667
	f.cmd.UsageLog.InputTokens = policyCount
	f.cmd.UsageLog.Outcome.UpstreamInputTokens = &rawInput
	f.cmd.UsageLog.Outcome.InputTokenPolicy = "vertex_like_discount_v1"
	f.cmd.UsageLog.Outcome.InputTokenPolicyCount = &policyCount
	result, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, f.cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)
	repo := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, repo, "audit-worker")
	removeFailure := usagePersistenceRejectInsert(t, "usage_response_outcomes", fmt.Sprintf("NEW.api_key_id=%d", f.keyID))
	require.Error(t, repo.Deliver(ctx, event.ID, "audit-worker"))
	balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Zero(t, usage)
	require.Zero(t, outcome)
	require.Equal(t, 1, dedup)
	require.Equal(t, 1, pending)
	removeFailure()
	require.NoError(t, repo.Deliver(ctx, event.ID, "audit-worker"))
	require.NoError(t, repo.Deliver(ctx, event.ID, "audit-worker"))
	balance, quota, usage, outcome, dedup, pending = usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Equal(t, 1, dedup)
	require.Zero(t, pending)
	var storedInput, storedRaw, storedCandidate int
	var storedPolicy string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT ul.input_tokens, uro.upstream_input_tokens, uro.input_token_policy, uro.input_token_policy_count
		FROM usage_logs ul JOIN usage_response_outcomes uro ON uro.usage_log_id=ul.id
		WHERE ul.request_id=$1 AND ul.api_key_id=$2`, f.requestID, f.keyID).Scan(&storedInput, &storedRaw, &storedPolicy, &storedCandidate))
	require.Equal(t, policyCount, storedInput)
	require.Equal(t, rawInput, storedRaw)
	require.Equal(t, "vertex_like_discount_v1", storedPolicy)
	require.Equal(t, policyCount, storedCandidate)
	// A later source-only retry must not clear audit evidence or change usage.
	metadataOnly := &service.UsageLog{RequestID: f.requestID, APIKeyID: f.keyID, UserID: f.userID, AccountID: f.accountID, InputTokensSource: service.UsageTokenSourceUpstream}
	usageRepo := newUsageLogRepositoryWithSQL(client, integrationDB)
	require.NoError(t, usageRepo.UpsertResponseOutcome(ctx, metadataOnly))
	var usageID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2`, f.requestID, f.keyID).Scan(&usageID))
	read, err := usageRepo.GetByID(ctx, usageID)
	require.NoError(t, err)
	require.Equal(t, policyCount, read.InputTokens)
	require.Equal(t, rawInput, *read.Outcome.UpstreamInputTokens)
	require.Equal(t, "vertex_like_discount_v1", read.Outcome.InputTokenPolicy)
	require.Equal(t, policyCount, *read.Outcome.InputTokenPolicyCount)
}

func TestGeminiInputCountingAuditKeepsUnknownDistinctFromZeroAcrossMigration(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	_, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, f.cmd)
	require.NoError(t, err)
	repo := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, repo, "audit-legacy-worker")
	require.NoError(t, repo.Deliver(ctx, event.ID, "audit-legacy-worker"))

	// A legacy/default-off snapshot has no counting evidence. Reapplying the
	// additive DDL must not turn its unknown counts into reported zero.
	migration, err := migrations.FS.ReadFile("257_gemini_input_counting_audit.sql")
	require.NoError(t, err)
	tx, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	_, err = tx.ExecContext(ctx, string(migration))
	require.NoError(t, err)
	require.NoError(t, tx.Commit())
	var rawInput, candidate sql.NullInt64
	var policy string
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT upstream_input_tokens, input_token_policy, input_token_policy_count
		FROM usage_response_outcomes WHERE request_id=$1 AND api_key_id=$2`, f.requestID, f.keyID).Scan(&rawInput, &policy, &candidate))
	require.False(t, rawInput.Valid)
	require.False(t, candidate.Valid)
	require.Empty(t, policy)

	// The durable audit representation also supports explicit measured zero;
	// policy eligibility is decided before persistence and never inferred here.
	zero := 0
	log := &service.UsageLog{RequestID: f.requestID, APIKeyID: f.keyID, UserID: f.userID, AccountID: f.accountID,
		Outcome: &service.ResponseOutcome{UpstreamInputTokens: &zero, InputTokenPolicy: service.GeminiInputCountingVertexLikeDiscountV1, InputTokenPolicyCount: &zero}}
	usageRepo := newUsageLogRepositoryWithSQL(client, integrationDB)
	require.NoError(t, usageRepo.UpsertResponseOutcome(ctx, log))
	log.Outcome = nil
	log.InputTokensSource = service.UsageTokenSourceUpstream
	require.NoError(t, usageRepo.UpsertResponseOutcome(ctx, log), "metadata retries must preserve explicit zero audit")
	var usageID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT id FROM usage_logs WHERE request_id=$1 AND api_key_id=$2", f.requestID, f.keyID).Scan(&usageID))
	read, err := usageRepo.GetByID(ctx, usageID)
	require.NoError(t, err)
	require.NotNil(t, read.Outcome)
	require.NotNil(t, read.Outcome.UpstreamInputTokens)
	require.NotNil(t, read.Outcome.InputTokenPolicyCount)
	require.Zero(t, *read.Outcome.UpstreamInputTokens)
	require.Zero(t, *read.Outcome.InputTokenPolicyCount)
	require.True(t, read.Outcome.StreamCompleted, "audit-only writes must preserve existing response evidence")
	_, _, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Equal(t, 1, dedup)
	require.Zero(t, pending)
}
