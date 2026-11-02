//go:build unit

package handler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// These fixtures exercise the real gateway schedulers. No HTTP upstream or
// customer credentials are installed, so selection cannot issue a paid request.
type probeGatewayAccounts struct {
	service.AccountRepository
	byGroup map[int64][]service.Account
	queries []int64
}

func (r *probeGatewayAccounts) GetByID(_ context.Context, id int64) (*service.Account, error) {
	for _, accounts := range r.byGroup {
		for _, account := range accounts {
			if account.ID == id {
				clone := account
				return &clone, nil
			}
		}
	}
	return nil, service.ErrNoAvailableAccounts
}

func (r *probeGatewayAccounts) ListSchedulableByGroupID(_ context.Context, id int64) ([]service.Account, error) {
	r.queries = append(r.queries, id)
	return append([]service.Account(nil), r.byGroup[id]...), nil
}

func (r *probeGatewayAccounts) ListSchedulableByGroupIDAndPlatform(ctx context.Context, id int64, platform string) ([]service.Account, error) {
	return r.ListSchedulableByGroupIDAndPlatforms(ctx, id, []string{platform})
}

func (r *probeGatewayAccounts) ListSchedulableByGroupIDAndPlatforms(ctx context.Context, id int64, platforms []string) ([]service.Account, error) {
	accounts, _ := r.ListSchedulableByGroupID(ctx, id)
	var out []service.Account
	for _, account := range accounts {
		for _, platform := range platforms {
			if account.Platform == platform {
				out = append(out, account)
				break
			}
		}
	}
	return out, nil
}

func (r *probeGatewayAccounts) ListModelAvailabilityCandidates(ctx context.Context, id *int64, _ []string, _ bool) ([]service.Account, error) {
	if id == nil {
		return nil, service.ErrNoAvailableAccounts
	}
	return r.ListSchedulableByGroupID(ctx, *id)
}

type probeGatewayChannels struct {
	service.ChannelRepository
	channel  service.Channel
	platform string
}

func (r probeGatewayChannels) ListAll(context.Context) ([]service.Channel, error) {
	return []service.Channel{r.channel}, nil
}
func (r probeGatewayChannels) GetGroupPlatforms(_ context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		out[id] = r.platform
	}
	return out, nil
}

type probeGatewaySlots struct {
	helperConcurrencyCacheStub
	idsMu sync.Mutex
	ids   []int64
}

func (s *probeGatewaySlots) AcquireAccountSlot(ctx context.Context, id int64, max int, request string) (bool, error) {
	s.idsMu.Lock()
	s.ids = append(s.ids, id)
	s.idsMu.Unlock()
	return s.helperConcurrencyCacheStub.AcquireAccountSlot(ctx, id, max, request)
}

func probeGatewayFixture(repo *probeGatewayAccounts, channel *service.ChannelService, routes service.CompositeModelRouteRepository, slots *probeGatewaySlots) *GatewayHandler {
	cfg := &config.Config{RunMode: config.RunModeStandard}
	concurrency := service.NewConcurrencyService(slots)
	var resolver *service.CompositeRouteResolver
	if routes != nil {
		resolver = service.NewCompositeRouteResolver(routes)
	}
	gateway := service.NewGatewayService(repo, nil, nil, nil, nil, nil, nil, nil, cfg,
		nil, concurrency, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, channel, nil, resolver, nil, nil)
	openAI := service.NewOpenAIGatewayService(repo, nil, nil, nil, nil, nil, nil, cfg,
		nil, concurrency, nil, nil, nil, nil, nil, nil, nil, nil, channel, nil, nil, nil)
	return &GatewayHandler{gatewayService: gateway, openAIGatewayService: openAI,
		concurrencyHelper: NewConcurrencyHelper(concurrency, SSEPingFormatClaude, 0)}
}

func probeGatewayAccount(id, group int64, platform, public, upstream string) service.Account {
	return service.Account{ID: id, Platform: platform, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true, Concurrency: 1, GroupIDs: []int64{group},
		Credentials: map[string]any{"model_mapping": map[string]any{public: upstream}}}
}

