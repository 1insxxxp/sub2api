package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type systemCustomModelsRouteRepo struct {
	service.SystemCustomGroupRepository
	routes []service.SystemCustomGroupModel
	groups map[int64]*service.Group
}

func (r systemCustomModelsRouteRepo) ListModels(context.Context, int64, bool) ([]service.SystemCustomGroupModel, error) {
	routes := append([]service.SystemCustomGroupModel(nil), r.routes...)
	for i := range routes {
		routes[i].SourceGroup = r.groups[routes[i].SourceGroupID]
	}
	return routes, nil
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

func (c systemCustomModelsCatalog) ListSystemCustomGroupModelAvailability(_ context.Context, sources []service.SystemCustomGroupModelListSource) (service.SystemCustomGroupModelAvailability, error) {
	availability := make(service.SystemCustomGroupModelAvailability, len(sources))
	for _, source := range sources {
		if c.unschedulable[source.Group.ID] {
			continue
		}
		available := make(map[string]bool, len(source.Models))
		for _, sourceModel := range source.Models {
			for _, model := range c.models[source.Group.ID] {
				if model == sourceModel {
					available[sourceModel] = true
				}
			}
		}
		availability[source.Group.ID] = available
	}
	return availability, nil
}

func TestSystemCustomGroupModelsExposesAvailableAliasesOnly(t *testing.T) {
	handler, key := newSystemCustomModelsHandler(t)

	for _, target := range []string{"/models", "/v1/models"} {
		t.Run(target, func(t *testing.T) {
			recorder, c := systemCustomModelsRequestContext(t, key, target)
			handler.Models(c)

			require.Equal(t, http.StatusOK, recorder.Code)
			require.Equal(t, "list", gjson.Get(recorder.Body.String(), "object").String())
			require.Equal(t, []string{"claude-monthly", "gemini-monthly"}, gjsonStrings(recorder.Body.String(), "data.#.id"))
			for _, item := range gjson.Get(recorder.Body.String(), "data").Array() {
				require.Equal(t, "model", item.Get("object").String())
				require.Equal(t, gjson.Number, item.Get("created").Type)
				require.Positive(t, item.Get("created").Int())
				require.NotEmpty(t, item.Get("owned_by").String())
			}
			require.NotContains(t, recorder.Body.String(), "claude-sonnet-4")
			require.NotContains(t, recorder.Body.String(), "gemini-2.5-flash")
		})
	}
}

func TestSystemCustomCodexModelsExposesAliasManifestOnly(t *testing.T) {
	handler, key := newSystemCustomModelsHandler(t)
	recorder, c := systemCustomModelsRequestContext(t, key, "/v1/models?client_version=0.144.0")

	handler.SystemCustomCodexModels(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []string{"claude-monthly", "gemini-monthly"}, gjsonStrings(recorder.Body.String(), "models.#.slug"))
	require.Equal(t, []string{"claude-monthly", "gemini-monthly"}, gjsonStrings(recorder.Body.String(), "models.#.display_name"))
	require.False(t, gjson.Get(recorder.Body.String(), "object").Exists())
	require.NotContains(t, recorder.Body.String(), "claude-sonnet-4")
	require.NotContains(t, recorder.Body.String(), "gemini-2.5-flash")
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
	routeRepo := systemCustomModelsRouteRepo{routes: []service.SystemCustomGroupModel{
		{GroupID: billingGroupID, PublicModel: "gemini-monthly", SourceGroupID: 20, SourceModel: "gemini-2.5-flash", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "claude-monthly", SourceGroupID: 10, SourceModel: "claude-sonnet-4", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "unavailable", SourceGroupID: 30, SourceModel: "gpt-5.6", Enabled: true},
		{GroupID: billingGroupID, PublicModel: "disabled", SourceGroupID: 10, SourceModel: "claude-sonnet-4", Enabled: false},
	}, groups: groupRepo.groups}
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
		Group: &service.Group{
			ID: billingGroupID, Platform: service.PlatformComposite, Status: service.StatusActive, Hydrated: true,
			SubscriptionType: service.SubscriptionTypeSubscription, SystemCustomRoutingEnabled: true,
		},
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
