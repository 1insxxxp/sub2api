//go:build unit

package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestGatewayForwardMayFailoverAfterWriteAllowsSafeTimeout(t *testing.T) {
	tests := []struct {
		name            string
		before          int
		current         int
		failoverErr     *service.UpstreamFailoverError
		wantMayFailover bool
	}{
		{
			name:            "nothing written",
			before:          0,
			current:         0,
			wantMayFailover: true,
		},
		{
			name:    "non semantic keepalive before timeout",
			before:  0,
			current: 12,
			failoverErr: &service.UpstreamFailoverError{
				SafeToFailoverAfterWrite: true,
			},
			wantMayFailover: true,
		},
		{
			name:            "written bytes without safe failover marker",
			before:          0,
			current:         12,
			failoverErr:     &service.UpstreamFailoverError{},
			wantMayFailover: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantMayFailover, gatewayForwardMayFailoverAfterWrite(tt.before, tt.current, tt.failoverErr))
		})
	}
}
