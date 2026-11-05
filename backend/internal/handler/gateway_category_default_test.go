package handler

import (
	"context"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type categoryDefaultModelsRepository struct {
	service.CategoryDefaultGroupRepository
	config  service.CategoryDefaultGroupConfig
	sources []service.CategoryDefaultSourceSnapshot
}

func (r *categoryDefaultModelsRepository) Get(context.Context, string) (*service.CategoryDefaultGroupConfig, error) {
	return &r.config, nil
}
func (r *categoryDefaultModelsRepository) GetByGroupID(context.Context, int64) (*service.CategoryDefaultGroupConfig, error) {
	return &r.config, nil
}
func (r *categoryDefaultModelsRepository) Sources(context.Context, []int64) ([]service.CategoryDefaultSourceSnapshot, error) {
	return r.sources, nil
}

func (r *categoryDefaultModelsRepository) Snapshot(_ context.Context, req service.CategoryDefaultSnapshotRequest) (*service.CategoryDefaultCatalogSnapshot, error) {
	config := r.config
	return &service.CategoryDefaultCatalogSnapshot{Config: &config, Sources: r.sources}, nil
}

type categoryDefaultModelsUserRepository struct{ service.UserRepository }

func (categoryDefaultModelsUserRepository) GetByID(_ context.Context, id int64) (*service.User, error) {
	return &service.User{ID: id, Status: service.StatusActive}, nil
}

func TestCategoryDefaultCodexManifestFollowsCurrentAssociatedDeclarations(t *testing.T) {
	entryID, sourceID := int64(91), int64(12)
	repo := &categoryDefaultModelsRepository{
		config: service.CategoryDefaultGroupConfig{GroupID: entryID, Category: "gemini", Enabled: true, SourceGroupIDs: []int64{sourceID}},
		sources: []service.CategoryDefaultSourceSnapshot{{
			Group:    &service.Group{ID: sourceID, Platform: service.PlatformGemini, Hydrated: true, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard},
			Accounts: []service.Account{{ID: 1, Platform: service.PlatformGemini, Type: service.AccountTypeAPIKey, Status: service.StatusActive, Credentials: map[string]any{"model_mapping": map[string]any{"friendly-model": "gemini-2.5-flash", "hidden-pending": "gemini-2.5-pro"}}}},
		}},
	}
	groups := service.NewCategoryDefaultGroupService(repo, nil)
	repo.config.Models = []service.CategoryDefaultPublishedModel{{PublicModel: "friendly-model", SourceGroupID: sourceID, SourceModel: "friendly-model"}}
	keys := service.NewAPIKeyService(nil, categoryDefaultModelsUserRepository{}, nil, nil, nil, nil, nil)
	keys.SetCategoryDefaultGroupService(groups)
	h := &GatewayHandler{apiKeyService: keys}
	key := &service.APIKey{ID: 2, UserID: 3, GroupID: &entryID, Group: &service.Group{ID: entryID, IsCategoryDefault: true, Platform: service.PlatformComposite}}
	recorder, c := systemCustomModelsRequestContext(t, key, "/v1/models?client_version=0.144.0")
	h.CategoryDefaultCodexModels(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, []string{"friendly-model", "hidden-pending"}, gjsonStrings(recorder.Body.String(), "models.#.slug"))
	require.True(t, gjson.Get(recorder.Body.String(), "models.0.supported_in_api").Bool())
	require.Equal(t, []string{"text"}, gjsonStrings(recorder.Body.String(), "models.0.input_modalities"))
	require.NotContains(t, recorder.Body.String(), "gemini-2.5-flash", "only the public model name reaches the client manifest")
}

func TestCategoryDefaultBillingInfoRequiresModel(t *testing.T) {
	entryID := int64(91)
	key := &service.APIKey{GroupID: &entryID, Group: &service.Group{ID: entryID, Platform: service.PlatformComposite, IsCategoryDefault: true, RateMultiplier: 1}}
	c, recorder := newKeyBillingContext(key)
	newKeyBillingHandler(nil).KeyBillingInfo(c)
	require.Equal(t, http.StatusBadRequest, recorder.Code)
	require.Contains(t, recorder.Body.String(), "按来源分组计费")
	require.NotContains(t, recorder.Body.String(), "group_rate_multiplier")
}
