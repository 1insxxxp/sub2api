//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGatewayBuildDynamicSystemCustomGroupCatalogUsesOrderedLiveSources(t *testing.T) {
	first := directSourceGroup(10, PlatformOpenAI)
	first.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"Shared", "Only-First"}}
	second := directSourceGroup(20, PlatformOpenAI)
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"shared": "gpt-5.4", "only-second": "gpt-5.4"})},
		{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"Shared": "gpt-5.4", "Only-First": "gpt-5.4", "Hidden": "gpt-5.4"})},
	}}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 20, Priority: 9, SourceGroup: second},
		{SourceGroupID: 10, Priority: 1, SourceGroup: first},
	}, "")

	require.NoError(t, err)
	require.Equal(t, []string{"Only-First", "only-second", "Shared"}, catalog.Models())
	shared, advertised := catalog.Resolve("SHARED")
	require.True(t, advertised)
	require.Len(t, shared, 2)
	require.Equal(t, []int64{10, 20}, []int64{shared[0].SourceGroup.ID, shared[1].SourceGroup.ID})
	require.Equal(t, "Shared", shared[0].PublicModel, "highest-priority valid source owns public spelling")
	_, hiddenAdvertised := catalog.Resolve("Hidden")
	require.False(t, hiddenAdvertised, "the source custom model list remains an explicit whitelist")
	require.Equal(t, 1, repo.calls)
	require.Equal(t, []int64{10, 20}, repo.requested, "source reference priority must determine the batch order")
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFallsThroughUnavailableAndUnpricedSources(t *testing.T) {
	unpriced := directSourceGroup(10, PlatformOpenAI)
	unavailable := directSourceGroup(20, PlatformOpenAI)
	available := directSourceGroup(30, PlatformOpenAI)
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"shared": "vendor-unpriced"})},
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformGemini, map[string]any{"shared": "gemini-3.1-pro"})},
		{GroupID: 30, Account: schedulableSystemCustomTestAccount(301, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})},
	}}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

	unpricedOnly, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: unpriced},
	}, "")
	require.NoError(t, err)
	unpricedCandidates, advertised := unpricedOnly.Resolve("shared")
	require.True(t, advertised)
	require.Empty(t, unpricedCandidates)
	require.Empty(t, unpricedOnly.Models())

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 30, Priority: 3, SourceGroup: available},
		{SourceGroupID: 10, Priority: 1, SourceGroup: unpriced},
		{SourceGroupID: 20, Priority: 2, SourceGroup: unavailable},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(30), candidates[0].SourceGroup.ID)
	require.Equal(t, []string{"shared"}, catalog.Models())
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogOmitsInvalidSourcesAndHonorsPlatform(t *testing.T) {
	inactive := directSourceGroup(10, PlatformOpenAI)
	inactive.Status = StatusDisabled
	nested := directSourceGroup(20, PlatformOpenAI)
	nested.SystemCustomRoutingEnabled = true
	unsupported := directSourceGroup(30, "kiro")
	gemini := directSourceGroup(40, PlatformGemini)
	openai := directSourceGroup(50, PlatformOpenAI)
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"inactive": "gpt-5.4"})},
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"nested": "gpt-5.4"})},
		{GroupID: 30, Account: schedulableSystemCustomTestAccount(301, "kiro", map[string]any{"unsupported": "gpt-5.4"})},
		{GroupID: 40, Account: schedulableSystemCustomTestAccount(401, PlatformGemini, map[string]any{"gemini-3.1-pro": "gemini-3.1-pro"})},
		{GroupID: 50, Account: schedulableSystemCustomTestAccount(501, PlatformOpenAI, map[string]any{"gpt-5.4": "gpt-5.4"})},
	}}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 5, Priority: 0, SourceGroup: nil},
		{SourceGroupID: 10, Priority: 1, SourceGroup: inactive},
		{SourceGroupID: 20, Priority: 2, SourceGroup: nested},
		{SourceGroupID: 30, Priority: 3, SourceGroup: unsupported},
		{SourceGroupID: 40, Priority: 4, SourceGroup: gemini},
		{SourceGroupID: 50, Priority: 5, SourceGroup: openai},
	}, PlatformGemini)

	require.NoError(t, err)
	require.Equal(t, []string{"gemini-3.1-pro"}, catalog.Models())
	for _, model := range []string{"inactive", "nested", "unsupported", "gpt-5.4"} {
		_, advertised := catalog.Resolve(model)
		require.False(t, advertised, model)
	}
	require.Equal(t, []int64{40}, repo.requested, "invalid and filtered references must not enter the account query")
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogReflectsLiveMappingChangesWithoutCache(t *testing.T) {
	source := directSourceGroup(10, PlatformOpenAI)
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"old-name": "gpt-5.4"})},
	}}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}
	refs := []SystemCustomGroupSource{{SourceGroupID: 10, Priority: 1, SourceGroup: source}}

	before, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), refs, "")
	require.NoError(t, err)
	require.Equal(t, []string{"old-name"}, before.Models())

	repo.accounts[0].Account = schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"new-name": "gpt-5.4"})
	after, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), refs, "")
	require.NoError(t, err)
	require.Equal(t, []string{"new-name"}, after.Models())
	_, oldAdvertised := after.Resolve("old-name")
	require.False(t, oldAdvertised)
	require.Equal(t, 2, repo.calls)
}

