//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReserveBatchImageBalanceHold_PersistsZeroValueMarker(t *testing.T) {
	holdAmount := 0.0
	apiKeyID := int64(42)
	job := &BatchImageJob{BatchID: "batch-free", UserID: 7, APIKeyID: &apiKeyID, HoldAmount: &holdAmount}
	repo := &fakeBatchImageBillingRepo{}

	err := reserveBatchImageBalanceHold(context.Background(), repo, job, "payload")
	require.NoError(t, err)
	require.Len(t, repo.reserves, 1)
	require.Zero(t, repo.reserves[0].HoldAmount)
	require.Equal(t, BatchImageHoldRequestID(job.BatchID), repo.reserves[0].RequestID)
}

func TestReleaseBatchImageBalanceHold_PersistsZeroValueTerminalMarker(t *testing.T) {
	holdAmount := 0.0
	apiKeyID := int64(42)
	job := &BatchImageJob{BatchID: "batch-free-release", UserID: 7, APIKeyID: &apiKeyID, HoldAmount: &holdAmount}
	repo := &fakeBatchImageBillingRepo{}

	err := releaseBatchImageBalanceHold(context.Background(), repo, job, "payload")
	require.NoError(t, err)
	require.Len(t, repo.releases, 1)
	require.Zero(t, repo.releases[0].HoldAmount)
	require.Equal(t, BatchImageReleaseRequestID(job.BatchID), repo.releases[0].RequestID)
}
