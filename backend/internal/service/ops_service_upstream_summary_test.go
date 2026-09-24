package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsServiceUpstreamErrorSummaryNilRepositoryReturnsEmpty(t *testing.T) {
	result, err := NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).GetUpstreamErrorSummary(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.Groups)
}

func TestOpsServiceUpstreamErrorSummaryRequiresMonitoring(t *testing.T) {
	result, err := NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: false}}, nil, nil, nil, nil, nil, nil, nil, nil).GetUpstreamErrorSummary(context.Background(), nil)
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrOpsDisabled)
}

func TestOpsServiceUpstreamErrorSummaryPassesThroughRepositoryError(t *testing.T) {
	want := errors.New("summary query failed")
	repo := &opsRepoMock{GetUpstreamErrorSummaryFn: func(context.Context, *OpsErrorLogFilter) (*OpsUpstreamErrorSummary, error) { return nil, want }}
	result, err := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).GetUpstreamErrorSummary(context.Background(), &OpsErrorLogFilter{})
	require.Nil(t, result)
	require.ErrorIs(t, err, want)
}
