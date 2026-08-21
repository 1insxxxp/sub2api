package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProjectUsageLogCompensationDoesNotExposeNewClaimActionInUsageList(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 21, 10, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	log := &UsageLog{
		ID:         100,
		ActualCost: 1.25,
		CreatedAt:  now.Add(-time.Hour),
		Outcome: &ResponseOutcome{
			HTTPStatus:       200,
			UpstreamStatus:   200,
			StreamCompleted: true,
			CollectorVersion: 1,
		},
	}

	projectUsageLogCompensation(log, now)

	require.False(t, log.CompensationEligible)
	require.Equal(t, UsageCompensationUnavailable, log.CompensationEligibility)
}
