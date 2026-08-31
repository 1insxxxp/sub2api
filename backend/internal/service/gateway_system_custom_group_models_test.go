//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
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

func TestGatewayBuildDynamicSystemCustomGroupCatalogRetainsOnlyPricedAccountIDs(t *testing.T) {
	source := directSourceGroup(10, PlatformOpenAI)
	unpriced := schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"shared": "vendor-unpriced"})
	unpriced.Priority = 1
	priced := schedulableSystemCustomTestAccount(102, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	priced.Priority = 2
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: unpriced},
		{GroupID: 10, Account: priced},
	}}

	catalog, err := (&GatewayService{accountRepo: repo, billingService: newTestBillingService()}).BuildSystemCustomGroupModelCatalog(
		context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, Priority: 1, SourceGroup: source}}, "",
	)

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, []int64{102}, candidates[0].AllowedAccounts.IDs())
}

func TestDynamicSystemCustomCatalogAndDispatchUseSavedGroupExplicitPricing(t *testing.T) {
	groupID := int64(10)
	group := directSourceGroup(groupID, PlatformAnthropic)
	group.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"explicit-only-model"}}
	group.ModelPricing = []ChannelModelPricing{{
		Platform: PlatformAnthropic, Models: []string{"explicit-only-model"}, BillingMode: BillingModeToken,
		InputPrice: coveragePrice(1e-6),
	}}
	account := schedulableSystemCustomTestAccount(101, PlatformAnthropic, map[string]any{"explicit-only-model": "vendor-unpriced"})
	batchRepo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{{GroupID: groupID, Account: account}}}
	billing := newTestBillingService()
	resolver := NewModelPricingResolver(nil, billing)
	catalogGateway := &GatewayService{accountRepo: batchRepo, billingService: billing, resolver: resolver}

	catalog, err := catalogGateway.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{{
		SourceGroupID: groupID, Priority: 1, SourceGroup: group,
	}}, "")

	require.NoError(t, err)
	require.Equal(t, []string{"explicit-only-model"}, catalog.Models())
	candidates, advertised := catalog.Resolve("explicit-only-model")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, []int64{account.ID}, candidates[0].AllowedAccounts.IDs())

	dispatch := &GatewayService{
		accountRepo:    &systemCustomDispatchAccountRepoStub{accounts: []Account{account}},
		groupRepo:      &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: group}},
		billingService: billing, resolver: resolver, cfg: testConfig(),
	}
	ctx := WithSystemCustomGroupResolution(context.Background(), SystemCustomGroupResolution{
		BillingGroupID: 99, SourceGroupID: groupID, PublicModel: "explicit-only-model", SourceModel: "explicit-only-model",
		SourcePlatform: PlatformAnthropic, AllowedAccounts: candidates[0].AllowedAccounts,
	})

	selected, err := dispatch.SelectAccountForModelWithExclusions(ctx, &groupID, "", "explicit-only-model", nil)

	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, account.ID, selected.ID)
}

type systemCustomDispatchAccountRepoStub struct {
	AccountRepository
	accounts []Account
}

func (s *systemCustomDispatchAccountRepoStub) ListSchedulableByGroupIDAndPlatform(_ context.Context, _ int64, platform string) ([]Account, error) {
	return s.byPlatforms([]string{platform}), nil
}

func (s *systemCustomDispatchAccountRepoStub) ListSchedulableByGroupIDAndPlatforms(_ context.Context, _ int64, platforms []string) ([]Account, error) {
	return s.byPlatforms(platforms), nil
}

func (s *systemCustomDispatchAccountRepoStub) GetByID(_ context.Context, accountID int64) (*Account, error) {
	for i := range s.accounts {
		if s.accounts[i].ID == accountID {
			account := s.accounts[i]
			return &account, nil
		}
	}
	return nil, ErrAccountNotFound
}

func (s *systemCustomDispatchAccountRepoStub) byPlatforms(platforms []string) []Account {
	allowed := make(map[string]struct{}, len(platforms))
	for _, platform := range platforms {
		allowed[platform] = struct{}{}
	}
	result := make([]Account, 0, len(s.accounts))
	for i := range s.accounts {
		if _, ok := allowed[s.accounts[i].Platform]; ok {
			result = append(result, s.accounts[i])
		}
	}
	return result
}

