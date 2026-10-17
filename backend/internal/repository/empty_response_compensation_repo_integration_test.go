//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type emptyCompensationFixture struct{ userID, groupID, accountID, apiKeyID, usageID, claimID int64 }

func newEmptyCompensationFixture(t *testing.T, actual float64, amount *float64) emptyCompensationFixture {
	t.Helper()
	ctx := context.Background()
	client := testEntClient(t)
	user := mustCreateUser(t, client, &service.User{Email: "empty-comp-" + uuid.NewString() + "@example.com", PasswordHash: "hash", Balance: 10})
	group := mustCreateGroup(t, client, &service.Group{Name: "empty-comp-" + uuid.NewString()})
	require.NoError(t, client.Group.UpdateOneID(group.ID).SetEmptyResponseCompensationEnabled(true).Exec(ctx))
	account := mustCreateAccount(t, client, &service.Account{Name: "empty-comp-" + uuid.NewString(), Extra: map[string]any{"quota_used": 99.0}})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, GroupID: &group.ID, Key: "sk-empty-comp-" + uuid.NewString(), Name: "empty-comp", QuotaUsed: actual})
	usage, err := client.UsageLog.Create().SetUserID(user.ID).SetAPIKeyID(key.ID).SetAccountID(account.ID).SetGroupID(group.ID).SetRequestID(uuid.NewString()).SetModel("test-model").SetInputTokens(200000).SetActualCost(actual).SetBillingType(service.BillingTypeBalance).Save(ctx)
	require.NoError(t, err)
	claimBuilder := client.EmptyResponseClaim.Create().SetUsageLogID(usage.ID).SetUserID(user.ID).SetAPIKeyID(key.ID).SetAccountID(account.ID).SetGroupID(group.ID).SetStatus(service.EmptyResponseClaimApproved).SetOriginalActualCost(actual)
	if amount != nil {
		claimBuilder.SetCompensationAmount(*amount).SetRuleVersion(5)
	}
	claim, err := claimBuilder.Save(ctx)
	require.NoError(t, err)
	f := emptyCompensationFixture{user.ID, group.ID, account.ID, key.ID, usage.ID, claim.ID}
	t.Cleanup(func() {
		for _, item := range []struct {
			query string
			args  []any
		}{
			{"DELETE FROM redeem_codes WHERE code = $1", []any{fmt.Sprintf("EMPTY-COMP-%d", f.claimID)}},
			{"DELETE FROM empty_response_claims WHERE id = $1", []any{f.claimID}},
			{"DELETE FROM usage_response_outcomes WHERE usage_log_id = $1", []any{f.usageID}},
			{"DELETE FROM usage_logs WHERE id = $1", []any{f.usageID}},
			{"DELETE FROM user_subscriptions WHERE user_id = $1", []any{f.userID}},
			{"DELETE FROM api_keys WHERE id = $1", []any{f.apiKeyID}},
			{"DELETE FROM account_groups WHERE account_id = $1 OR group_id = $2", []any{f.accountID, f.groupID}},
			{"DELETE FROM accounts WHERE id = $1", []any{f.accountID}},
			{"DELETE FROM groups WHERE id = $1", []any{f.groupID}},
			{"DELETE FROM users WHERE id = $1", []any{f.userID}},
		} {
			_, _ = integrationDB.ExecContext(context.Background(), item.query, item.args...)
		}
	})
	return f
}

