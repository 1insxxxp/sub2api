package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func categoryDefaultSource(id int64, name string, mapping map[string]any) CategoryDefaultSourceSnapshot {
	return CategoryDefaultSourceSnapshot{
		Group:    &Group{ID: id, Name: name, Platform: PlatformGemini, Status: StatusActive, Hydrated: true, SubscriptionType: SubscriptionTypeStandard},
		Accounts: []Account{{ID: id, Platform: PlatformGemini, Status: StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": mapping}}},
	}
}

func TestCategoryDefaultCatalogConflictsAcrossSources(t *testing.T) {
	// Account keys are exact routing declarations, not trim-normalized input.
	// Both sources must really serve their spelling before testing collision.
	sources := []CategoryDefaultSourceSnapshot{categoryDefaultSource(1, "first", map[string]any{"Foo": "upstream-a"}), categoryDefaultSource(2, "second", map[string]any{"foo": "upstream-b"})}
	for i, name := range []string{"Foo", "foo"} {
		_, matched := sources[i].Accounts[0].ResolveMappedModel(name)
		require.True(t, matched)
	}
	models, conflicts, err := buildCategoryDefaultCatalog("gemini", sources)
	require.NoError(t, err)
	require.Empty(t, models)
	require.Len(t, conflicts, 1)
	require.Equal(t, []int64{1, 2}, conflicts[0].SourceGroupIDs)
}

func TestCategoryDefaultCatalogDoesNotInventRoutesFromWhitespaceMappingKeys(t *testing.T) {
	source := categoryDefaultSource(1, "first", map[string]any{" Foo ": "upstream-a", "safe": "upstream-safe"})
	_, matched := source.Accounts[0].ResolveMappedModel("Foo")
	require.False(t, matched, "the real account lookup does not normalize a stored mapping key")
	models, conflicts, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	require.Empty(t, conflicts)
	require.Len(t, models, 1)
	require.Equal(t, "safe", models[0].PublicModel)
}

func TestCategoryDefaultCatalogFingerprintAndAlias(t *testing.T) {
	source := categoryDefaultSource(1, "first", map[string]any{"raw": "upstream-a"})
	source.Group.ModelAliases = []domain.ModelAlias{{Alias: "public", UpstreamModel: "raw"}}
	initial, _, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	require.Len(t, initial, 2)
	source.Group.RateMultiplier = 3
	source.Accounts[0].ErrorMessage = "transient"
	unchanged, _, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	require.Equal(t, initial, unchanged)
	source.Accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"raw": "upstream-b"}}
	changed, _, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	require.NotEqual(t, initial[0].Fingerprint, changed[0].Fingerprint)
	source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"public"}}
	filtered, _, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	require.Len(t, filtered, 1)
	require.Equal(t, "public", filtered[0].PublicModel)
}

func TestCategoryDefaultRejectsUnboundedMapping(t *testing.T) {
	source := categoryDefaultSource(1, "first", map[string]any{"*": "*"})
	_, _, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.ErrorIs(t, err, ErrCategoryDefaultUnboundedSource)
	source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"foo"}}
	models, _, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	require.Len(t, models, 1)
}

func TestCategoryDefaultEffectiveDisplayCategory(t *testing.T) {
	require.Equal(t, "domestic", EffectiveKeyDisplayCategory(&Group{Platform: PlatformDeepseek}))
	require.Equal(t, "image", EffectiveKeyDisplayCategory(&Group{Platform: PlatformGemini, Tag: "image"}))
	require.Equal(t, "grok", EffectiveKeyDisplayCategory(&Group{Platform: PlatformGemini, Tag: "image", KeyDisplayCategory: "grok"}))
}

type categoryDefaultStoreFake struct {
	config  *CategoryDefaultGroupConfig
	sources []CategoryDefaultSourceSnapshot
}

func (f *categoryDefaultStoreFake) Snapshot(_ context.Context, req CategoryDefaultSnapshotRequest) (*CategoryDefaultCatalogSnapshot, error) {
	config := &CategoryDefaultGroupConfig{Category: req.Category, SourceGroupIDs: []int64{}, Models: []CategoryDefaultPublishedModel{}}
	if f.config != nil && (req.Category == f.config.Category || req.GroupID == f.config.GroupID) {
		clone := *f.config
		config = &clone
	} else if req.GroupID != 0 {
		return nil, ErrCategoryDefaultNotFound
	}
	if !req.IncludeHistory {
		config.InvalidatedModels = nil
	}
	ids := append(append([]int64(nil), config.SourceGroupIDs...), req.AdditionalSourceGroupIDs...)
	return &CategoryDefaultCatalogSnapshot{Config: config, Sources: categoryDefaultSelectedSources(f.sources, ids)}, nil
}

