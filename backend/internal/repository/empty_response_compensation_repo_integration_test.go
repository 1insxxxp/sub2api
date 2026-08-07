//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEmptyResponseCompensationRepositoryAppliesLedgerOnceConcurrently(t *testing.T) {
	ctx := context.Background()
	client := testEntClient(t)
	refund := 1.25

	user := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("empty-comp-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Balance:      10,
	})
	group := mustCreateGroup(t, client, &service.Group{Name: "empty-comp-" + uuid.NewString()})
	account := mustCreateAccount(t, client, &service.Account{
		Name:  "empty-comp-" + uuid.NewString(),
		Extra: map[string]any{"quota_used": 99.0},
	})
	apiKey := mustCreateApiKey(t, client, &service.APIKey{
		UserID:    user.ID,
		GroupID:   &group.ID,
		Key:       "sk-empty-comp-" + uuid.NewString(),
		Name:      "empty-comp",
		QuotaUsed: refund,
	})
	usage, err := client.UsageLog.Create().
		SetUserID(user.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetGroupID(group.ID).
		SetRequestID(uuid.NewString()).
		SetModel("test-model").
		SetActualCost(refund).
		SetBillingType(service.BillingTypeBalance).
		Save(ctx)
	require.NoError(t, err)
	claim, err := client.EmptyResponseClaim.Create().
		SetUsageLogID(usage.ID).
		SetUserID(user.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetGroupID(group.ID).
		SetStatus(service.EmptyResponseClaimApproved).
		SetOriginalActualCost(refund).
		Save(ctx)
	require.NoError(t, err)

	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM empty_response_claims WHERE id = $1", claim.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM usage_response_outcomes WHERE usage_log_id = $1", usage.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM usage_logs WHERE id = $1", usage.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM api_keys WHERE id = $1", apiKey.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM account_groups WHERE account_id = $1 OR group_id = $2", account.ID, group.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM accounts WHERE id = $1", account.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM groups WHERE id = $1", group.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", user.ID)
	})

	repo := newEmptyResponseCompensationRepository(integrationDB)
	results := make([]*service.EmptyResponseCompensationResult, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range results {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errs[index] = repo.Compensate(ctx, claim.ID)
		}(i)
	}
	wg.Wait()
	for _, callErr := range errs {
		require.NoError(t, callErr)
	}
	applied := 0
	for _, result := range results {
		require.NotNil(t, result)
		if result.Applied {
			applied++
		}
	}
	require.Equal(t, 1, applied)

	var balance, quotaUsed, actualCost, compensatedCost float64
	var claimStatus string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id = $1", user.ID).Scan(&balance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id = $1", apiKey.ID).Scan(&quotaUsed))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT actual_cost, compensated_cost FROM usage_logs WHERE id = $1", usage.ID).Scan(&actualCost, &compensatedCost))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status FROM empty_response_claims WHERE id = $1", claim.ID).Scan(&claimStatus))
	require.InDelta(t, 10+refund, balance, 0.000001)
	require.InDelta(t, 0, quotaUsed, 0.000001)
	require.InDelta(t, refund, actualCost, 0.000001)
	require.InDelta(t, refund, compensatedCost, 0.000001)
	require.Equal(t, service.EmptyResponseClaimCompensated, claimStatus)

	var accountExtra string
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT extra::text FROM accounts WHERE id = $1", account.ID).Scan(&accountExtra))
	require.Contains(t, accountExtra, "quota_used")
}