func TestEmptyResponseCompensationRepositoryAppliesLedgerOnceConcurrently(t *testing.T) {
	for _, tc := range []struct {
		name             string
		charge, expected float64
		legacy           bool
	}{
		{"legacy_approved_twelve", 12, 12, true},
		{"capped_twelve", 12, 5, false},
		{"full_two", 2, 2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			var amount *float64
			if !tc.legacy {
				amount = &tc.expected
			}
			f := newEmptyCompensationFixture(t, tc.charge, amount)
			repo := newEmptyResponseCompensationRepository(integrationDB)
			results := make([]*service.EmptyResponseCompensationResult, 2)
			errs := make([]error, 2)
			var wg sync.WaitGroup
			for i := range results {
				wg.Add(1)
				go func(index int) { defer wg.Done(); results[index], errs[index] = repo.Compensate(ctx, f.claimID) }(i)
			}
			wg.Wait()
			applied := 0
			for i, result := range results {
				require.NoError(t, errs[i])
				require.NotNil(t, result)
				require.InDelta(t, tc.expected, result.RefundAmount, 1e-10)
				if result.Applied {
					applied++
				}
			}
			require.Equal(t, 1, applied)
			// Later repairs to the usage record must not reprice a completed refund.
			_, err := integrationDB.ExecContext(ctx, "UPDATE usage_logs SET input_tokens=300000,actual_cost=$2 WHERE id=$1", f.usageID, tc.charge+1)
			require.NoError(t, err)
			result, err := repo.Compensate(ctx, f.claimID)
			require.NoError(t, err)
			require.False(t, result.Applied)
			require.InDelta(t, tc.expected, result.RefundAmount, 1e-10)
			var balance, quota, actual, compensated, original, balanceRefund, quotaRefund, history float64
			var status string
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", f.userID).Scan(&balance))
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id=$1", f.apiKeyID).Scan(&quota))
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT actual_cost,compensated_cost FROM usage_logs WHERE id=$1", f.usageID).Scan(&actual, &compensated))
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT status,original_actual_cost,balance_refund,api_key_quota_refund FROM empty_response_claims WHERE id=$1", f.claimID).Scan(&status, &original, &balanceRefund, &quotaRefund))
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT value FROM redeem_codes WHERE code=$1", fmt.Sprintf("EMPTY-COMP-%d", f.claimID)).Scan(&history))
			require.InDelta(t, 10+tc.expected, balance, 1e-10)
			require.InDelta(t, tc.charge-tc.expected, quota, 1e-10)
			require.Equal(t, tc.charge+1, actual)
			require.Equal(t, tc.charge, original)
			for _, value := range []float64{compensated, balanceRefund, quotaRefund, history} {
				require.InDelta(t, tc.expected, value, 1e-10)
			}
			require.Equal(t, service.EmptyResponseClaimCompensated, status)
			var accountExtra string
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT extra::text FROM accounts WHERE id=$1", f.accountID).Scan(&accountExtra))
			require.Contains(t, accountExtra, `"quota_used": 99`)
		})
	}
}

func TestEmptyResponseCompensationRepositoryPartialSubscriptionRefund(t *testing.T) {
	ctx := context.Background()
	amount := 5.0
	f := newEmptyCompensationFixture(t, 12, &amount)
	createdAt := time.Now().UTC().Add(-time.Hour)
	subscription, err := testEntClient(t).UserSubscription.Create().SetUserID(f.userID).SetGroupID(f.groupID).
		SetStartsAt(createdAt.Add(-time.Hour)).SetExpiresAt(createdAt.Add(24 * time.Hour)).
		SetDailyWindowStart(createdAt.Add(-time.Minute)).SetWeeklyWindowStart(createdAt.Add(-8 * 24 * time.Hour)).SetMonthlyWindowStart(createdAt.Add(-time.Minute)).
		SetDailyUsageUsd(10).SetWeeklyUsageUsd(14).SetMonthlyUsageUsd(15).Save(ctx)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE usage_logs SET subscription_id=$1,billing_type=$2,created_at=$3 WHERE id=$4", subscription.ID, service.BillingTypeSubscription, createdAt, f.usageID)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE empty_response_claims SET subscription_id=$1 WHERE id=$2", subscription.ID, f.claimID)
	require.NoError(t, err)
	result, err := newEmptyResponseCompensationRepository(integrationDB).Compensate(ctx, f.claimID)
	require.NoError(t, err)
	require.True(t, result.Applied)
	require.Equal(t, amount, result.RefundAmount)
	var daily, weekly, monthly, balance, balanceRefund, subscriptionRefund, quotaRefund, quota float64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT daily_usage_usd,weekly_usage_usd,monthly_usage_usd FROM user_subscriptions WHERE id=$1", subscription.ID).Scan(&daily, &weekly, &monthly))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", f.userID).Scan(&balance))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance_refund,subscription_refund,api_key_quota_refund FROM empty_response_claims WHERE id=$1", f.claimID).Scan(&balanceRefund, &subscriptionRefund, &quotaRefund))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id=$1", f.apiKeyID).Scan(&quota))
	require.InDelta(t, 10-amount, daily, 1e-10)
	require.Equal(t, 14.0, weekly)
	require.InDelta(t, 15-amount, monthly, 1e-10)
	require.Equal(t, 10.0, balance)
	require.Zero(t, balanceRefund)
	require.Equal(t, amount, subscriptionRefund)
	require.Equal(t, amount, quotaRefund)
	require.InDelta(t, 12-amount, quota, 1e-10)
	var histories int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM redeem_codes WHERE code=$1", fmt.Sprintf("EMPTY-COMP-%d", f.claimID)).Scan(&histories))
	require.Zero(t, histories)
}

