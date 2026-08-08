//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type emptyResponseAdminRepoStub struct {
	claim     *EmptyResponseClaim
	reviewErr error
}

func (s *emptyResponseAdminRepoStub) List(context.Context, pagination.PaginationParams, EmptyResponseClaimListFilters) ([]EmptyResponseClaim, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}

func (s *emptyResponseAdminRepoStub) Review(context.Context, int64, string, int64, string) (*EmptyResponseClaim, error) {
	if s.reviewErr != nil {
		return nil, s.reviewErr
	}
	return s.claim, nil
}

type emptyResponseAdminCompensatorStub struct{}

func (emptyResponseAdminCompensatorStub) CompensateApprovedClaim(context.Context, int64) error {
	return nil
}

func TestEmptyResponseClaimAdminApproveReportsSubscriptionRefundSource(t *testing.T) {
	subscriptionID := int64(18)
	repo := &emptyResponseAdminRepoStub{claim: &EmptyResponseClaim{
		ID:                 12,
		SubscriptionID:     &subscriptionID,
		OriginalActualCost: 1.25,
	}}
	svc := NewEmptyResponseClaimAdminService(repo, emptyResponseAdminCompensatorStub{})

	claim, err := svc.Approve(context.Background(), 12, 9, "verified")

	require.NoError(t, err)
	require.Equal(t, EmptyResponseClaimCompensated, claim.Status)
	require.Zero(t, claim.BalanceRefund)
	require.Equal(t, 1.25, claim.SubscriptionRefund)
	require.Equal(t, 1.25, claim.APIKeyQuotaRefund)
}

func TestEmptyResponseClaimCompensationSourceDistinguishesAutomaticAndManual(t *testing.T) {
	automatic := &EmptyResponseClaim{Status: EmptyResponseClaimCompensated}
	manualReviewer := int64(9)
	manual := &EmptyResponseClaim{Status: EmptyResponseClaimCompensated, ReviewedBy: &manualReviewer}
	rejected := &EmptyResponseClaim{Status: EmptyResponseClaimRejected}

	require.Equal(t, "automatic", automatic.CompensationSource())
	require.Equal(t, "manual", manual.CompensationSource())
	require.Equal(t, "none", rejected.CompensationSource())
}

func TestEmptyResponseClaimBatchRedactsInternalFailureDetails(t *testing.T) {
	repo := &emptyResponseAdminRepoStub{reviewErr: errors.New("sql: password=should-not-leak")}
	svc := NewEmptyResponseClaimAdminService(repo, nil)

	result, err := svc.Batch(context.Background(), EmptyResponseClaimBatchInput{
		IDs:        []int64{12},
		Action:     EmptyResponseClaimApproved,
		ReviewerID: 9,
	})

	require.NoError(t, err)
	require.Equal(t, "review_failed", result.Failed[12])
	require.NotContains(t, result.Failed[12], "password")
}
