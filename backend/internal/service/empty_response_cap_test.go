//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestQuoteEmptyResponseCompensationCapsActualCharge(t *testing.T) {
	for _, tc := range []struct {
		name string
		cost float64
		want float64
	}{
		{"below cap", 2, 2},
		{"at cap", 5, 5},
		{"above cap", 12, 5},
		{"large charge", 12345, 5},
		{"ledger precision", 1.23456789129, 1.2345678912},
		{"smallest payable", 0.0000000001, 0.0000000001},
	} {
		t.Run(tc.name, func(t *testing.T) {
			quote := QuoteEmptyResponseCompensation(UsageLog{ActualCost: tc.cost})
			require.Equal(t, tc.want, quote.Amount)
			require.LessOrEqual(t, quote.Amount, tc.cost)
		})
	}
}

func TestQuoteEmptyResponseCompensationDoesNotDependOnTokens(t *testing.T) {
	for _, usage := range []UsageLog{
		{ActualCost: 12},
		{InputTokens: 150000, ActualCost: 12},
		{InputTokens: 40000, CacheCreationTokens: 10000, CacheReadTokens: 100000, ActualCost: 12},
		{InputTokens: math.MaxInt, CacheReadTokens: math.MaxInt, ActualCost: 12},
		{InputTokens: -100, CacheReadTokens: -100, ActualCost: 12},
	} {
		require.Equal(t, 5.0, QuoteEmptyResponseCompensation(usage).Amount)
	}
}

func TestQuoteEmptyResponseCompensationRejectsUnpayableCosts(t *testing.T) {
	for _, cost := range []float64{0, -1, math.NaN(), math.Inf(1), math.Inf(-1), 0.00000000001} {
		require.Zero(t, QuoteEmptyResponseCompensation(UsageLog{ActualCost: cost}).Amount)
	}
}

func TestEmptyResponseClaimAcceptsHighInputWithoutTierLimit(t *testing.T) {
	now := time.Now()
	for _, usage := range []UsageLog{
		{InputTokens: 150000, ActualCost: 12, CreatedAt: now},
		{InputTokens: 40000, CacheCreationTokens: 10000, CacheReadTokens: 100000, ActualCost: 12, CreatedAt: now},
	} {
		decision := EvaluateEmptyResponseClaim(now, usage, nil, Group{EmptyResponseCompensationEnabled: true}, 0)
		require.Equal(t, EmptyResponseClaimApproved, decision.Status)
		require.Equal(t, EmptyResponseReasonLowOutput, decision.ReasonCode)
	}
}

func TestEmptyResponseClaimZeroRefundDoesNotCreateClaim(t *testing.T) {
	now := time.Now()
	repo := &emptyResponseClaimRepoStub{
		evaluation: &EmptyResponseClaimEvaluation{
			Usage: UsageLog{ID: 100, UserID: 7, ActualCost: 0.00000000001, CreatedAt: now},
			Group: Group{EmptyResponseCompensationEnabled: true},
		},
	}
	svc := NewEmptyResponseClaimService(repo, nil)
	svc.now = func() time.Time { return now }
	_, err := svc.Submit(context.Background(), EmptyResponseClaimSubmitInput{UserID: 7, UsageLogID: 100})
	require.ErrorIs(t, err, ErrEmptyResponseClaimNotFound)
	require.Nil(t, repo.createIn)
}

func TestEmptyResponseClaimFrozenRefundAndLegacyPolicy(t *testing.T) {
	amount := 5.0
	claim := &EmptyResponseClaim{OriginalActualCost: 12, CompensationAmount: &amount, RuleVersion: 5}
	require.Equal(t, amount, claim.ExpectedRefund())
	legacy := &EmptyResponseClaim{OriginalActualCost: 12, RuleVersion: 4}
	require.Equal(t, 12.0, legacy.ExpectedRefund(), "old approved amounts are not capped retroactively")
	missingSnapshot := &EmptyResponseClaim{OriginalActualCost: 12, RuleVersion: 5}
	require.Zero(t, missingSnapshot.ExpectedRefund(), "new claims cannot fall back to a full refund")
}

func TestEmptyResponseClaimServiceSnapshotsAndReportsCappedRefund(t *testing.T) {
	for _, billing := range []int8{BillingTypeBalance, BillingTypeSubscription} {
		now := time.Now()
		amount := 5.0
		repo := &emptyResponseClaimRepoStub{
			evaluation: &EmptyResponseClaimEvaluation{
				Usage: UsageLog{ID: 100, UserID: 7, InputTokens: 200000, ActualCost: 12, BillingType: billing, CreatedAt: now},
				Group: Group{EmptyResponseCompensationEnabled: true},
			},
			claim:   &EmptyResponseClaim{ID: 200, OriginalActualCost: 12, CompensationAmount: &amount, RuleVersion: 5},
			created: true,
		}
		svc := NewEmptyResponseClaimService(repo, &emptyResponseClaimCompensatorStub{})
		svc.now = func() time.Time { return now }
		claim, err := svc.Submit(context.Background(), EmptyResponseClaimSubmitInput{UserID: 7, UsageLogID: 100})
		require.NoError(t, err)
		require.NotNil(t, repo.createIn.CompensationAmount)
		require.Equal(t, amount, *repo.createIn.CompensationAmount)
		require.Equal(t, 12.0, repo.createIn.OriginalActualCost)
		require.Equal(t, amount, claim.APIKeyQuotaRefund)
		require.Equal(t, amount, claim.BalanceRefund+claim.SubscriptionRefund)
		if billing == BillingTypeSubscription {
			require.Zero(t, claim.BalanceRefund)
		} else {
			require.Zero(t, claim.SubscriptionRefund)
		}
	}
}

