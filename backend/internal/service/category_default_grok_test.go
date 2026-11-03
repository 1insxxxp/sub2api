package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/stretchr/testify/require"
)

func categoryDefaultGrokSource(opts xai.ModelMappingOptions) CategoryDefaultSourceSnapshot {
	source := categoryDefaultSource(1, "grok", nil)
	source.Group.Platform = PlatformGrok
	source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"grok", "grok-4.6"}}
	source.Accounts[0].Platform = PlatformGrok
	source.GrokMappingOptions = &opts
	return source
}

func TestCategoryDefaultGrokCatalogUsesFrozenLocalOptions(t *testing.T) {
	before := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(before) })
	first := xai.ModelMappingOptions{DefaultText: "grok-4.6"}
	second := xai.ModelMappingOptions{DefaultText: "grok-4.7", EnableCrossClientMap: true}
	xai.SetRuntimeModelMappingOptions(first)
	source := categoryDefaultGrokSource(first)
	published, err := CategoryDefaultSourceDeclarations(source)
	require.NoError(t, err)
	xai.SetRuntimeModelMappingOptions(second)
	historical, err := CategoryDefaultSourceDeclarations(source)
	require.NoError(t, err)
	require.Equal(t, published, historical, "a historic Grok declaration must not be reinterpreted by the current process defaults")
	source.GrokMappingOptions = &second
	current, err := CategoryDefaultSourceDeclarations(source)
	require.NoError(t, err)
	require.Len(t, current, 2)
	for i := range published {
		if published[i].PublicModel == "grok" {
			require.NotEqual(t, published[i].Fingerprint, current[i].Fingerprint)
		} else {
			require.Equal(t, published[i].Fingerprint, current[i].Fingerprint, "an unchanged concrete model is not affected by the default alias")
		}
	}
}

func TestCategoryDefaultGrokRuntimeWaitsForOnlyAffectedProcessMappings(t *testing.T) {
	before := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(before) })
	database := xai.ModelMappingOptions{DefaultText: "grok-4.7", EnableCrossClientMap: true}
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{DefaultText: "grok-4.6"})
	svc, _ := categoryDefaultPublishSource(t, categoryDefaultGrokSource(database), "grok")
	user := &User{ID: 2, Status: StatusActive}
	_, err := svc.ResolveForUser(context.Background(), 100, user, "grok")
	require.ErrorIs(t, err, ErrCategoryDefaultPendingSync, "a replica with stale Grok defaults must not forward the published alias to a different target")
	models, err := svc.ModelsForUser(context.Background(), 100, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"grok-4.6"}, models)
	xai.SetRuntimeModelMappingOptions(database)
	_, err = svc.ResolveForUser(context.Background(), 100, user, "grok")
	require.NoError(t, err)
}
