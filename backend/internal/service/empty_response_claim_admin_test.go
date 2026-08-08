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
	claim        *EmptyResponseClaim
	detailClaim  *EmptyResponseClaim
	reviewErr    error
	detailErr    error
	detailLoaded bool
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

func (s *emptyResponseAdminRepoStub) GetAdminByID(context.Context, int64) (*EmptyResponseClaim, error) {
	s.detailLoaded = true
	if s.detailErr != nil {
		return nil, s.detailErr
	}
	return s.detailClaim, nil
}

type emptyResponseAdminCompensatorStub struct{}

func (emptyResponseAdminCompensatorStub) CompensateApprovedClaim(context.Context, int64) error {
	return nil
}

func TestEmptyResponseClaimAdminApproveReportsSubscriptionRefundSource(t *testing.T) {
	subscriptionID := int64(18)
	detail := &EmptyResponseClaim{
		ID:                 12,
		Model:              "claude-opus-4-6",
		UserEmail:          "user@example.com",
		RequestID:          "req-empty-response",
		SubscriptionID:     &subscriptionID,
		OriginalActualCost: 1.25,
		Status:             EmptyResponseClaimCompensated,
		SubscriptionRefund: 1.25,
		APIKeyQuotaRefund:  1.25,
	}
	repo := &emptyResponseAdminRepoStub{claim: &EmptyResponseClaim{
		ID:                 12,
		SubscriptionID:     &subscriptionID,
		OriginalActualCost: 1.25,
	}, detailClaim: detail}
	svc := NewEmptyResponseClaimAdminService(repo, emptyResponseAdminCompensatorStub{})

	claim, err := svc.Approve(context.Background(), 12, 9, "verified")

	require.NoError(t, err)
	require.True(t, repo.detailLoaded)
	require.Same(t, detail, claim)
	require.Equal(t, EmptyResponseClaimCompensated, claim.Status)
	require.Equal(t, "claude-opus-4-6", claim.Model)
	require.Equal(t, "user@example.com", claim.UserEmail)
	require.Equal(t, "req-empty-response", claim.RequestID)
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

func TestEmptyResponseClaimAdminApproveDoesNotReportFailureWhenDetailReloadFails(t *testing.T) {
	reviewed := &EmptyResponseClaim{ID: 12, OriginalActualCost: 1.25}
	repo := &emptyResponseAdminRepoStub{
		claim:     reviewed,
		detailErr: errors.New("temporary read failure"),
	}
	svc := NewEmptyResponseClaimAdminService(repo, emptyResponseAdminCompensatorStub{})

	claim, err := svc.Approve(context.Background(), 12, 9, "verified")

	require.NoError(t, err)
	require.True(t, repo.detailLoaded)
	require.Same(t, reviewed, claim)
	require.Equal(t, EmptyResponseClaimCompensated, claim.Status)
	require.Equal(t, 1.25, claim.BalanceRefund)
	require.Equal(t, 1.25, claim.APIKeyQuotaRefund)
}