func TestEmptyResponseClaimAdminCappedRefundSurvivesDetailReloadFailure(t *testing.T) {
	amount := 5.0
	repo := &emptyResponseAdminRepoStub{
		claim:     &EmptyResponseClaim{ID: 200, OriginalActualCost: 12, CompensationAmount: &amount, RuleVersion: 5},
		detailErr: errors.New("temporary read failure"),
	}
	svc := NewEmptyResponseClaimAdminService(repo, emptyResponseAdminCompensatorStub{})
	claim, err := svc.Approve(context.Background(), 200, 9, "verified")
	require.NoError(t, err)
	require.Equal(t, amount, claim.BalanceRefund)
	require.Equal(t, amount, claim.APIKeyQuotaRefund)
}

func TestEmptyResponseRecordUsesFrozenQuoteAndKeepsLegacyHistory(t *testing.T) {
	now := time.Now()
	usage := UsageLog{InputTokens: 200000, ActualCost: 12, CreatedAt: now}
	newCandidate := EmptyResponseRecentCandidate{Evaluation: EmptyResponseClaimEvaluation{Usage: usage, Group: Group{EmptyResponseCompensationEnabled: true}}}
	record, include, claimable := emptyResponseRecordFromCandidate(now, newCandidate)
	require.True(t, include)
	require.True(t, claimable)
	require.Equal(t, 5.0, record.EstimatedRefund)
	claimID := int64(300)
	legacy := newCandidate
	legacy.ClaimID = &claimID
	legacy.ClaimStatus = EmptyResponseClaimCompensated
	legacy.ClaimReasonCode = EmptyResponseReasonLowOutput
	legacy.ClaimOriginalActualCost = 8
	legacy.ClaimRuleVersion = 4
	legacy.RefundedAmount = 8
	record, include, claimable = emptyResponseRecordFromCandidate(now, legacy)
	require.True(t, include)
	require.False(t, claimable)
	require.Equal(t, 8.0, record.EstimatedRefund, "legacy history uses its original charge even if usage was repaired")
	frozen := 3.0
	legacy.ClaimCompensationAmount = &frozen
	legacy.ClaimRuleVersion = 5
	record, _, _ = emptyResponseRecordFromCandidate(now, legacy)
	require.Equal(t, frozen, record.EstimatedRefund)
	legacy.ClaimCompensationAmount = nil
	record, _, _ = emptyResponseRecordFromCandidate(now, legacy)
	require.Zero(t, record.EstimatedRefund, "new claims without a snapshot fail closed")
}

func TestUsageCompensationProjectionIncludesServerQuote(t *testing.T) {
	now := time.Now()
	log := &UsageLog{ActualCost: 12, InputTokens: 200000, CreatedAt: now, Group: &Group{EmptyResponseCompensationEnabled: true}}
	projectUsageLogCompensation(log, now)
	require.True(t, log.CompensationEligible)
	require.Equal(t, 5.0, log.CompensationEstimatedRefund)
	log.ActualCost = 2
	projectUsageLogCompensation(log, now)
	require.True(t, log.CompensationEligible)
	require.Equal(t, 2.0, log.CompensationEstimatedRefund)
}

type frozenClaimLookupStub struct {
	*emptyResponseClaimRepoStub
	existing *EmptyResponseClaim
}

func (s *frozenClaimLookupStub) GetByUsageLogID(context.Context, int64, int64) (*EmptyResponseClaim, error) {
	return s.existing, nil
}

func TestEmptyResponseClaimRetryHonorsFrozenPolicyBeforeNewEligibility(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		now := time.Now()
		claim := &EmptyResponseClaim{ID: 200, Status: EmptyResponseClaimApproved, OriginalActualCost: 12, RuleVersion: 4}
		want := 12.0
		if !legacy {
			want = 5
			claim.RuleVersion = 5
			claim.CompensationAmount = &want
		}
		repo := &frozenClaimLookupStub{
			emptyResponseClaimRepoStub: &emptyResponseClaimRepoStub{
				evaluation: &EmptyResponseClaimEvaluation{
					Usage: UsageLog{ID: 100, UserID: 7, InputTokens: 150000, ActualCost: 12, CreatedAt: now.Add(-8 * 24 * time.Hour)},
					Group: Group{},
				},
				dailyCount: 15,
			},
			existing: claim,
		}
		compensator := &emptyResponseClaimCompensatorStub{}
		svc := NewEmptyResponseClaimService(repo, compensator)
		got, err := svc.Submit(context.Background(), EmptyResponseClaimSubmitInput{UserID: 7, UsageLogID: 100})
		require.NoError(t, err)
		require.Same(t, claim, got)
		require.Equal(t, int64(200), compensator.claimID)
		require.Equal(t, want, got.BalanceRefund)
		require.Nil(t, repo.createIn, "retry must not consume another daily slot")
	}
}