func (f *categoryDefaultStoreFake) List(context.Context) ([]CategoryDefaultGroupConfig, error) {
	if f.config == nil {
		return nil, nil
	}
	return []CategoryDefaultGroupConfig{*f.config}, nil
}
func (f *categoryDefaultStoreFake) Get(context.Context, string) (*CategoryDefaultGroupConfig, error) {
	if f.config == nil {
		return nil, ErrCategoryDefaultNotFound
	}
	return f.config, nil
}
func (f *categoryDefaultStoreFake) GetByGroupID(context.Context, int64) (*CategoryDefaultGroupConfig, error) {
	if f.config == nil {
		return nil, ErrCategoryDefaultNotFound
	}
	return f.config, nil
}
func (f *categoryDefaultStoreFake) Sources(context.Context, []int64) ([]CategoryDefaultSourceSnapshot, error) {
	return f.sources, nil
}
func (f *categoryDefaultStoreFake) CandidateSources(context.Context, string) ([]CategoryDefaultSourceSnapshot, error) {
	return f.sources, nil
}
func (f *categoryDefaultStoreFake) Publish(_ context.Context, category string, ids []int64, expected int64, validate CategoryDefaultPublishValidator) (*CategoryDefaultGroupConfig, error) {
	current := f.config
	if current == nil {
		current = &CategoryDefaultGroupConfig{Category: category}
	}
	if current.Revision != expected {
		return nil, ErrCategoryDefaultStalePreview
	}
	models, err := validate(current, categoryDefaultSelectedSources(f.sources, ids))
	if err != nil {
		return nil, err
	}
	f.config = &CategoryDefaultGroupConfig{GroupID: 100, Category: category, Enabled: true, Revision: expected + 1, SourceGroupIDs: ids, Models: models, ModelsCount: len(models)}
	return f.config, nil
}
func (f *categoryDefaultStoreFake) SetStatus(_ context.Context, _ string, enabled bool) (*CategoryDefaultGroupConfig, error) {
	if f.config == nil {
		return nil, ErrCategoryDefaultNotFound
	}
	f.config.Enabled = enabled
	return f.config, nil
}

func TestCategoryDefaultSavedSourcesFollowPermissionAndMappingChanges(t *testing.T) {
	ctx := context.Background()
	fake := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{categoryDefaultSource(1, "first", map[string]any{"foo": "upstream"})}}
	svc := NewCategoryDefaultGroupService(fake, nil)
	preview, err := svc.Preview(ctx, "gemini", []int64{1})
	require.NoError(t, err)
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, ExpectedRevision: 0, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	user := &User{ID: 2, Status: StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{1}}
	route, err := svc.ResolveForUser(ctx, 100, user, "foo")
	require.NoError(t, err)
	require.Equal(t, int64(1), route.SourceGroup.ID)
	can, err := svc.CanBind(ctx, 100, user)
	require.NoError(t, err)
	require.True(t, can)
	fake.sources[0].Accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"foo": "upstream", "bar": "new-upstream"}}
	models, err := svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"bar", "foo"}, models)
	_, err = svc.ResolveForUser(ctx, 100, user, "bar")
	require.NoError(t, err)
	fake.sources[0].Accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"foo": "changed"}}
	_, err = svc.ResolveForUser(ctx, 100, user, "foo")
	require.NoError(t, err)
	user.AllowedGroups = nil
	models, err = svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Empty(t, models)
}

func TestCategoryDefaultStalePreviewAndUnpublishedConflict(t *testing.T) {
	ctx := context.Background()
	fake := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{categoryDefaultSource(1, "first", map[string]any{"foo": "upstream"})}}
	svc := NewCategoryDefaultGroupService(fake, nil)
	preview, err := svc.Preview(ctx, "gemini", []int64{1})
	require.NoError(t, err)
	fake.sources[0].Accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"foo": "changed"}}
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, PreviewHash: preview.PreviewHash})
	require.ErrorIs(t, err, ErrCategoryDefaultStalePreview)
	require.Nil(t, fake.config)
	preview, err = svc.Preview(ctx, "gemini", []int64{1})
	require.NoError(t, err)
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	fake.config.SourceGroupIDs = []int64{1, 2}
	fake.sources = append(fake.sources, categoryDefaultSource(2, "second", map[string]any{"FOO": "other", "bar": "unchanged"}))
	_, err = svc.ResolveForUser(ctx, 100, &User{ID: 2, Status: StatusActive}, "foo")
	require.ErrorIs(t, err, ErrCategoryDefaultModelConflict)
}

