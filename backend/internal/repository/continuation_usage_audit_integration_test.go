//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestContinuationUsageAuditPostgresRetryRetainsPerRoundEvidenceWithoutRebilling(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	f := newUsagePersistenceFixture(t, client)
	firstDelivered, secondDelivered := 500, 50
	audit := []service.ContinuationUsageAudit{
		{
			RequestID: "private-round-first", LedgerRequestID: "private-ledger-first", LedgerIdentityHash: strings.Repeat("a", 64),
			Model: "gemini-4-argon", UpstreamModel: "upstream-gemini-4-argon", InputTokens: 120000, OutputTokens: 500,
			CustomerInputTokens: 110000, CustomerOutputTokens: 500, DeliveredOutputTokens: &firstDelivered,
			InputTokensSource: service.UsageTokenSourceUpstream, OutputTokensSource: service.UsageTokenSourceUpstream,
			ServiceTier: "default", ResponseServiceTier: "default", InputTokenPolicy: service.GeminiInputCountingVertexLikeDiscountV1, InputTokenPolicyCount: 110000,
		},
		{
			RequestID: "private-round-second", LedgerRequestID: "private-ledger-second", LedgerIdentityHash: strings.Repeat("b", 64),
			Model: "gemini-4-argon", UpstreamModel: "upstream-gemini-4-argon", InputTokens: 120500, OutputTokens: 100,
			CustomerInputTokens: 100000, CustomerOutputTokens: 50, DeliveredOutputTokens: &secondDelivered,
			InputTokensSource: service.UsageTokenSourceUpstream, OutputTokensSource: service.UsageTokenSourceUpstream,
			ServiceTier: "priority", ResponseServiceTier: "priority",
		},
	}
	log := f.cmd.UsageLog
	log.Model, log.RequestedModel = "gemini-4-argon", "continued-gemini"
	log.InputTokens, log.OutputTokens, log.UpstreamOutputTokens = 210000, 550, 600
	log.AntiTruncationRounds = 1
	log.Outcome.StreamCompleted = false
	log.Outcome.DisconnectSource = service.DisconnectSourceClient
	log.Outcome.ContinuationUsage = audit
	result, err := NewUsageBillingRepository(client, integrationDB).Apply(ctx, f.cmd)
	require.NoError(t, err)
	require.True(t, result.Applied)

	repo := NewUsagePersistenceOutboxRepository(client, integrationDB)
	event := claimUsagePersistenceFixture(t, repo, "continuation-audit-worker")
	removeFailure := usagePersistenceRejectInsert(t, "usage_response_outcomes", fmt.Sprintf("NEW.api_key_id=%d", f.keyID))
	require.Error(t, repo.Deliver(ctx, event.ID, "continuation-audit-worker"))
	balance, quota, usage, outcome, dedup, pending := usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance)
	require.Equal(t, 2.0, quota)
	require.Zero(t, usage, "a failed outcome write must roll back the logical usage insert")
	require.Zero(t, outcome)
	require.Equal(t, 1, dedup)
	require.Equal(t, 1, pending)

	removeFailure()
	require.NoError(t, repo.Deliver(ctx, event.ID, "continuation-audit-worker"))
	require.NoError(t, repo.Deliver(ctx, event.ID, "continuation-audit-worker"), "acknowledged delivery must be idempotent")
	balance, quota, usage, outcome, dedup, pending = usagePersistenceCounts(t, f)
	require.Equal(t, 8.0, balance, "retry must not debit the wallet again")
	require.Equal(t, 2.0, quota)
	require.Equal(t, 1, usage)
	require.Equal(t, 1, outcome)
	require.Equal(t, 1, dedup)
	require.Zero(t, pending)

	var raw []byte
	var input, output, upstreamOutput, rounds int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT ul.input_tokens, ul.output_tokens, ul.upstream_output_tokens,
		       uro.anti_truncation_rounds, uro.continuation_usage
		FROM usage_logs ul JOIN usage_response_outcomes uro ON uro.usage_log_id=ul.id
		WHERE ul.request_id=$1 AND ul.api_key_id=$2`, f.requestID, f.keyID).
		Scan(&input, &output, &upstreamOutput, &rounds, &raw))
	require.Equal(t, 210000, input)
	require.Equal(t, 550, output)
	require.Equal(t, 600, upstreamOutput)
	require.Equal(t, 1, rounds)
	var stored []service.ContinuationUsageAudit
	require.NoError(t, json.Unmarshal(raw, &stored))
	require.Equal(t, audit, stored, "PostgreSQL JSONB must retain distinct raw/customer counts, delivery, pricing tiers and recovery identities")

	// A later provenance-only retry must not clear the private round evidence.
	usageRepo := newUsageLogRepositoryWithSQL(client, integrationDB)
	require.NoError(t, usageRepo.UpsertResponseOutcome(ctx, &service.UsageLog{
		RequestID: f.requestID, APIKeyID: f.keyID, UserID: f.userID, AccountID: f.accountID,
		InputTokensSource: service.UsageTokenSourceUpstream,
	}))
	require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT continuation_usage FROM usage_response_outcomes WHERE request_id=$1 AND api_key_id=$2`, f.requestID, f.keyID).Scan(&raw))
	require.NoError(t, json.Unmarshal(raw, &stored))
	require.Equal(t, audit, stored)

	for name, public := range map[string]any{
		"outcome":   log.Outcome,
		"user DTO":  dto.UsageLogFromService(log),
		"admin DTO": dto.UsageLogFromServiceAdmin(log),
	} {
		encoded, err := json.Marshal(public)
		require.NoError(t, err, name)
		for _, private := range []string{"continuation_usage", "private-round-first", "private-round-second", "private-ledger-first", "private-ledger-second", "upstream-gemini-4-argon", strings.Repeat("a", 64), strings.Repeat("b", 64)} {
			require.NotContains(t, string(encoded), private, name+" must not expose internal provider-round accounting")
		}
	}
}
