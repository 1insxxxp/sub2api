package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type systemCustomRuntimeAccountRepo struct {
	service.AccountRepository
	accounts []service.SystemCustomGroupSchedulableAccount
}

func (r *systemCustomRuntimeAccountRepo) ListSchedulableByGroupIDs(_ context.Context, _ []int64) ([]service.SystemCustomGroupSchedulableAccount, error) {
	return append([]service.SystemCustomGroupSchedulableAccount(nil), r.accounts...), nil
}

type systemCustomModelsRouteRepo struct {
	service.SystemCustomGroupRepository
	group *service.SystemCustomGroup
}

func (r systemCustomModelsRouteRepo) Get(context.Context, int64) (*service.SystemCustomGroup, error) {
	return r.group, nil
}

func (r systemCustomModelsRouteRepo) GetRuntime(context.Context, int64) (*service.SystemCustomGroup, error) {
	clone := *r.group
	clone.Models = nil
	return &clone, nil
}

type systemCustomModelsGroupRepo struct {
	service.GroupRepository
	groups map[int64]*service.Group
}

func (r systemCustomModelsGroupRepo) GetByIDLite(_ context.Context, id int64) (*service.Group, error) {
	group, ok := r.groups[id]
	if !ok {
		return nil, service.ErrGroupNotFound
	}
	clone := *group
	return &clone, nil
}

type systemCustomModelsCatalog struct {
	models        map[int64][]string
	unschedulable map[int64]bool
}

func (c systemCustomModelsCatalog) GetAvailableModels(_ context.Context, groupID *int64, _ string) []string {
	if groupID == nil {
		return nil
	}
	return append([]string(nil), c.models[*groupID]...)
}

func (c systemCustomModelsCatalog) HasSchedulableAccountsForGroupPlatform(_ context.Context, groupID int64, _ string) bool {
	return !c.unschedulable[groupID]
}

func (c systemCustomModelsCatalog) BuildSystemCustomGroupModelCatalog(_ context.Context, sources []service.SystemCustomGroupSource, platform string) (*service.SystemCustomGroupRuntimeCatalog, error) {
	candidates := make([]service.SystemCustomGroupRuntimeCandidate, 0)
	advertised := make([]string, 0)
	for _, source := range sources {
		if source.SourceGroup == nil || c.unschedulable[source.SourceGroupID] ||
			(platform != "" && source.SourceGroup.Platform != platform) {
			continue
		}
		for _, model := range c.models[source.SourceGroupID] {
			advertised = append(advertised, model)
			candidates = append(candidates, service.SystemCustomGroupRuntimeCandidate{
				SourceGroup: *source.SourceGroup, PublicModel: model, SourceModel: model,
			})
		}
	}
	return service.NewSystemCustomGroupRuntimeCatalog(candidates, advertised), nil
}

func (c systemCustomModelsCatalog) ResolveSystemCustomGroupModelCatalog(_ context.Context, sources []service.SystemCustomGroupSource, platform, model string) ([]service.SystemCustomGroupRuntimeCandidate, bool, error) {
	catalog, err := c.BuildSystemCustomGroupModelCatalog(context.Background(), sources, platform)
	if err != nil {
		return nil, false, err
	}
	candidates, advertised := catalog.Resolve(model)
	return candidates, advertised, nil
}

func TestSystemCustomGroupModelsExposesAvailableAliasesOnly(t *testing.T) {
	handler, key := newSystemCustomModelsHandler(t)

	for _, target := range []string{"/models", "/v1/models"} {
		t.Run(target, func(t *testing.T) {
			recorder, c := systemCustomModelsRequestContext(t, key, target)
			handler.Models(c)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "list", gjson.Get(recorder.Body.String(), "object").String())
			require.Equal(t, []string{"claude-sonnet-4", "gemini-2.5-flash"}, gjsonStrings(recorder.Body.String(), "data.#.id"))
			for _, item := range gjson.Get(recorder.Body.String(), "data").Array() {
				require.Equal(t, "model", item.Get("object").String())
				require.Equal(t, gjson.Number, item.Get("created").Type)
				require.Positive(t, item.Get("created").Int())
				require.NotEmpty(t, item.Get("owned_by").String())
			}
			require.NotContains(t, recorder.Body.String(), "retained-static")
		})
	}
}

func TestSystemCustomCodexModelsExposesAliasManifestOnly(t *testing.T) {
	handler, key := newSystemCustomModelsHandler(t)
	recorder, c := systemCustomModelsRequestContext(t, key, "/v1/models?client_version=0.144.0")

	handler.SystemCustomCodexModels(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []string{"claude-sonnet-4", "gemini-2.5-flash"}, gjsonStrings(recorder.Body.String(), "models.#.slug"))
	require.Equal(t, []string{"claude-sonnet-4", "gemini-2.5-flash"}, gjsonStrings(recorder.Body.String(), "models.#.display_name"))
	require.False(t, gjson.Get(recorder.Body.String(), "object").Exists())
	require.NotContains(t, recorder.Body.String(), "retained-static")
}

