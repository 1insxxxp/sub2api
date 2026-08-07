//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type emptyResponseMetricsRepoStub struct {
	metrics *EmptyResponseClaimMetrics
	start   time.Time
	end     time.Time
}

func (s *emptyResponseMetricsRepoStub) GetEmptyResponseClaimMetrics(_ context.Context, start, end time.Time) (*EmptyResponseClaimMetrics, error) {
	s.start, s.end = start, end
	return s.metrics, nil
}

func TestEmptyResponseClaimMetricsServiceReturnsAggregatedRatesAndRankings(t *testing.T) {
	start := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	want := &EmptyResponseClaimMetrics{
		TotalChargedRequests: 100,
		TotalClaims:          4,
		CompensatedClaims:    3,
		TotalRefundAmount:    2.5,
		ByGroup:              []EmptyResponseClaimMetricDimension{{ID: 7, Name: "cc", ChargedRequests: 80, Claims: 4, RefundAmount: 2.5}},
		ByAccount:            []EmptyResponseClaimMetricDimension{{ID: 9, Name: "pool", ChargedRequests: 10, Claims: 2, RefundAmount: 1}},
	}
	repo := &emptyResponseMetricsRepoStub{metrics: want}
	svc := NewEmptyResponseClaimMetricsService(repo)

	got, err := svc.Get(context.Background(), start, end)
	require.NoError(t, err)
	require.Same(t, want, got)
	require.InDelta(t, 0.04, got.EmptyResponseRate, 1e-12)
	require.InDelta(t, 0.05, got.ByGroup[0].EmptyResponseRate, 1e-12)
	require.InDelta(t, 0.2, got.ByAccount[0].EmptyResponseRate, 1e-12)
	require.Equal(t, start, repo.start)
	require.Equal(t, end, repo.end)
}

func TestEmptyResponseClaimMetricsServiceRejectsInvalidRange(t *testing.T) {
	svc := NewEmptyResponseClaimMetricsService(&emptyResponseMetricsRepoStub{})
	_, err := svc.Get(context.Background(), time.Now(), time.Now().Add(-time.Hour))
	require.ErrorIs(t, err, ErrEmptyResponseClaimInvalidInput)
}
