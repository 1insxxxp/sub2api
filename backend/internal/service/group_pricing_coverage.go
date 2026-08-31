package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type PricingCoverageStatus string

const (
	PricingCoveragePriced  PricingCoverageStatus = "priced"
	PricingCoverageMissing PricingCoverageStatus = "missing"
	PricingCoverageInvalid PricingCoverageStatus = "invalid"
)

type GroupPricingCoverageInput struct {
	GroupID          *int64
	Platform         string
	Models           []string
	ModelPricing     *[]ChannelModelPricing
	ProspectiveGroup *Group
}

type GroupPricingCoverageModel struct {
	Model          string                `json:"model"`
	Status         PricingCoverageStatus `json:"status"`
	Source         string                `json:"source,omitempty"`
	BillingMode    BillingMode           `json:"billing_mode,omitempty"`
	Reason         string                `json:"reason,omitempty"`
	RequiredFields []string              `json:"required_fields,omitempty"`
}

type GroupPricingCoverageResult struct {
	Models []GroupPricingCoverageModel `json:"models"`
}

type GroupPricingCoverageService struct {
	resolver *ModelPricingResolver
}

func NewGroupPricingCoverageService(resolver *ModelPricingResolver) *GroupPricingCoverageService {
	return &GroupPricingCoverageService{resolver: resolver}
}

func (s *GroupPricingCoverageService) Preview(ctx context.Context, input GroupPricingCoverageInput) GroupPricingCoverageResult {
	group := prospectivePricingGroup(input)
	invalid := invalidProspectivePricingByModel(group.Platform, group.ModelPricing)
	models := normalizedUniquePricingModels(input.Models)
	result := GroupPricingCoverageResult{Models: make([]GroupPricingCoverageModel, 0, len(models))}
	for _, model := range models {
		entry := GroupPricingCoverageModel{Model: model, BillingMode: BillingModeToken}
		matchedGroupPricing := matchGroupModelPricing(group, model)
		if matchedGroupPricing != nil && matchedGroupPricing.BillingMode != "" {
			entry.BillingMode = matchedGroupPricing.BillingMode
		}
		if reason := invalid[model]; reason != "" {
			entry.Status = PricingCoverageInvalid
			entry.Reason = reason
			entry.RequiredFields = requiredPricingFields(entry.BillingMode)
			result.Models = append(result.Models, entry)
			continue
		}

		var resolved *ResolvedPricing
		if s != nil && s.resolver != nil {
			resolved = s.resolver.Resolve(ctx, PricingInput{Model: model, GroupID: input.GroupID, Group: group})
		}
		evaluation := evaluateEffectiveResolvedPricing(resolved)
		entry.BillingMode = evaluation.billingMode
		if entry.BillingMode == "" {
			entry.BillingMode = BillingModeToken
		}
		if evaluation.usable {
			entry.Status = PricingCoveragePriced
			entry.Source = evaluation.source
		} else if matchedGroupPricing != nil {
			entry.Status = PricingCoverageInvalid
			entry.Reason = "INCOMPLETE_EFFECTIVE_PRICE"
			entry.RequiredFields = requiredPricingFields(entry.BillingMode)
		} else {
			entry.Status = PricingCoverageMissing
			entry.Reason = "PRICING_NOT_FOUND"
			entry.RequiredFields = requiredPricingFields(entry.BillingMode)
		}
		result.Models = append(result.Models, entry)
	}
	return result
}

