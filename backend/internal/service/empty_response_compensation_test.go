//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type emptyResponseCompensationRepoStub struct {
	result *EmptyResponseCompensationResult
	err    error
	steps  *[]string
}

func (s *emptyResponseCompensationRepoStub) Compensate(_ context.Context, _ int64) (*EmptyResponseCompensationResult, error) {
	*s.steps = append(*s.steps, "commit")
	return s.result, s.err
}

type emptyResponseCompensationCacheStub struct {
	steps *[]string
	err   error
}

func (s *emptyResponseCompensationCacheStub) InvalidateUserBalance(_ context.Context, _ int64) error {
	*s.steps = append(*s.steps, "balance_cache")
	return s.err
}

func (s *emptyResponseCompensationCacheStub) InvalidateSubscription(_ context.Context, _, _ int64) error {
	*s.steps = append(*s.steps, "subscription_cache")
	return s.err
}

type emptyResponseCompensationAuthStub struct {
	steps *[]string
}

func (s *emptyResponseCompensationAuthStub) InvalidateAuthCacheByKey(_ context.Context, _ string) {
	*s.steps = append(*s.steps, "auth_cache")
}

func (s *emptyResponseCompensationAuthStub) InvalidateAuthCacheByUserID(context.Context, int64)  {}
func (s *emptyResponseCompensationAuthStub) InvalidateAuthCacheByGroupID(context.Context, int64) {}

func TestEmptyResponseCompensationInvalidatesCachesOnlyAfterCommit(t *testing.T) {
	steps := []string{}
	groupID := int64(9)
	subscriptionID := int64(10)
	repo := &emptyResponseCompensationRepoStub{steps: &steps, result: &EmptyResponseCompensationResult{
		Applied:        true,
		UserID:         7,
		APIKeyID:       8,
		APIKey:         "sk-test",
		GroupID:        &groupID,
		SubscriptionID: &subscriptionID,
	}}
	svc := NewEmptyResponseCompensationService(
		repo,
		&emptyResponseCompensationCacheStub{steps: &steps},
		&emptyResponseCompensationAuthStub{steps: &steps},
	)

	err := svc.CompensateApprovedClaim(context.Background(), 100)

	require.NoError(t, err)
	require.Equal(t, []string{"commit", "balance_cache", "subscription_cache", "auth_cache"}, steps)
}

func TestEmptyResponseCompensationDoesNotInvalidateBeforeFailedTransaction(t *testing.T) {
	steps := []string{}
	repoErr := errors.New("transaction failed")
	svc := NewEmptyResponseCompensationService(
		&emptyResponseCompensationRepoStub{steps: &steps, err: repoErr},
		&emptyResponseCompensationCacheStub{steps: &steps},
		&emptyResponseCompensationAuthStub{steps: &steps},
	)

	err := svc.CompensateApprovedClaim(context.Background(), 100)

	require.ErrorIs(t, err, repoErr)
	require.Equal(t, []string{"commit"}, steps)
}