func TestModelProbeGatewayMapsGroupAndChannelBeforeOpenAISelection(t *testing.T) {
	group := &service.Group{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true,
		ModelAliases: []domain.ModelAlias{{Alias: "public-model", UpstreamModel: "route-model"}}}
	wrong := probeGatewayAccount(71, 7, service.PlatformOpenAI, "route-model", "wrong-native")
	right := probeGatewayAccount(72, 7, service.PlatformOpenAI, "channel-model", "native-model")
	outside := probeGatewayAccount(81, 8, service.PlatformOpenAI, "channel-model", "other-native")
	repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {wrong, right}, 8: {outside}}}
	channels := service.NewChannelService(probeGatewayChannels{channel: service.Channel{ID: 1, Status: service.StatusActive, GroupIDs: []int64{7},
		ModelMapping: map[string]map[string]string{service.PlatformOpenAI: {"route-model": "channel-model"}}}, platform: service.PlatformOpenAI}, nil, nil, nil, nil)
	slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
	h := probeGatewayFixture(repo, channels, nil, slots)
	selection, err := h.SelectForProbe(context.Background(), group, "public-model")
	require.NoError(t, err)
	require.Equal(t, right.ID, selection.AccountID)
	// AccountTestService applies account mapping itself: passing native-model
	// here would map a chain twice instead of sending the same native test.
	require.Equal(t, "channel-model", selection.Model)
	require.Equal(t, "native-model", right.GetMappedModel(selection.Model))
	require.NotEmpty(t, repo.queries)
	for _, id := range repo.queries {
		require.Equal(t, group.ID, id)
	}
	selection.Release()
	selection.Release()
	slots.mu.Lock()
	releases := slots.accountReleaseCalls
	slots.mu.Unlock()
	require.Equal(t, 1, releases)
}

func TestModelProbeGatewayKeepsCompositeTargetAndReleasesOnCancel(t *testing.T) {
	group := &service.Group{ID: 7, Platform: service.PlatformComposite, Status: service.StatusActive, Hydrated: true}
	account := probeGatewayAccount(71, 7, service.PlatformGemini, "private-route-model", "native-model")
	repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {account}}}
	routes := &gatewayModelsRouteRepoStub{routes: []service.CompositeModelRoute{{ID: 1, GroupID: 7, Enabled: true,
		MatchType: service.CompositeRouteMatchExact, PublicModel: "public-model", TargetPlatform: service.PlatformGemini,
		UpstreamModel: "private-route-model", Endpoint: service.CompositeRouteEndpointGemini}}}
	slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
	h := probeGatewayFixture(repo, nil, routes, slots)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	selection, err := h.SelectForProbe(ctx, group, "public-model")
	require.NoError(t, err)
	require.Equal(t, account.ID, selection.AccountID)
	require.Equal(t, "private-route-model", selection.Model)
	cancel()
	require.Eventually(t, func() bool { slots.mu.Lock(); defer slots.mu.Unlock(); return slots.accountReleaseCalls == 1 }, time.Second, time.Millisecond)
	selection.Release()
	slots.mu.Lock()
	releases := slots.accountReleaseCalls
	slots.mu.Unlock()
	require.Equal(t, 1, releases)
}

func TestModelProbeGatewayCatalogHonorsPublicAllowlistAndGroup(t *testing.T) {
	group := &service.Group{ID: 7, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true,
		ModelAliases:   []domain.ModelAlias{{Alias: "public-model", UpstreamModel: "route-model"}},
		ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"public-model"}}}
	repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{
		7: {probeGatewayAccount(71, 7, service.PlatformGemini, "route-model", "native-model")},
		8: {probeGatewayAccount(81, 8, service.PlatformGemini, "other-group-model", "other-native")}}}
	h := probeGatewayFixture(repo, nil, nil, &probeGatewaySlots{})
	models, err := h.ModelsForProbe(context.Background(), group)
	require.NoError(t, err)
	require.Equal(t, []string{"public-model"}, models)
	for _, id := range repo.queries {
		require.Equal(t, group.ID, id)
	}
}