func TestEmptyResponseClaimRepositoryCapSnapshotsAndHighInputCandidates(t *testing.T) {
	ctx := context.Background()
	f := newEmptyCompensationFixture(t, 12, nil)
	_, err := integrationDB.ExecContext(ctx, "UPDATE empty_response_claims SET reason_code=$1 WHERE id=$2", service.EmptyResponseReasonPureEmpty, f.claimID)
	require.NoError(t, err)
	client := testEntClient(t)
	newUsage := func(cost float64, input, cache int) int64 {
		usage, err := client.UsageLog.Create().SetUserID(f.userID).SetAPIKeyID(f.apiKeyID).SetAccountID(f.accountID).SetGroupID(f.groupID).SetRequestID(uuid.NewString()).SetModel("test-model").SetInputTokens(input).SetCacheReadTokens(cache).SetActualCost(cost).Save(ctx)
		require.NoError(t, err)
		return usage.ID
	}
	usageID := newUsage(12, 200000, 100000)
	smallID := newUsage(2, 300000, 100000)
	tinyID := newUsage(0.0000000001, 200000, 100000)
	zeroID := newUsage(0, 200000, 100000)
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM empty_response_claims WHERE usage_log_id=$1", usageID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM usage_logs WHERE id IN ($1,$2,$3,$4)", usageID, smallID, tinyID, zeroID)
	})
	repo := NewEmptyResponseClaimRepository(integrationDB)
	evaluation, err := repo.LoadEvaluation(ctx, f.userID, usageID)
	require.NoError(t, err)
	quote := service.QuoteEmptyResponseCompensation(evaluation.Usage)
	require.Equal(t, 5.0, quote.Amount)
	claim, created, err := repo.Create(ctx, &service.EmptyResponseClaimCreateInput{Evaluation: *evaluation, Decision: service.ClaimDecision{Status: service.EmptyResponseClaimApproved, ReasonCode: service.EmptyResponseReasonPureEmpty, RuleVersion: 5}, CompensationAmount: &quote.Amount})
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, 12.0, claim.OriginalActualCost)
	require.Equal(t, 5.0, claim.ExpectedRefund())
	owned, err := repo.GetByUsageLogID(ctx, f.userID, usageID)
	require.NoError(t, err)
	require.Equal(t, claim.ID, owned.ID)
	require.Equal(t, 5.0, owned.ExpectedRefund())
	admin, err := repo.GetAdminByID(ctx, claim.ID)
	require.NoError(t, err)
	require.Equal(t, 5.0, admin.ExpectedRefund())
	now := time.Now().Add(time.Second)
	candidates, page, err := repo.ListRecentEvaluations(ctx, f.userID, now.Add(-7*24*time.Hour), now, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Equal(t, int64(4), page.Total)
	found := map[int64]service.EmptyResponseRecentCandidate{}
	for _, candidate := range candidates {
		found[candidate.Evaluation.Usage.ID] = candidate
	}
	for _, id := range []int64{f.usageID, usageID, smallID, tinyID} {
		require.Contains(t, found, id, "input token count must not exclude otherwise eligible usage")
	}
	require.NotContains(t, found, zeroID)
	require.NotNil(t, found[usageID].ClaimCompensationAmount)
	require.Equal(t, 5.0, *found[usageID].ClaimCompensationAmount)
	require.Equal(t, 12.0, found[usageID].ClaimOriginalActualCost)
	require.Equal(t, 5, found[usageID].ClaimRuleVersion)
}

func TestEmptyResponseCompensationRepositoryRejectsUnsafeNewSnapshots(t *testing.T) {
	for _, name := range []string{"missing_snapshot", "above_cap"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			var amount *float64
			overCap := 6.0
			if name == "above_cap" {
				amount = &overCap
			}
			f := newEmptyCompensationFixture(t, 12, amount)
			_, err := integrationDB.ExecContext(ctx, "UPDATE empty_response_claims SET rule_version=5 WHERE id=$1", f.claimID)
			require.NoError(t, err)
			result, err := newEmptyResponseCompensationRepository(integrationDB).Compensate(ctx, f.claimID)
			require.ErrorIs(t, err, service.ErrEmptyResponseCompensationInvalidState)
			require.Nil(t, result)
			var balance, quota, compensated float64
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT balance FROM users WHERE id=$1", f.userID).Scan(&balance))
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT quota_used FROM api_keys WHERE id=$1", f.apiKeyID).Scan(&quota))
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT compensated_cost FROM usage_logs WHERE id=$1", f.usageID).Scan(&compensated))
			require.Equal(t, 10.0, balance)
			require.Equal(t, 12.0, quota)
			require.Zero(t, compensated)
			var histories int
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM redeem_codes WHERE code=$1", fmt.Sprintf("EMPTY-COMP-%d", f.claimID)).Scan(&histories))
			require.Zero(t, histories)
		})
	}
}

