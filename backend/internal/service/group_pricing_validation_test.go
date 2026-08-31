//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func newGroupPricingValidationService(repo GroupRepository) *adminServiceImpl {
	billing := NewBillingService(&config.Config{}, &PricingService{pricingData: map[string]*LiteLLMModelPricing{}})
	coverage := NewGroupPricingCoverageService(NewModelPricingResolver(nil, billing))
	return &adminServiceImpl{groupRepo: repo, groupPricingCoverage: coverage}
}

func TestAdminServiceCreateGroupRejectsNewlyPublishedMissingPricing(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 51}
	svc := newGroupPricingValidationService(repo)

	_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name:             "source",
		Platform:         PlatformGemini,
		RateMultiplier:   1,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"new-unique-model"}},
	})

	require.Error(t, err)
	require.Equal(t, "GROUP_MODEL_PRICING_REQUIRED", infraerrors.Reason(err))
	require.Nil(t, repo.created)
}

func TestAdminServiceCreateGroupAcceptsProspectivePricingForNewModel(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 51}
	svc := newGroupPricingValidationService(repo)
	pricing := []ChannelModelPricing{{
		Platform:    PlatformGemini,
		Models:      []string{"new-unique-model"},
		BillingMode: BillingModeToken,
		InputPrice:  coveragePrice(1e-6),
	}}

	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name:             "source",
		Platform:         PlatformGemini,
		RateMultiplier:   1,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"new-unique-model"}},
		ModelPricing:     pricing,
	})

	require.NoError(t, err)
	require.NotNil(t, group)
	require.NotNil(t, repo.created)
}

func TestAdminServiceUpdateGroupAllowsHistoricalMissingPricingOnUnrelatedEdit(t *testing.T) {
	description := "edited"
	existing := &Group{
		ID:               9,
		Name:             "source",
		Platform:         PlatformGemini,
		RateMultiplier:   1,
		Status:           StatusActive,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"legacy-unpriced-model"}},
	}
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := newGroupPricingValidationService(repo)

	group, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{Description: &description})

	require.NoError(t, err)
	require.Equal(t, description, group.Description)
	require.NotNil(t, repo.updated)
}

func TestAdminServiceUpdateGroupAllowsHistoricalInvalidPricingOnUnrelatedEdit(t *testing.T) {
	description := "edited"
	existing := &Group{
		ID: 9, Name: "source", Platform: PlatformGemini, RateMultiplier: 1, Status: StatusActive,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"legacy-invalid-model"}},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformGemini, Models: []string{"legacy-invalid-model"}, BillingMode: BillingModeToken,
		}},
	}
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := newGroupPricingValidationService(repo)

	group, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{Description: &description})

	require.NoError(t, err)
	require.Equal(t, description, group.Description)
	require.NotNil(t, repo.updated)
}

func TestAdminServiceUpdateGroupRejectsOnlyNewMissingModel(t *testing.T) {
	existing := &Group{
		ID:               9,
		Name:             "source",
		Platform:         PlatformGemini,
		RateMultiplier:   1,
		Status:           StatusActive,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"legacy-unpriced-model"}},
	}
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := newGroupPricingValidationService(repo)
	models := GroupModelsListConfig{Enabled: true, Models: []string{"legacy-unpriced-model", "new-unique-model"}}

	_, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{ModelsListConfig: &models})

	require.Error(t, err)
	require.Equal(t, "GROUP_MODEL_PRICING_REQUIRED", infraerrors.Reason(err))
	require.Nil(t, repo.updated)
}

func TestAdminServiceUpdateGroupAcceptsNewModelWithProspectivePricing(t *testing.T) {
	existing := &Group{
		ID:               9,
		Name:             "source",
		Platform:         PlatformGemini,
		RateMultiplier:   1,
		Status:           StatusActive,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"legacy-unpriced-model"}},
	}
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := newGroupPricingValidationService(repo)
	models := GroupModelsListConfig{Enabled: true, Models: []string{"legacy-unpriced-model", "new-unique-model"}}
	pricing := []ChannelModelPricing{{
		Platform:    PlatformGemini,
		Models:      []string{"new-unique-model"},
		BillingMode: BillingModeToken,
		OutputPrice: coveragePrice(2e-6),
	}}

	group, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{
		ModelsListConfig: &models,
		ModelPricing:     &pricing,
	})

	require.NoError(t, err)
	require.NotNil(t, group)
	require.NotNil(t, repo.updated)
}

func TestAdminServiceUpdateGroupRejectsDeletingOnlyPriceFromPricedModel(t *testing.T) {
	existing := pricedLegacyGroupForValidation()
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := newGroupPricingValidationService(repo)
	emptyPricing := []ChannelModelPricing{}

	_, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{ModelPricing: &emptyPricing})

	require.Error(t, err)
	require.Equal(t, "GROUP_MODEL_PRICING_REQUIRED", infraerrors.Reason(err))
	require.Nil(t, repo.updated)
}

func TestAdminServiceUpdateGroupRejectsMakingPricedModelIncomplete(t *testing.T) {
	existing := pricedLegacyGroupForValidation()
	repo := &groupRepoStubForAdmin{getByID: existing}
	svc := newGroupPricingValidationService(repo)
	incompletePricing := []ChannelModelPricing{{
		Platform: PlatformGemini, Models: []string{"legacy-explicit-model"}, BillingMode: BillingModeToken,
	}}

	_, err := svc.UpdateGroup(context.Background(), existing.ID, &UpdateGroupInput{ModelPricing: &incompletePricing})

	require.Error(t, err)
	require.Equal(t, "GROUP_MODEL_PRICING_REQUIRED", infraerrors.Reason(err))
	require.Nil(t, repo.updated)
}

func pricedLegacyGroupForValidation() *Group {
	return &Group{
		ID: 9, Name: "source", Platform: PlatformGemini, RateMultiplier: 1, Status: StatusActive,
		ModelsListConfig: GroupModelsListConfig{Enabled: true, Models: []string{"legacy-explicit-model"}},
		ModelPricing: []ChannelModelPricing{{
			Platform: PlatformGemini, Models: []string{"legacy-explicit-model"}, BillingMode: BillingModeToken,
			InputPrice: coveragePrice(1e-6),
		}},
	}
}
