package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelProbeCancellationSkipsBillingDrain(t *testing.T) {
	for _, probe := range []bool{false, true} {
		ctx := context.Background()
		if probe {
			ctx = WithModelProbeExecution(ctx)
		}
		ctx, cancel := context.WithCancel(ctx)
		upstream, release := detachUpstreamContext(ctx, time.Minute)
		cancel()
		if probe {
			select {
			case <-upstream.Done():
			case <-time.After(50 * time.Millisecond):
				t.Error("stopped probe retained a detached upstream")
			}
		} else {
			require.NoError(t, upstream.Err(), "customer usage drain must stay unchanged")
		}
		release()
	}
}