func TestDynamicSystemCustomGroupCatalogModelsAreDeterministic(t *testing.T) {
	catalog := &SystemCustomGroupRuntimeCatalog{availableModels: map[string]string{
		"zulu": "Zulu", "alpha": "alpha", "bravo": "Bravo",
	}}
	models := catalog.Models()
	require.Equal(t, []string{"alpha", "Bravo", "Zulu"}, models)
}

type systemCustomGroupBatchAccountRepoStub struct {
	AccountRepository
	accounts  []SystemCustomGroupSchedulableAccount
	calls     int
	requested []int64
}

func (s *systemCustomGroupBatchAccountRepoStub) ListSchedulableByGroupIDs(_ context.Context, groupIDs []int64) ([]SystemCustomGroupSchedulableAccount, error) {
	s.calls++
	s.requested = append([]int64(nil), groupIDs...)
	return append([]SystemCustomGroupSchedulableAccount(nil), s.accounts...), nil
}

func TestGatewayListSystemCustomGroupModelAvailabilityUsesSchedulerSupportInOneBatch(t *testing.T) {
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{
			GroupID: 10,
			Account: Account{
				ID: 101, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{
					"claude-*": "upstream-claude",
				}},
			},
		},
		{
			GroupID: 20,
			Account: Account{
				ID: 202, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{
					"Claude-*": "upstream-claude",
				}},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	availability, err := svc.ListSystemCustomGroupModelAvailability(context.Background(), []SystemCustomGroupModelListSource{
		{Group: *directSourceGroup(10, PlatformAnthropic), Models: []string{"claude-sonnet-4-6"}},
		{Group: *directSourceGroup(20, PlatformAnthropic), Models: []string{"claude-sonnet-4-6"}},
	})

	require.NoError(t, err)
	require.True(t, availability[10]["claude-sonnet-4-6"], "scheduler wildcard support must make the alias visible")
	require.False(t, availability[20]["claude-sonnet-4-6"], "display normalization must not broaden case-sensitive scheduler support")
	require.Equal(t, 1, repo.calls, "all source groups must share one account snapshot query")
	require.ElementsMatch(t, []int64{10, 20}, repo.requested)
}

func TestGatewayListSystemCustomGroupModelAvailabilityIncludesRouteServedByEmptyMappingAccount(t *testing.T) {
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{
			GroupID: 10,
			Account: Account{
				ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{
					"foo": "upstream-foo",
				}},
			},
		},
		{
			GroupID: 10,
			Account: Account{
				ID: 102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Credentials: map[string]any{},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	availability, err := svc.ListSystemCustomGroupModelAvailability(context.Background(), []SystemCustomGroupModelListSource{
		{Group: *directSourceGroup(10, PlatformOpenAI), Models: []string{"gpt-5.5"}},
	})

	require.NoError(t, err)
	require.True(t, availability[10]["gpt-5.5"], "an explicit route must stay visible when any real scheduler candidate can serve it")
	require.Equal(t, 1, repo.calls)
}

func TestGatewayListSystemCustomGroupModelAvailabilityKeepsSourceAndRuntimeControls(t *testing.T) {
	customNotSelected := directSourceGroup(30, PlatformOpenAI)
	customNotSelected.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.4"}}
	customSelected := directSourceGroup(31, PlatformOpenAI)
	customSelected.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.5"}}
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"foo": "upstream-foo"})},
		{GroupID: 30, Account: schedulableSystemCustomTestAccount(301, PlatformOpenAI, nil)},
		{GroupID: 31, Account: schedulableSystemCustomTestAccount(311, PlatformOpenAI, map[string]any{"foo": "upstream-foo"})},
		{GroupID: 31, Account: schedulableSystemCustomTestAccount(312, PlatformOpenAI, nil)},
		{GroupID: 40, Account: schedulableSystemCustomTestAccount(401, PlatformGemini, nil)},
		{GroupID: 50, Account: schedulableSystemCustomTestAccount(501, PlatformOpenAI, nil)},
	}}
	svc := &GatewayService{accountRepo: repo}

	availability, err := svc.ListSystemCustomGroupModelAvailability(context.Background(), []SystemCustomGroupModelListSource{
		{Group: *directSourceGroup(20, PlatformOpenAI), Models: []string{"gpt-5.5"}},
		{Group: *customNotSelected, Models: []string{"gpt-5.5"}},
		{Group: *customSelected, Models: []string{"gpt-5.5"}},
		{Group: *directSourceGroup(40, PlatformGemini), Models: []string{"gemini-2.0-flash"}},
		{Group: *directSourceGroup(50, PlatformOpenAI), Models: []string{"gpt-5.4"}},
	})

	require.NoError(t, err)
	require.False(t, availability[20]["gpt-5.5"], "non-empty mappings that all reject the route must keep it hidden")
	require.False(t, availability[30]["gpt-5.5"], "an enabled custom model list must remain a source-group whitelist")
	require.True(t, availability[31]["gpt-5.5"], "a selected custom model may use any real scheduler candidate")
	require.False(t, availability[40]["gemini-2.0-flash"], "provider lifecycle restrictions must still hide unavailable models")
	require.True(t, availability[50]["gpt-5.4"], "default models on empty mappings must remain visible")
	require.Equal(t, 1, repo.calls)
}

func schedulableSystemCustomTestAccount(id int64, platform string, mapping map[string]any) Account {
	credentials := map[string]any{}
	if mapping != nil {
		credentials["model_mapping"] = mapping
	}
	return Account{
		ID: id, Platform: platform, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Credentials: credentials,
	}
}
