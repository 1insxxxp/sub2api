package repository

import (
	"encoding/json"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func categoryDefaultHistoryFixture(t *testing.T, sourceID int64, mapping map[string]any) service.CategoryDefaultSourceSnapshot {
	t.Helper()
	return service.CategoryDefaultSourceSnapshot{
		Group:    &service.Group{ID: sourceID, Platform: service.PlatformGemini, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, Hydrated: true},
		Accounts: []service.Account{{ID: sourceID, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Credentials: map[string]any{"model_mapping": mapping}}},
	}
}

func TestCategoryDefaultHistoricalGrokOptionsAreFrozen(t *testing.T) {
	initial := xai.RuntimeModelMappingOptions()
	t.Cleanup(func() { xai.SetRuntimeModelMappingOptions(initial) })
	xai.SetRuntimeModelMappingOptions(xai.ModelMappingOptions{DefaultText: "grok-4.6", EnableCrossClientMap: true})
	source := service.CategoryDefaultSourceSnapshot{
		Group:              &service.Group{ID: 1, Platform: service.PlatformGrok, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, Hydrated: true},
		Accounts:           []service.Account{{ID: 1, Platform: service.PlatformGrok, Type: service.AccountTypeAPIKey}},
		GrokMappingOptions: &xai.ModelMappingOptions{DefaultText: "grok-4.6", EnableCrossClientMap: true},
	}
	models, err := service.CategoryDefaultSourceDeclarations(source)
	require.NoError(t, err)
	config := &service.CategoryDefaultGroupConfig{GroupID: 100, Category: "grok", Models: models}
	historical := categoryDefaultHistoricalSource{
		Group:              &dbent.Group{ID: 1, Platform: service.PlatformGrok, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard},
		Accounts:           source.Accounts,
		GrokMappingOptions: &xai.ModelMappingOptions{DefaultText: "grok-4.3", EnableCrossClientMap: true},
	}
	raw, err := json.Marshal(historical)
	require.NoError(t, err)
	changed, err := categoryDefaultDecodeHistoricalSource(raw)
	require.NoError(t, err)
	require.Equal(t, historical.GrokMappingOptions, changed.GrokMappingOptions)
	invalid := categoryDefaultHistoryInvalidations(config, 1, changed)
	require.True(t, invalid["grok"], "the previous target must not be recomputed using the process's restored default")
	require.False(t, invalid["grok-4.7"], "an unchanged literal model must remain available")
}

func categoryDefaultHistoryConfig(t *testing.T, source service.CategoryDefaultSourceSnapshot) *service.CategoryDefaultGroupConfig {
	t.Helper()
	models, err := service.CategoryDefaultSourceDeclarations(source)
	require.NoError(t, err)
	return &service.CategoryDefaultGroupConfig{GroupID: 100, Category: "gemini", Revision: 1, SourceGroupIDs: []int64{source.Group.ID}, Models: models}
}

func TestCategoryDefaultHistoricalChangeOnlyInvalidatesAffectedPublishedModel(t *testing.T) {
	original := categoryDefaultHistoryFixture(t, 1, map[string]any{"foo": "gemini-2.5-flash", "bar": "gemini-2.5-pro"})
	config := categoryDefaultHistoryConfig(t, original)
	changed := categoryDefaultHistoryFixture(t, 1, map[string]any{"foo": "gemini-3-pro", "bar": "gemini-2.5-pro"})
	require.Equal(t, map[string]bool{"foo": true}, categoryDefaultHistoryInvalidations(config, 1, changed))
	// The original declaration now matches, but the prior durable event remains
	// invalidating. The loader unions events instead of considering only latest.
	require.Empty(t, categoryDefaultHistoryInvalidations(config, 1, original))
	changed.Group.RateMultiplier = 9
	changed.Accounts[0].RateLimitedAt = nil
	require.Equal(t, map[string]bool{"foo": true}, categoryDefaultHistoryInvalidations(config, 1, changed))
}

func TestCategoryDefaultHistoricalConflictsOnlyInvalidateTheCollidingName(t *testing.T) {
	source := categoryDefaultHistoryFixture(t, 1, map[string]any{"Foo": "gemini-2.5-flash", "bar": "gemini-2.5-pro"})
	config := categoryDefaultHistoryConfig(t, source)
	config.SourceGroupIDs = append(config.SourceGroupIDs, 2)
	other := categoryDefaultHistoryFixture(t, 2, map[string]any{"foo": "gemini-3-pro", "new": "gemini-2.5-pro"})
	_, matched := other.Accounts[0].ResolveMappedModel("foo")
	require.True(t, matched, "the conflicting declaration must be routable by the actual account lookup")
	require.Equal(t, map[string]bool{"foo": true}, categoryDefaultHistoryInvalidations(config, 2, other))
	other.Group.Status = "inactive"
	require.Empty(t, categoryDefaultHistoryInvalidations(config, 2, other), "unavailable foreign source does not disable unrelated owners")
	malformed := categoryDefaultHistoryFixture(t, 2, map[string]any{" foo ": "gemini-3-pro", "new": "gemini-2.5-pro"})
	_, matched = malformed.Accounts[0].ResolveMappedModel("foo")
	require.False(t, matched)
	require.Empty(t, categoryDefaultHistoryInvalidations(config, 2, malformed), "an unroutable whitespace key must not fabricate a historical conflict")
}

func TestCategoryDefaultHistoricalSourceDisableCategoryAndRemovalAreSticky(t *testing.T) {
	source := categoryDefaultHistoryFixture(t, 1, map[string]any{"foo": "gemini-2.5-flash"})
	config := categoryDefaultHistoryConfig(t, source)
	for _, change := range []func(*service.CategoryDefaultSourceSnapshot){
		func(s *service.CategoryDefaultSourceSnapshot) { s.Group.Status = "inactive" },
		func(s *service.CategoryDefaultSourceSnapshot) { s.Group.KeyDisplayCategory = "openai" },
		func(s *service.CategoryDefaultSourceSnapshot) { s.Group = nil },
	} {
		copy := categoryDefaultHistoryFixture(t, 1, map[string]any{"foo": "gemini-2.5-flash"})
		change(&copy)
		require.Equal(t, map[string]bool{"foo": true}, categoryDefaultHistoryInvalidations(config, 1, copy))
	}
}

func TestCategoryDefaultHistoricalSnapshotUsesTheSameSourcePlatformPool(t *testing.T) {
	historical := categoryDefaultHistoricalSource{
		Group: &dbent.Group{ID: 1, Platform: service.PlatformGemini, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard, ModelAliases: []domain.ModelAlias{{Alias: "public", UpstreamModel: "foo"}}},
		Accounts: []service.Account{
			{ID: 1, Platform: service.PlatformGemini},
			{ID: 2, Platform: service.PlatformAntigravity},
			{ID: 3, Platform: service.PlatformOpenAI},
		},
	}
	raw, err := json.Marshal(historical)
	require.NoError(t, err)
	snapshot, err := categoryDefaultDecodeHistoricalSource(raw)
	require.NoError(t, err)
	require.True(t, snapshot.Group.Hydrated)
	require.Len(t, snapshot.Accounts, 2)
	require.Equal(t, service.PlatformAntigravity, snapshot.Accounts[1].Platform)
	require.Equal(t, "public", snapshot.Group.ModelAliases[0].Alias)
}
