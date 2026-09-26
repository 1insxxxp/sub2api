package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestOpsServiceSLAErrorSummaryRequiresMonitoring(t *testing.T) {
	result, err := NewOpsService(nil, nil, &config.Config{Ops: config.OpsConfig{Enabled: false}}, nil, nil, nil, nil, nil, nil, nil, nil).GetSLAErrorSummary(context.Background(), nil)
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrOpsDisabled)
}

func TestOpsServiceSLAErrorSummaryNilRepositoryReturnsEmpty(t *testing.T) {
	result, err := NewOpsService(nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).GetSLAErrorSummary(context.Background(), nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Empty(t, result.Groups)
}

func TestOpsServiceSLAErrorSummaryPassesThroughRepositoryError(t *testing.T) {
	want := errors.New("sla summary query failed")
	repo := &opsRepoMock{GetSLAErrorSummaryFn: func(context.Context, *OpsErrorLogFilter) (*OpsSLAErrorSummary, error) {
		return nil, want
	}}
	result, err := NewOpsService(repo, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil).GetSLAErrorSummary(context.Background(), &OpsErrorLogFilter{})
	require.Nil(t, result)
	require.ErrorIs(t, err, want)
}