func TestCategoryDefaultRejectsWrongCategoryNestedAndSubscriptionSources(t *testing.T) {
	for _, mutate := range []func(*Group){
		func(g *Group) { g.KeyDisplayCategory = "openai" },
		func(g *Group) { g.IsCategoryDefault = true },
		func(g *Group) { g.SystemCustomRoutingEnabled = true },
		func(g *Group) { g.SubscriptionType = SubscriptionTypeSubscription },
		func(g *Group) { g.Status = "inactive" },
	} {
		source := categoryDefaultSource(1, "first", map[string]any{"foo": "target"})
		mutate(source.Group)
		_, _, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
		require.ErrorIs(t, err, ErrCategoryDefaultInvalidSource)
	}
	for _, category := range []string{"image", "all", "", "unknown"} {
		_, _, err := buildCategoryDefaultCatalog(category, nil)
		require.ErrorIs(t, err, ErrCategoryDefaultInvalidCategory)
	}
}

func TestCategoryDefaultCompositeExactRoutesAndGeminiModels(t *testing.T) {
	source := CategoryDefaultSourceSnapshot{Group: &Group{ID: 1, Name: "composite", Platform: PlatformComposite, KeyDisplayCategory: "gemini", Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, Hydrated: true}, Routes: []CompositeModelRoute{{PublicModel: "public-text", MatchType: CompositeRouteMatchExact, TargetPlatform: PlatformGemini, UpstreamModel: "gemini-2.5-flash", Endpoint: CompositeRouteEndpointGemini, Enabled: true}}}
	models, conflicts, err := buildCategoryDefaultCatalog("gemini", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	require.Empty(t, conflicts)
	require.Len(t, models, 1)
	require.Equal(t, "public-text", models[0].SourceModel)
	require.True(t, categoryDefaultPlatformAllowed(source.Group, models[0], PlatformGemini))
}

func TestCategoryDefaultBrokenSourceKeepsOtherPublishedModels(t *testing.T) {
	ctx := context.Background()
	fake := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{
		categoryDefaultSource(1, "first", map[string]any{"foo": "upstream"}),
		categoryDefaultSource(2, "second", map[string]any{"bar": "upstream"}),
	}}
	svc := NewCategoryDefaultGroupService(fake, nil)
	preview, err := svc.Preview(ctx, "gemini", []int64{1, 2})
	require.NoError(t, err)
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1, 2}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	fake.sources[0].Accounts[0].Credentials = map[string]any{"model_mapping": map[string]any{"*": "*"}}
	models, err := svc.ModelsForUser(ctx, 100, &User{ID: 1, Status: StatusActive}, "")
	require.NoError(t, err)
	require.Equal(t, []string{"bar"}, models)
}

func TestCategoryDefaultAntigravityNativeListExcludesClaudeTargets(t *testing.T) {
	source := CategoryDefaultSourceSnapshot{Group: &Group{ID: 1, Name: "ag", Platform: PlatformAntigravity, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, Hydrated: true}, Accounts: []Account{{Platform: PlatformAntigravity, Credentials: map[string]any{"model_mapping": map[string]any{"friendly-gemini": "gemini-2.5-flash", "friendly-claude": "claude-sonnet-4-5"}}}}}
	models, _, err := buildCategoryDefaultCatalog("other", []CategoryDefaultSourceSnapshot{source})
	require.NoError(t, err)
	for _, model := range models {
		if model.PublicModel == "friendly-claude" {
			require.False(t, categoryDefaultPlatformAllowed(source.Group, model, PlatformGemini))
		}
		if model.PublicModel == "friendly-gemini" {
			require.True(t, categoryDefaultPlatformAllowed(source.Group, model, PlatformGemini))
		}
	}
}

