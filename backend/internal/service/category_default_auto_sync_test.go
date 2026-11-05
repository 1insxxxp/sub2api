package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestCategoryDefaultAutoFollowDeclarationsWithoutPublishing(t *testing.T) {
	ctx := context.Background()
	svc, store := categoryDefaultPublishSource(t, categoryDefaultSource(1, "source", map[string]any{"old": "gemini-2.5-flash"}), "gemini")
	user := &User{ID: 2, Status: StatusActive}
	revision := store.config.Revision
	storedModels := append([]CategoryDefaultPublishedModel(nil), store.config.Models...)
	store.config.InvalidatedModels = []string{"old", "renamed"}
	store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"renamed": "gemini-2.5-pro", "added": "gemini-2.5-flash"}
	store.sources[0].Group.RateMultiplier = 2.5
	reset := time.Now().Add(time.Hour)
	store.sources[0].Accounts[0].RateLimitResetAt = &reset

	models, err := svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"added", "renamed"}, models)
	_, err = svc.ResolveForUser(ctx, 100, user, "old")
	require.ErrorIs(t, err, ErrCategoryDefaultModelNotAllowed)
	route, err := svc.ResolveForUser(ctx, 100, user, "renamed")
	require.NoError(t, err)
	require.Equal(t, int64(1), route.SourceGroup.ID)
	require.Equal(t, "renamed", route.SourceModel)
	require.Equal(t, 2.5, route.SourceGroup.RateMultiplier)

	store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"old": "gemini-2.5-flash"}
	models, err = svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"old"}, models, "a restored declaration must not remain latched by historical invalidation")
	require.Equal(t, revision, store.config.Revision)
	require.Equal(t, storedModels, store.config.Models, "reads must never republish or mutate the persisted manifest")
}

func TestCategoryDefaultAutoFollowAdminListAndPreviewUseCurrentOldSources(t *testing.T) {
	ctx := context.Background()
	svc, store := categoryDefaultPublishSource(t, categoryDefaultSource(1, "source", map[string]any{"old": "gemini-2.5-flash"}), "gemini")
	store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"current": "gemini-2.5-pro"}
	store.config.InvalidatedModels = []string{"old"}
	store.config.Enabled = false
	configs, err := svc.List(ctx)
	require.NoError(t, err)
	for _, config := range configs {
		if config.Category == "gemini" {
			require.False(t, config.Enabled)
			require.Equal(t, 1, config.ModelsCount)
			require.Len(t, config.Models, 1)
			require.Equal(t, "current", config.Models[0].PublicModel)
		}
	}
	preview, err := svc.Preview(ctx, "gemini", []int64{1})
	require.NoError(t, err)
	require.Empty(t, preview.Added)
	require.Empty(t, preview.Modified)
	require.Empty(t, preview.Removed)
	require.Len(t, preview.Unchanged, 1)
	require.Equal(t, "current", preview.Unchanged[0].PublicModel)
	require.Equal(t, "old", store.config.Models[0].PublicModel)
}

func TestCategoryDefaultAutoFollowConflictIsGlobalAndRecovers(t *testing.T) {
	ctx := context.Background()
	first := categoryDefaultSource(1, "first", map[string]any{"shared": "gemini-2.5-flash", "safe": "gemini-2.5-pro"})
	second := categoryDefaultSource(2, "private", map[string]any{"other": "gemini-2.5-pro"})
	second.Group.IsExclusive = true
	store := &categoryDefaultStoreFake{sources: []CategoryDefaultSourceSnapshot{first, second}}
	svc := NewCategoryDefaultGroupService(store, nil)
	preview, err := svc.Preview(ctx, "gemini", []int64{1, 2})
	require.NoError(t, err)
	_, err = svc.Apply(ctx, "gemini", CategoryDefaultGroupApplyRequest{SourceGroupIDs: []int64{1, 2}, PreviewHash: preview.PreviewHash})
	require.NoError(t, err)
	user := &User{ID: 3, Status: StatusActive, RestrictPublicGroups: true, AllowedGroups: []int64{1}}
	store.sources[1].Accounts[0].Credentials["model_mapping"] = map[string]any{"SHARED": "gemini-2.5-pro"}
	models, err := svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"safe"}, models)
	_, err = svc.ResolveForUser(ctx, 100, user, "shared")
	require.ErrorIs(t, err, ErrCategoryDefaultModelConflict, "lack of permission to the second source must not select the first")
	store.sources[1].Accounts[0].Credentials["model_mapping"] = map[string]any{"other": "gemini-2.5-pro"}
	store.config.InvalidatedModels = []string{"shared"}
	models, err = svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Equal(t, []string{"safe", "shared"}, models)
	user.AllowedGroups = nil
	models, err = svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Empty(t, models)
}

func TestCategoryDefaultAutoFollowEmptyAccountPoolDoesNotInventModels(t *testing.T) {
	ctx := context.Background()
	svc, store := categoryDefaultPublishSource(t, categoryDefaultSource(1, "source", map[string]any{"old": "gemini-2.5-flash"}), "gemini")
	store.sources[0].Accounts = nil
	user := &User{ID: 2, Status: StatusActive}
	models, err := svc.ModelsForUser(ctx, 100, user, "")
	require.NoError(t, err)
	require.Empty(t, models)
	_, err = svc.Preview(ctx, "gemini", []int64{1})
	require.ErrorIs(t, err, ErrCategoryDefaultUnboundedSource, "saving must not replace an empty source with the platform defaults")
}

func TestCategoryDefaultAutoFollowDeletionWithStaleAllowlistOrAlias(t *testing.T) {
	for _, kind := range []string{"allowlist", "alias"} {
		t.Run(kind, func(t *testing.T) {
			source := categoryDefaultSource(1, "source", map[string]any{"a": "upstream-a", "b": "upstream-b"})
			if kind == "allowlist" {
				source.Group.ModelAllowlist = GroupModelAllowlist{Enabled: true, Models: []string{"a", "b"}}
			} else {
				source.Group.ModelAliases = []domain.ModelAlias{{Alias: "alias-a", UpstreamModel: "a"}}
			}
			svc, store := categoryDefaultPublishSource(t, source, "gemini")
			store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"b": "upstream-b"}
			models, err := svc.ModelsForUser(context.Background(), 100, &User{ID: 2, Status: StatusActive}, "")
			require.NoError(t, err)
			require.Equal(t, []string{"b"}, models)
		})
	}
}

func TestCategoryDefaultAutoFollowStatusReturnsCurrentCounts(t *testing.T) {
	svc, store := categoryDefaultPublishSource(t, categoryDefaultSource(1, "source", map[string]any{"a": "upstream-a"}), "gemini")
	store.sources[0].Accounts[0].Credentials["model_mapping"] = map[string]any{"a": "upstream-a", "b": "upstream-b"}
	for _, enabled := range []bool{false, true} {
		config, err := svc.SetStatus(context.Background(), "gemini", enabled)
		require.NoError(t, err)
		require.Equal(t, enabled, config.Enabled)
		require.Equal(t, 2, config.ModelsCount)
		require.Len(t, config.Models, 2)
	}
}
