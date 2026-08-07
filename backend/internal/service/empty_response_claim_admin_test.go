//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type emptyResponseAdminRepoStub struct {
	claim *EmptyResponseClaim
}

func (s *emptyResponseAdminRepoStub) List(context.Context, pagination.PaginationParams, EmptyResponseClaimListFilters) ([]EmptyResponseClaim, *pagination.PaginationResult, error) {
	return nil, &pagination.PaginationResult{}, nil
}

func (s *emptyResponseAdminRepoStub) Review(context.Context, int64, string, int64, string) (*EmptyResponseClaim, error) {
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