func TestCategoryDefaultResolutionCarriesVerifiedNativeCapability(t *testing.T) {
	ctx := context.Background()
	source := categoryDefaultSource(1, "ag", map[string]any{"friendly-gemini": "gemini-2.5-flash", "friendly-claude": "claude-sonnet-4-5"})
	source.Group.Platform = PlatformAntigravity
	source.Accounts[0].Platform = PlatformAntigravity
	store := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{source}}
	svc := NewCategoryDefaultGroupService(store, nil)
	preview, err := svc.Preview(ctx, "other", []int64{1})
	require.NoError(t, err)
	_, err = svc.Apply(ctx, "other", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	user := &User{ID: 2, Status: StatusActive}
	gemini, err := svc.ResolveForUser(ctx, 100, user, "friendly-gemini")
	require.NoError(t, err)
	require.True(t, gemini.GeminiNative)
	claude, err := svc.ResolveForUser(ctx, 100, user, "friendly-claude")
	require.NoError(t, err)
	require.False(t, claude.GeminiNative)
}

func TestCategoryDefaultCatalogMatchesAccountLookupExactly(t *testing.T) {
	source := categoryDefaultSource(1, "ag", map[string]any{"Friendly": "gemini-2.5-flash", "friendly": "claude-sonnet-4-5"})
	source.Group.Platform = PlatformAntigravity
	source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"Friendly"}}
	source.Accounts[0].Platform = PlatformAntigravity
	for i := 0; i < 64; i++ {
		models, err := categoryDefaultSourceModels(source)
		require.NoError(t, err)
		require.Len(t, models, 1)
		require.Equal(t, "Friendly", models[0].SourceModel)
		require.True(t, models[0].GeminiNative, "the real exact account mapping must win over case variants")
	}
}

func TestCategoryDefaultCompactMappingFollowsEffectiveRouteChanges(t *testing.T) {
	for _, key := range []string{"public-model", "gpt-*"} {
		t.Run(key, func(t *testing.T) {
			ctx := context.Background()
			source := categoryDefaultSource(1, "openai", map[string]any{"public-model": "gpt-5"})
			source.Group.Platform = PlatformOpenAI
			source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}
			source.Accounts[0].Platform = PlatformOpenAI
			compact := map[string]any{key: "gpt-5-mini", "unrelated": "other"}
			source.Accounts[0].Credentials["compact_model_mapping"] = compact
			store := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{source}}
			svc := NewCategoryDefaultGroupService(store, nil)
			preview, err := svc.Preview(ctx, "openai", []int64{1})
			require.NoError(t, err)
			_, err = svc.Apply(ctx, "openai", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, PreviewHash: preview.PreviewHash})
			require.NoError(t, err)
			user := &User{ID: 2, Status: StatusActive}
			compact["unrelated"] = "changed-unrelated"
			_, err = svc.ResolveForUser(ctx, 100, user, "public-model")
			require.NoError(t, err)
			compact[key] = "gpt-5-nano"
			_, err = svc.ResolveForUser(ctx, 100, user, "public-model")
			require.NoError(t, err)
		})
	}
}

func TestCategoryDefaultHistoryOnlyExpiresSavePreview(t *testing.T) {
	ctx := context.Background()
	svc, store := categoryDefaultPublishSource(t, categoryDefaultSource(1, "first", map[string]any{"foo": "gemini-2.5-flash"}), "gemini")
	oldPreview, err := svc.Preview(ctx, "gemini", []int64{1})
	require.NoError(t, err)
	// The declaration has returned to its approved value after an intervening
	// change; history expires save previews but must not block current routes.
	store.config.InvalidatedModels = []string{"foo"}
	user := &User{ID: 2, Status: StatusActive}
	_, err = svc.ResolveForUser(ctx, 100, user, "foo")
	require.NoError(t, err)
	models, err := svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"foo"}, models)
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, ExpectedRevision: oldPreview.Revision, PreviewHash: oldPreview.PreviewHash})
	require.ErrorIs(t, err, ErrCategoryDefaultStalePreview)
	preview, err := svc.Preview(ctx, "gemini", []int64{1})
	require.NoError(t, err)
	require.Empty(t, preview.Modified)
	require.Len(t, preview.Unchanged, 1)
	store.sources[0].DeclarationEpoch++
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
	require.ErrorIs(t, err, ErrCategoryDefaultStalePreview, "a new change/restore cycle must expire a preview even when the blocked names are unchanged")
	preview, err = svc.Preview(ctx, "gemini", []int64{1})
	require.NoError(t, err)
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1}, ExpectedRevision: preview.Revision, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	_, err = svc.ResolveForUser(ctx, 100, user, "foo")
	require.NoError(t, err)
}