func TestSystemCustomAllowedAccountsBindGatewayDispatchToPricedAccount(t *testing.T) {
	groupID := int64(10)
	group := directSourceGroup(groupID, PlatformAnthropic)
	unpriced := schedulableSystemCustomTestAccount(101, PlatformAnthropic, map[string]any{"shared": "vendor-unpriced"})
	unpriced.Priority = 1
	priced := schedulableSystemCustomTestAccount(102, PlatformAnthropic, map[string]any{"shared": "claude-sonnet-4"})
	priced.Priority = 2
	batchRepo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: groupID, Account: unpriced},
		{GroupID: groupID, Account: priced},
	}}
	catalogGateway := &GatewayService{accountRepo: batchRepo, billingService: newTestBillingService()}
	catalog, err := catalogGateway.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: groupID, Priority: 1, SourceGroup: group},
	}, "")
	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)

	ctx := WithSystemCustomGroupResolution(context.Background(), SystemCustomGroupResolution{
		BillingGroupID: 99, SourceGroupID: groupID, PublicModel: "shared", SourceModel: "shared",
		SourcePlatform: PlatformAnthropic, AllowedAccounts: candidates[0].AllowedAccounts,
	})
	dispatchRepo := &systemCustomDispatchAccountRepoStub{accounts: []Account{unpriced, priced}}
	dispatch := &GatewayService{
		accountRepo:    dispatchRepo,
		groupRepo:      &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: group}},
		billingService: newTestBillingService(), cfg: testConfig(),
	}

	selected, err := dispatch.SelectAccountForModelWithExclusions(ctx, &groupID, "", "shared", nil)

	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, int64(102), selected.ID)
}

func TestSystemCustomAllowedAccountsBindOpenAIDispatchToPricedAccount(t *testing.T) {
	groupID := int64(10)
	unpriced := schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"shared": "vendor-unpriced"})
	unpriced.Priority = 1
	priced := schedulableSystemCustomTestAccount(202, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	priced.Priority = 2
	ctx := WithSystemCustomGroupResolution(context.Background(), SystemCustomGroupResolution{
		BillingGroupID: 99, SourceGroupID: groupID, PublicModel: "shared", SourceModel: "shared",
		SourcePlatform: PlatformOpenAI, AllowedAccounts: NewSystemCustomGroupAccountAllowlist([]int64{priced.ID}),
	})
	dispatch := &OpenAIGatewayService{
		accountRepo:    &systemCustomDispatchAccountRepoStub{accounts: []Account{unpriced, priced}},
		billingService: newTestBillingService(), cfg: testConfig(),
	}

	selected, err := dispatch.SelectAccountForModelWithExclusions(ctx, &groupID, "", "shared", nil)

	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, int64(202), selected.ID)
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

type systemCustomSchedulerCacheStub struct {
	SchedulerCache
	snapshots map[SchedulerBucket][]*Account
	calls     int
}

func (s *systemCustomSchedulerCacheStub) GetSnapshot(_ context.Context, bucket SchedulerBucket) ([]*Account, bool, error) {
	s.calls++
	accounts, hit := s.snapshots[bucket]
	if !hit {
		return nil, false, nil
	}
	cloned := make([]*Account, 0, len(accounts))
	for _, account := range accounts {
		if account == nil {
			continue
		}
		copy := *account
		cloned = append(cloned, &copy)
	}
	return cloned, true, nil
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFallsBackWhenSchedulerSnapshotOmitsStaleFirstSource(t *testing.T) {
	first := directSourceGroup(10, PlatformOpenAI)
	second := directSourceGroup(20, PlatformOpenAI)
	stale := schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	available := schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: stale},
		{GroupID: 20, Account: available},
	}}
	cache := &systemCustomSchedulerCacheStub{snapshots: map[SchedulerBucket][]*Account{
		{GroupID: 10, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}: {},
		{GroupID: 20, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}: {&available},
	}}
	svc := &GatewayService{
		accountRepo: repo, billingService: newTestBillingService(),
		schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
	}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: first},
		{SourceGroupID: 20, Priority: 2, SourceGroup: second},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(20), candidates[0].SourceGroup.ID)
	require.Equal(t, 1, repo.calls)
	require.Equal(t, 2, cache.calls, "scheduler buckets must be read once per source, never per model")
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFallsBackWhenOpenAIRuntimeBlockRejectsFirstSource(t *testing.T) {
	first := directSourceGroup(10, PlatformOpenAI)
	second := directSourceGroup(20, PlatformOpenAI)
	blocked := schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	available := schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: blocked},
		{GroupID: 20, Account: available},
	}}
	cache := &systemCustomSchedulerCacheStub{snapshots: map[SchedulerBucket][]*Account{
		{GroupID: 10, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}: {&blocked},
		{GroupID: 20, Platform: PlatformOpenAI, Mode: SchedulerModeSingle}: {&available},
	}}
	openAI := &OpenAIGatewayService{}
	openAI.BlockAccountScheduling(&blocked, time.Now().Add(time.Minute), "test runtime block")
	svc := &GatewayService{
		accountRepo: repo, billingService: newTestBillingService(),
		schedulerSnapshot: &SchedulerSnapshotService{cache: cache},
	}
	svc.SetSystemCustomOpenAIRuntimeEligibilityProbe(openAI)

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: first},
		{SourceGroupID: 20, Priority: 2, SourceGroup: second},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(20), candidates[0].SourceGroup.ID)
	require.Equal(t, 1, repo.calls)
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFallsBackWhenSchedulingThresholdBlocksFirstSource(t *testing.T) {
	first := directSourceGroup(10, PlatformOpenAI)
	second := directSourceGroup(20, PlatformOpenAI)
	blocked := schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	blocked.Credentials["account_scheduling_threshold"] = 1
	blocked.Extra = map[string]any{
		"codex_7d_used_percent": 91.5,
		"codex_7d_reset_at":     time.Now().UTC().Add(6 * time.Hour).Format(time.RFC3339),
	}
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: blocked},
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})},
	}}
	cfg := &config.Config{}
	rateLimits := NewRateLimitService(&rateLimitAccountRepoStub{}, nil, cfg, nil, nil)
	rateLimits.SetSettingService(NewSettingService(nil, cfg))
	svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService(), rateLimitService: rateLimits}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: first},
		{SourceGroupID: 20, Priority: 2, SourceGroup: second},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(20), candidates[0].SourceGroup.ID)
	require.Nil(t, repo.accounts[0].Account.TempUnschedulableUntil, "catalog filtering must not mutate the repository snapshot")
	require.Equal(t, 1, repo.calls, "threshold filtering must reuse the one batched account snapshot")
}