func TestEmptyResponseClaimRepositoryDailyLimitSerializesDistinctConcurrentRequests(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	fixture := newEmptyCompensationFixture(t, 12, nil)
	ids := make([]int64, 0, 16)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, id := range ids {
			_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM empty_response_claims WHERE usage_log_id=$1", id)
			_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM usage_logs WHERE id=$1", id)
		}
	})
	inputs := make([]*service.EmptyResponseClaimCreateInput, 0, 16)
	for i := 0; i < 16; i++ {
		usage, err := testEntClient(t).UsageLog.Create().SetUserID(fixture.userID).SetAPIKeyID(fixture.apiKeyID).SetAccountID(fixture.accountID).SetGroupID(fixture.groupID).SetRequestID(uuid.NewString()).SetModel("probe").SetActualCost(2).Save(ctx)
		require.NoError(t, err)
		ids = append(ids, usage.ID)
		amount := 2.0
		inputs = append(inputs, &service.EmptyResponseClaimCreateInput{Evaluation: service.EmptyResponseClaimEvaluation{Usage: service.UsageLog{ID: usage.ID, UserID: fixture.userID, APIKeyID: fixture.apiKeyID, AccountID: fixture.accountID, GroupID: &fixture.groupID, ActualCost: 2}}, Decision: service.ClaimDecision{Status: service.EmptyResponseClaimApproved, ReasonCode: service.EmptyResponseReasonPureEmpty, RuleVersion: 5}, CompensationAmount: &amount})
	}
	// Hold the same advisory key until all contenders have entered their statement.
	blocker, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	_, err = blocker.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(($1::bigint)::text || ':' || to_char(NOW() AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD'), 0))", fixture.userID)
	require.NoError(t, err)
	var wg sync.WaitGroup
	errs := make([]error, len(inputs))
	for i, input := range inputs {
		wg.Add(1)
		go func(index int, input *service.EmptyResponseClaimCreateInput) {
			defer wg.Done()
			_, _, errs[index] = NewEmptyResponseClaimRepository(integrationDB).Create(ctx, input)
		}(i, input)
	}
	waiting := 0
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM pg_stat_activity WHERE wait_event='advisory'").Scan(&waiting))
		if waiting == len(inputs) {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(t, blocker.Commit())
	wg.Wait()
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM empty_response_claims WHERE user_id=$1", fixture.userID).Scan(&count))
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	t.Logf("blocked=%d concurrent_successes=%d final_claim_count=%d daily_limit=%d", waiting, successes, count, service.EmptyResponseClaimDailyLimit)
	require.Equal(t, len(inputs), waiting, "all contenders must wait on the same daily key")
	require.Equal(t, service.EmptyResponseClaimDailyLimit, count)
	require.Equal(t, service.EmptyResponseClaimDailyLimit-1, successes)
	var existingInput *service.EmptyResponseClaimCreateInput
	for i, err := range errs {
		if err != nil {
			require.ErrorIs(t, err, service.ErrEmptyResponseClaimDailyLimitExceeded)
		} else if existingInput == nil {
			existingInput = inputs[i]
		}
	}
	require.NotNil(t, existingInput)
	existing, err := NewEmptyResponseClaimRepository(integrationDB).GetByUsageLogID(ctx, fixture.userID, existingInput.Evaluation.Usage.ID)
	require.NoError(t, err)
	require.NotNil(t, existing)
	// Duplicates must still resolve the frozen claim when the daily limit is full.
	duplicateErrors := make([]error, 4)
	for i := range duplicateErrors {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			claim, created, callErr := NewEmptyResponseClaimRepository(integrationDB).Create(ctx, existingInput)
			if callErr == nil && (created || claim.ID != existing.ID || claim.ExpectedRefund() != 2) {
				callErr = fmt.Errorf("duplicate changed frozen claim: created=%v claim=%+v", created, claim)
			}
			duplicateErrors[index] = callErr
		}(i)
	}
	wg.Wait()
	for _, callErr := range duplicateErrors {
		require.NoError(t, callErr)
	}
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT count(*) FROM empty_response_claims WHERE user_id=$1", fixture.userID).Scan(&count))
	require.Equal(t, service.EmptyResponseClaimDailyLimit, count)
	amount := 5.0
	legacyInput := &service.EmptyResponseClaimCreateInput{Evaluation: service.EmptyResponseClaimEvaluation{Usage: service.UsageLog{ID: fixture.usageID, UserID: fixture.userID, APIKeyID: fixture.apiKeyID, AccountID: fixture.accountID, ActualCost: 12}}, Decision: service.ClaimDecision{Status: service.EmptyResponseClaimApproved, ReasonCode: service.EmptyResponseReasonPureEmpty, RuleVersion: 5}, CompensationAmount: &amount}
	legacy, created, err := NewEmptyResponseClaimRepository(integrationDB).Create(ctx, legacyInput)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, fixture.claimID, legacy.ID)
	require.Equal(t, 12.0, legacy.ExpectedRefund())
}
