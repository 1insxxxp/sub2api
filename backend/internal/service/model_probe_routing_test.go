//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestModelProbeCompositeRouteDoesNotLosePrivateTarget(t *testing.T) {
	repo := &compositeRouteRepoStubForAdmin{routes: []CompositeModelRoute{{ID: 1, GroupID: 7, Enabled: true, MatchType: CompositeRouteMatchExact, PublicModel: "public-model", TargetPlatform: PlatformGemini, UpstreamModel: "private-model", Endpoint: CompositeRouteEndpointGemini}}}
	svc := &GatewayService{compositeResolver: NewCompositeRouteResolver(repo)}
	ctx, platform, model, endpoint, err := svc.ResolveModelProbeRoute(context.Background(), &Group{ID: 7, Platform: PlatformComposite}, "public-model")
	require.NoError(t, err)
	require.Equal(t, PlatformGemini, platform)
	require.Equal(t, "private-model", model)
	require.Equal(t, CompositeRouteEndpointGemini, endpoint)
	got, ok := ResolvedUpstreamModelFromContext(ctx)
	require.True(t, ok)
	require.Equal(t, "private-model", got)
}
func TestModelProbeConflictingEndpointRoutesAreUnsupported(t *testing.T) {
	repo := &compositeRouteRepoStubForAdmin{routes: []CompositeModelRoute{
		{ID: 1, GroupID: 7, Enabled: true, MatchType: CompositeRouteMatchExact, PublicModel: "public-model", TargetPlatform: PlatformGemini, UpstreamModel: "gemini-private", Endpoint: CompositeRouteEndpointGemini},
		{ID: 2, GroupID: 7, Enabled: true, MatchType: CompositeRouteMatchExact, PublicModel: "public-model", TargetPlatform: PlatformOpenAI, UpstreamModel: "gpt-private", Endpoint: CompositeRouteEndpointResponses}}}
	svc := &GatewayService{compositeResolver: NewCompositeRouteResolver(repo)}
	_, _, _, _, err := svc.ResolveModelProbeRoute(context.Background(), &Group{ID: 7, Platform: PlatformComposite}, "public-model")
	require.ErrorIs(t, err, ErrModelProbeUnsupported)
}
