package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestResolveGroupModelAlias(t *testing.T) {
	group := &service.Group{ModelAliases: []domain.ModelAlias{{Alias: "fast", UpstreamModel: "claude-sonnet-4-6"}}}
	model, found := resolveGroupModelAlias(group, "FAST")
	require.True(t, found)
	require.Equal(t, "claude-sonnet-4-6", model)

	model, found = resolveGroupModelAlias(group, "other")
	require.False(t, found)
	require.Equal(t, "other", model)
	model, found = resolveGroupModelAlias(nil, "other")
	require.False(t, found)
	require.Equal(t, "other", model)
}

func TestResolveGroupModelRouteKeepsPublicModelAndResolvesTarget(t *testing.T) {
	group := &service.Group{ModelAliases: []domain.ModelAlias{{Alias: "fast", UpstreamModel: "gpt-5.4"}}}
	publicModel, routeModel := resolveGroupModelRoute(group, "fast")
	require.Equal(t, "fast", publicModel)
	require.Equal(t, "gpt-5.4", routeModel)
}

func TestAppendAvailableGroupModelAliasesRequiresAvailableTarget(t *testing.T) {
	group := &service.Group{ModelAliases: []domain.ModelAlias{
		{Alias: "sonnet-fast", UpstreamModel: "claude-sonnet-4-6"},
		{Alias: "missing", UpstreamModel: "claude-opus-4-6"},
		{Alias: "already-listed", UpstreamModel: "claude-sonnet-4-6"},
	}}
	got := appendAvailableGroupModelAliases([]string{"claude-sonnet-4-6", "already-listed"}, group)
	require.Equal(t, []string{"claude-sonnet-4-6", "already-listed", "sonnet-fast"}, got)
	require.Equal(t, []string{"claude-sonnet-4-6"}, appendAvailableGroupModelAliases([]string{"claude-sonnet-4-6"}, nil))
}

func TestAppendAvailableGroupModelAliasesRespectsEffectiveAllowlist(t *testing.T) {
	group := &service.Group{
		ModelAliases:   []domain.ModelAlias{{Alias: "sonnet-fast", UpstreamModel: "claude-sonnet-4-6"}},
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"sonnet-fast"}},
	}
	models := appendAvailableGroupModelAliases([]string{"claude-sonnet-4-6"}, group)
	require.Equal(t, []string{"sonnet-fast"}, group.EffectiveModelAllowlist().FilterForListing(models))

	group.ModelAllowlist.Models = []string{"claude-sonnet-4-6"}
	require.Equal(t, []string{"claude-sonnet-4-6"}, appendAvailableGroupModelAliases([]string{"claude-sonnet-4-6"}, group))
}