func TestNewGatewayHandlerWiresOpenAIRuntimeBlocksIntoDynamicCatalog(t *testing.T) {
	firstGroup := &service.Group{ID: 10, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
	secondGroup := &service.Group{ID: 20, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true}
	firstAccount := service.Account{
		ID: 101, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"shared": "gpt-5.4"}},
	}
	secondAccount := service.Account{
		ID: 201, Platform: service.PlatformOpenAI, Type: service.AccountTypeAPIKey,
		Status: service.StatusActive, Schedulable: true,
		Credentials: map[string]any{"model_mapping": map[string]any{"shared": "gpt-5.4"}},
	}
	repo := &systemCustomRuntimeAccountRepo{accounts: []service.SystemCustomGroupSchedulableAccount{
		{GroupID: firstGroup.ID, Account: firstAccount},
		{GroupID: secondGroup.ID, Account: secondAccount},
	}}
	cfg := &config.Config{}
	billing := service.NewBillingService(cfg, nil)
	gateway := service.NewGatewayService(
		repo, nil, nil, nil, nil, nil, nil, nil, cfg,
		nil, nil, billing, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	openAI := service.NewOpenAIGatewayService(
		nil, nil, nil, nil, nil, nil, nil, cfg, nil, nil, nil, nil, nil,
		nil, nil, nil, nil, nil, nil, nil, nil, nil,
	)
	openAI.BlockAccountScheduling(&firstAccount, time.Now().Add(time.Minute), "test")
	NewGatewayHandler(gateway, openAI, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, cfg, nil)

	catalog, err := gateway.BuildSystemCustomGroupModelCatalog(context.Background(), []service.SystemCustomGroupSource{
		{SourceGroupID: firstGroup.ID, Priority: 1, SourceGroup: firstGroup},
		{SourceGroupID: secondGroup.ID, Priority: 2, SourceGroup: secondGroup},
	}, "")

	require.NoError(t, err)
	candidates, advertised := catalog.Resolve("shared")
	require.True(t, advertised)
	require.Len(t, candidates, 1)
	require.Equal(t, secondGroup.ID, candidates[0].SourceGroup.ID)
}

func gjsonStrings(body, path string) []string {
	results := gjson.Get(body, path).Array()
	values := make([]string, 0, len(results))
	for _, result := range results {
		values = append(values, result.String())
	}
	return values
}

func newSystemCustomModelsHandler(t *testing.T) (*GatewayHandler, *service.APIKey) {
	t.Helper()
	billingGroupID := int64(25)
	groupRepo := systemCustomModelsGroupRepo{groups: map[int64]*service.Group{
		10: {ID: 10, Platform: service.PlatformAnthropic, Status: service.StatusActive, Hydrated: true},
		20: {ID: 20, Platform: service.PlatformGemini, Status: service.StatusActive, Hydrated: true},
		30: {ID: 30, Platform: service.PlatformOpenAI, Status: service.StatusActive, Hydrated: true},
	}}
	container := &service.Group{
		ID: billingGroupID, Platform: service.PlatformComposite, Status: service.StatusActive, Hydrated: true,
		SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
	}
	routeRepo := systemCustomModelsRouteRepo{group: &service.SystemCustomGroup{
		Group: *container,
		Sources: []service.SystemCustomGroupSource{
			{SourceGroupID: 20, Priority: 2, SourceGroup: groupRepo.groups[20]},
			{SourceGroupID: 10, Priority: 1, SourceGroup: groupRepo.groups[10]},
			{SourceGroupID: 30, Priority: 3, SourceGroup: groupRepo.groups[30]},
		},
		Models: []service.SystemCustomGroupModel{{PublicModel: "retained-static", Enabled: true}},
	}}
	catalog := systemCustomModelsCatalog{
		models: map[int64][]string{
			10: {"claude-sonnet-4"},
			20: {"gemini-2.5-flash"},
			30: {"gpt-5.6"},
		},
		unschedulable: map[int64]bool{30: true},
	}
	apiKeyService := service.NewAPIKeyService(nil, nil, groupRepo, nil, nil, nil, nil)
	apiKeyService.SetSystemCustomGroupRepository(routeRepo)
	apiKeyService.SetSystemCustomGroupModelCatalog(catalog)
	key := &service.APIKey{
		GroupID: &billingGroupID,
		Group:   container,
	}
	return &GatewayHandler{apiKeyService: apiKeyService}, key
}

func systemCustomModelsRequestContext(t *testing.T, key *service.APIKey, target string) (*httptest.ResponseRecorder, *gin.Context) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	c.Set(string(middleware2.ContextKeyAPIKey), key)
	return recorder, c
}