func TestGatewayBuildDynamicSystemCustomGroupCatalogFallsBackWhenGrokFreeQuotaBlocksFirstSource(t *testing.T) {
	first := directSourceGroup(10, PlatformGrok)
	second := directSourceGroup(20, PlatformGrok)
	blocked := schedulableSystemCustomTestAccount(91001, PlatformGrok, map[string]any{"shared": "grok-3-mini"})
	blocked.Type = AccountTypeOAuth
	blocked.Credentials["subscription_tier"] = "free"
	available := schedulableSystemCustomTestAccount(92001, PlatformGrok, map[string]any{"shared": "grok-3-mini"})
	available.Type = AccountTypeOAuth
	available.Credentials["subscription_tier"] = "pro"
	gatewayGrokFreeQuotaGateCache.Store(blocked.ID, grokFreeQuotaGateCacheEntry{
		tokens: 475_000, checkedAt: time.Now().UTC(), known: true,
	})
	t.Cleanup(func() { gatewayGrokFreeQuotaGateCache.Delete(blocked.ID) })
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 10, Account: blocked},
		{GroupID: 20, Account: available},
	}}
	svc := &GatewayService{
		accountRepo: repo, billingService: newTestBillingService(),
		cfg: grokFreeQuotaTestConfig(), usageLogRepo: &grokFreeQuotaUsageRepoStub{},
	}

	catalog, err := svc.BuildSystemCustomGroupModelCatalog(context.Background(), []SystemCustomGroupSource{
		{SourceGroupID: 10, Priority: 1, SourceGroup: first},
		{SourceGroupID: 20, Priority: 2, SourceGroup: second},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, int64(20), candidates[0].SourceGroup.ID)
	require.Equal(t, 1, repo.calls, "Grok quota filtering must reuse the one batched account snapshot")
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

func TestGatewayResolveDynamicSystemCustomGroupModelUsesDefaultAndCustomModelListSemantics(t *testing.T) {
	t.Run("empty mapping uses defaults", func(t *testing.T) {
		source := directSourceGroup(10, PlatformOpenAI)
		repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
			{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, nil)},
		}}
		svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

		candidates, advertised, err := svc.ResolveSystemCustomGroupModelCatalog(
			context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "", "gpt-5.4",
		)

		require.NoError(t, err)
		require.True(t, advertised)
		require.Len(t, candidates, 1)
		require.Equal(t, 1, repo.calls)
	})

	t.Run("openai passthrough uses defaults", func(t *testing.T) {
		source := directSourceGroup(10, PlatformOpenAI)
		account := schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"mapped-only": "gpt-5.4"})
		account.Extra = map[string]any{"openai_passthrough": true}
		repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{{GroupID: 10, Account: account}}}
		svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

		candidates, advertised, err := svc.ResolveSystemCustomGroupModelCatalog(
			context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "", "gpt-5.4",
		)

		require.NoError(t, err)
		require.True(t, advertised)
		require.Len(t, candidates, 1)
		_, mappedAdvertised, err := svc.ResolveSystemCustomGroupModelCatalog(
			context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "", "mapped-only",
		)
		require.NoError(t, err)
		require.False(t, mappedAdvertised)
	})

	t.Run("custom list preserves configured spelling and filters mappings", func(t *testing.T) {
		source := directSourceGroup(10, PlatformOpenAI)
		source.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"Allowed", "Not-Mapped"}}
		repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
			{GroupID: 10, Account: schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"Allowed": "gpt-5.4", "Hidden": "gpt-5.4"})},
		}}
		svc := &GatewayService{accountRepo: repo, billingService: newTestBillingService()}

		candidates, advertised, err := svc.ResolveSystemCustomGroupModelCatalog(
			context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "", "allowed",
		)

		require.NoError(t, err)
		require.True(t, advertised)
		require.Len(t, candidates, 1)
		require.Equal(t, "Allowed", candidates[0].PublicModel)
		for _, model := range []string{"Hidden", "Not-Mapped"} {
			_, advertised, err := svc.ResolveSystemCustomGroupModelCatalog(
				context.Background(), []SystemCustomGroupSource{{SourceGroupID: 10, SourceGroup: source}}, "", model,
			)
			require.NoError(t, err)
			require.False(t, advertised, model)
		}
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
