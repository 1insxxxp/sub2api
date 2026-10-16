package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayPublicModelKeepsAliasSeparateFromRouteModel(t *testing.T) {
	ctx := WithGatewayPublicModel(t.Context(), "fast")
	require.Equal(t, "fast", gatewayPublicModel(ctx, "gpt-5.4"))
	require.Equal(t, "gpt-5.4", gatewayPublicModel(t.Context(), "gpt-5.4"))
}