func (s *GroupPricingCoverageService) ValidateNewlyPublished(ctx context.Context, previous, prospective *Group) error {
	newModels := newlyPublishedPricingModels(previous, prospective)
	if len(newModels) == 0 {
		return nil
	}
	var groupID *int64
	if prospective != nil && prospective.ID > 0 {
		id := prospective.ID
		groupID = &id
	}
	result := s.Preview(ctx, GroupPricingCoverageInput{
		GroupID:          groupID,
		Platform:         prospective.Platform,
		Models:           newModels,
		ProspectiveGroup: prospective,
	})
	failed := make([]string, 0)
	reasons := make([]string, 0)
	for _, model := range result.Models {
		if model.Status == PricingCoveragePriced {
			continue
		}
		failed = append(failed, model.Model)
		reasons = append(reasons, model.Reason)
	}
	if len(failed) == 0 {
		return nil
	}
	return infraerrors.BadRequest(
		"GROUP_MODEL_PRICING_REQUIRED",
		"newly published models require effective pricing",
	).WithMetadata(map[string]string{
		"models":  strings.Join(failed, ","),
		"reasons": strings.Join(reasons, ","),
	})
}

func newlyPublishedPricingModels(previous, prospective *Group) []string {
	prospectiveModels := advertisedPricingModels(prospective)
	if len(prospectiveModels) == 0 {
		return nil
	}
	existing := make(map[string]struct{})
	for _, model := range advertisedPricingModels(previous) {
		existing[model] = struct{}{}
	}
	newModels := make([]string, 0, len(prospectiveModels))
	for _, model := range prospectiveModels {
		if _, ok := existing[model]; !ok {
			newModels = append(newModels, model)
		}
	}
	return newModels
}

func advertisedPricingModels(group *Group) []string {
	if group == nil || !group.ModelsListConfig.Enabled {
		return nil
	}
	return normalizedUniquePricingModels(group.ModelsListConfig.Models)
}

func prospectivePricingGroup(input GroupPricingCoverageInput) *Group {
	group := &Group{}
	if input.ProspectiveGroup != nil {
		clone := *input.ProspectiveGroup
		clone.ModelPricing = cloneChannelModelPricing(input.ProspectiveGroup.ModelPricing)
		group = &clone
	}
	if strings.TrimSpace(input.Platform) != "" {
		group.Platform = NormalizeGroupPlatform(input.Platform)
	}
	if input.ModelPricing != nil {
		group.ModelPricing = cloneChannelModelPricing(*input.ModelPricing)
	}
	return group
}

func cloneChannelModelPricing(pricing []ChannelModelPricing) []ChannelModelPricing {
	out := make([]ChannelModelPricing, len(pricing))
	for i := range pricing {
		out[i] = pricing[i].Clone()
	}
	return out
}

func normalizedUniquePricingModels(models []string) []string {
	seen := make(map[string]struct{}, len(models))
	out := make([]string, 0, len(models))
	for _, model := range models {
		model = normalizeChannelPricingModelName(model)
		if model == "" {
			continue
		}
		if _, ok := seen[model]; ok {
			continue
		}
		seen[model] = struct{}{}
		out = append(out, model)
	}
	return out
}

func invalidProspectivePricingByModel(platform string, pricing []ChannelModelPricing) map[string]string {
	invalid := make(map[string]string)
	for i := range pricing {
		entry := pricing[i].Clone()
		if _, err := normalizeGroupModelPricing(platform, []ChannelModelPricing{entry}); err != nil {
			reason := infraerrors.Reason(err)
			if reason == "" {
				reason = "INVALID_MODEL_PRICING"
			}
			for _, model := range entry.Models {
				invalid[normalizeChannelPricingModelName(model)] = reason
			}
		}
	}
	if _, err := normalizeGroupModelPricing(platform, cloneChannelModelPricing(pricing)); err != nil {
		reason := infraerrors.Reason(err)
		if reason == "" {
			reason = "INVALID_MODEL_PRICING"
		}
		for _, entry := range pricing {
			for _, model := range entry.Models {
				key := normalizeChannelPricingModelName(model)
				if invalid[key] == "" {
					invalid[key] = reason
				}
			}
		}
	}
	return invalid
}

func requiredPricingFields(mode BillingMode) []string {
	switch mode {
	case BillingModePerRequest:
		return []string{"per_request_price"}
	case BillingModeImage:
		return []string{"image_output_price"}
	default:
		return []string{"input_price", "output_price"}
	}
}