func TestModelProbeGatewayRejectsDisabledImagesAfterAccountMapping(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			group := &service.Group{ID: 7, Platform: platform, Status: service.StatusActive, Hydrated: true}
			native := "gpt-image-1"
			if platform == service.PlatformGemini {
				native = "gemini-2.5-flash-image"
			}
			account := probeGatewayAccount(71, 7, platform, "public-picture", native)
			repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {account}}}
			slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
			h := probeGatewayFixture(repo, nil, nil, slots)
			selection, err := h.SelectForProbe(context.Background(), group, "public-picture")
			require.ErrorIs(t, err, service.ErrModelProbeUnsupported)
			require.Nil(t, selection)
			slots.mu.Lock()
			releases := slots.accountReleaseCalls
			slots.mu.Unlock()
			require.Equal(t, 1, releases, "a disabled final image model must release its selected slot")
		})
	}
}

func TestModelProbeGatewayNativeSelectionHonorsGroupProfitGate(t *testing.T) {
	for _, tc := range []struct {
		name string
		rate float64
	}{{"over threshold", 1}, {"within threshold", 0.1}} {
		t.Run(tc.name, func(t *testing.T) {
			rate := tc.rate
			group := &service.Group{ID: 7, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true,
				RateMultiplier: 1, ProfitControlEnabled: true, ProfitMinMargin: 0.5}
			account := probeGatewayAccount(71, 7, service.PlatformGemini, "public-model", "native-model")
			account.RateMultiplier = &rate
			repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {account}}}
			slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
			h := probeGatewayFixture(repo, nil, nil, slots)
			selection, err := h.SelectForProbe(context.Background(), group, "public-model")
			if rate > 0.5 {
				require.Error(t, err)
				require.Nil(t, selection)
				slots.mu.Lock()
				acquired := slots.accountAcquireCalls
				slots.mu.Unlock()
				require.Zero(t, acquired, "profit-rejected models must not take a business slot")
				return
			}
			require.NoError(t, err)
			require.Equal(t, account.ID, selection.AccountID)
			selection.Release()
		})
	}
}

func TestModelProbeGatewayIncludesOpenAIChatOnlyNativeDefault(t *testing.T) {
	group := &service.Group{ID: 7, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
	account := probeGatewayAccount(71, 7, service.PlatformOpenAI, "public-model", "native-model")
	account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: false}
	require.False(t, account.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityResponses))
	require.True(t, account.SupportsOpenAIEndpointCapability(service.OpenAIEndpointCapabilityChatCompletions))
	repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {account}}}
	slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
	h := probeGatewayFixture(repo, nil, nil, slots)
	selection, err := h.SelectForProbe(context.Background(), group, "public-model")
	require.NoError(t, err, "a native/default probe must retain chat-only API-key accounts")
	require.Equal(t, account.ID, selection.AccountID)
	require.Equal(t, "public-model", selection.Model)
	selection.Release()
}

func TestModelProbeGatewayDoesNotClaimDifferentExplicitEndpointPassed(t *testing.T) {
	group := &service.Group{ID: 7, Platform: service.PlatformComposite, Status: service.StatusActive, Hydrated: true}
	account := probeGatewayAccount(71, 7, service.PlatformOpenAI, "route-model", "native-model")
	account.Extra = map[string]any{openai_compat.ExtraKeyResponsesSupported: true}
	repo := &probeGatewayAccounts{byGroup: map[int64][]service.Account{7: {account}}}
	routes := &gatewayModelsRouteRepoStub{routes: []service.CompositeModelRoute{{ID: 1, GroupID: 7, Enabled: true,
		MatchType: service.CompositeRouteMatchExact, PublicModel: "public-model", TargetPlatform: service.PlatformOpenAI,
		UpstreamModel: "route-model", Endpoint: service.CompositeRouteEndpointChatCompletions}}}
	slots := &probeGatewaySlots{helperConcurrencyCacheStub: helperConcurrencyCacheStub{accountSeq: []bool{true}}}
	h := probeGatewayFixture(repo, nil, routes, slots)
	selection, err := h.SelectForProbe(context.Background(), group, "public-model")
	require.ErrorIs(t, err, service.ErrModelProbeUnsupported, "a default Responses test does not establish an explicit Chat Completions route")
	require.Nil(t, selection)
	slots.mu.Lock()
	releases := slots.accountReleaseCalls
	slots.mu.Unlock()
	require.Equal(t, 1, releases)
}
