//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
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

func TestGatewayBuildDynamicSystemCustomGroupCatalogAdvertisesDisabledAndNoAccountSources(t *testing.T) {
	disabled := directSourceGroup(10, PlatformOpenAI)
	disabled.Status = StatusDisabled
	disabled.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"disabled-model"}}
	noAccounts := directSourceGroup(20, PlatformOpenAI)
	nested := directSourceGroup(30, PlatformOpenAI)
	nested.SystemCustomRoutingEnabled = true
	nested.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"nested-model"}}
	unsupported := directSourceGroup(40, "kiro")
	unsupported.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"unsupported-model"}}
	repo := &systemCustomGroupBatchAccountRepoStub{}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: disabled},
		{SourceGroupID: 20, Priority: 2, SourceGroup: noAccounts},
		{SourceGroupID: 30, Priority: 3, SourceGroup: nested},
		{SourceGroupID: 40, Priority: 4, SourceGroup: unsupported},
		{SourceGroupID: 50, Priority: 5, SourceGroup: nil},
	}, "")

	require.NoError(t, err)
	disabledCandidates, disabledAdvertised := catalog.Resolve("DISABLED-MODEL")
	require.True(t, disabledAdvertised)
	require.Empty(t, disabledCandidates)
	defaultCandidates, defaultAdvertised := catalog.Resolve("gpt-5.4")
	require.True(t, defaultAdvertised)
	require.Empty(t, defaultCandidates)
	for _, model := range []string{"nested-model", "unsupported-model"} {
		candidates, advertised := catalog.Resolve(model)
		require.True(t, advertised, model)
		require.Empty(t, candidates, model)
	}
	_, deletedAdvertised := catalog.Resolve("deleted-model")
	require.False(t, deletedAdvertised)
	require.Empty(t, catalog.Models(), "advertised-but-unavailable models must not enter the public model list")
	require.Equal(t, 1, repo.calls)
	require.Equal(t, []int64{20}, repo.requested, "invalid sources must not enter the account snapshot")
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogUsesSchedulerMixedPlatformRules(t *testing.T) {
	accepted := directSourceGroup(10, PlatformAnthropic)
	rejected := directSourceGroup(20, PlatformAnthropic)
	mixed := schedulableSystemCustomTestAccount(101, PlatformAntigravity, map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"})
	mixed.Extra = map[string]any{"mixed_scheduling": true}
	notMixed := schedulableSystemCustomTestAccount(201, PlatformAntigravity, map[string]any{"claude-opus-4-6": "claude-opus-4-6"})
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: mixed},
		{GroupID: 20, Account: notMixed},
	}}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: accepted},
		{SourceGroupID: 20, Priority: 2, SourceGroup: rejected},
	}, "")

	require.NoError(t, err)
	acceptedCandidates, acceptedAdvertised := catalog.Resolve("claude-sonnet-4-6")
	require.True(t, acceptedAdvertised)
	require.Len(t, acceptedCandidates, 1)
	rejectedCandidates, rejectedAdvertised := catalog.Resolve("claude-opus-4-6")
	require.True(t, rejectedAdvertised)
	require.Empty(t, rejectedCandidates)
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFallsBackWhenPrivacyGateRejectsFirstSource(t *testing.T) {
	private := directSourceGroup(10, PlatformOpenAI)
	private.RequirePrivacySet = true
	fallback := directSourceGroup(20, PlatformOpenAI)
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})},
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})},
	}}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: private},
		{SourceGroupID: 20, Priority: 2, SourceGroup: fallback},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(20), candidates[0].SourceGroup.ID)
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFallsBackWithDispatchProfitContext(t *testing.T) {
	first := gatewayProfitTestGroup(10, PlatformOpenAI)
	second := directSourceGroup(20, PlatformOpenAI)
	expensive := gatewayProfitTestAccount(101, PlatformOpenAI, 0.8, first.ID)
	expensive.Credentials = map[string]any{"model_mapping": map[string]any{"shared": "gpt-5.4"}}
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: expensive},
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})},
	}}
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}
	billingGroup := &Group{ID: 99, Platform: PlatformComposite, Status: StatusActive, Hydrated: true, RateMultiplier: 0.5}
	ctx := context.WithValue(context.Background(), ctxkey.Group, billingGroup)
	ctx, _ = WithGatewayTokenRequestPricing(ctx)

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(ctx, []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: first},
		{SourceGroupID: 20, Priority: 2, SourceGroup: second},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(20), candidates[0].SourceGroup.ID)
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFailsClosedWithoutPricing(t *testing.T) {
	source := directSourceGroup(10, PlatformOpenAI)
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"gpt-5.4": "gpt-5.4"})},
	}}

	_, err := (&GatewayService{accountRepo: repo}).BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: source},
	}, "")

	require.ErrorContains(t, err, "pricing")
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogUsesDefaultAndCustomModelListSemantics(t *testing.T) {
	t.Run("empty mapping uses defaults", func(t *testing.T) {
		source := directSourceGroup(10, PlatformOpenAI)
		repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
			{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, nil)},
		}}
		catalog, err := (&GatewayService{accountRepo: repo, billingService: newTestBillingService()}).BuildSystemCustomGroupModelCatalog(
			context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "",
		)
		require.NoError(t, err)
		candidates, advertised := catalog.Resolve("gpt-5.4")
		require.True(t, advertised)
		require.Len(t, candidates, 1)
	})

	t.Run("openai passthrough uses defaults", func(t *testing.T) {
		source := directSourceGroup(10, PlatformOpenAI)
		account := schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"mapped-only": "gpt-5.4"})
		account.Extra = map[string]any{"openai_passthrough": true}
		repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{{GroupID: 10, Account: account}}}
		catalog, err := (&GatewayService{accountRepo: repo, billingService: newTestBillingService()}).BuildSystemCustomGroupModelCatalog(
			context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "",
		)
		require.NoError(t, err)
		_, defaultAdvertised := catalog.Resolve("gpt-5.4")
		require.True(t, defaultAdvertised)
		_, mappedAdvertised := catalog.Resolve("mapped-only")
		require.False(t, mappedAdvertised)
	})

	t.Run("custom list allows and filters mapped models", func(t *testing.T) {
		source := directSourceGroup(10, PlatformOpenAI)
		source.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"Allowed", "Not-Mapped"}}
		repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
			{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"Allowed": "gpt-5.4", "Hidden": "gpt-5.4"})},
		}}
		catalog, err := (&GatewayService{accountRepo: repo, billingService: newTestBillingService()}).BuildSystemCustomGroupModelCatalog(
			context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "",
		)
		require.NoError(t, err)
		require.Equal(t, []string{"Allowed"}, catalog.Models())
		_, hiddenAdvertised := catalog.Resolve("Hidden")
		require.False(t, hiddenAdvertised)
		_, missingAdvertised := catalog.Resolve("Not-Mapped")
		require.False(t, missingAdvertised)
	})
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogAdvertisesInvalidSourcesButHonorsPlatform(t *testing.T) {
	inactive := directSourceGroup(10, PlatformOpenAI)
	inactive.Status = StatusDisabled
	inactive.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"inactive"}}
	nested := directSourceGroup(20, PlatformOpenAI)
	nested.SystemCustomRoutingEnabled = true
	nested.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"nested"}}
	unsupported := directSourceGroup(30, "kiro")
	unsupported.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"unsupported"}}
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
