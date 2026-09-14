//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPublicGroupSyncSnapshotFiltersExclusiveGroupsAndPreservesMappingAndPricing(t *testing.T) {
	inputPrice := 0.003
	outputPrice := 0.015
	imagePrice := 0.04
	videoPrice := 0.08
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{
		{ID: 2, Name: "exclusive", Status: StatusActive, IsExclusive: true, RateMultiplier: 2},
		{ID: 1, Name: "public", Status: StatusActive, RateMultiplier: 1.5, UpdatedAt: time.Unix(10, 0)},
	}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
		return []Channel{
			{
				ID:       10,
				Status:   StatusActive,
				GroupIDs: []int64{1, 2},
				ModelMapping: map[string]map[string]string{
					"openai": {"gpt-public": "gpt-upstream"},
				},
				ModelPricing: []ChannelModelPricing{{
					Platform:    "openai",
					Models:      []string{"gpt-public"},
					BillingMode: BillingModeToken,
					InputPrice:  &inputPrice,
					OutputPrice: &outputPrice,
				}, {
					Platform:        "openai",
					Models:          []string{"gpt-image-public"},
					BillingMode:     BillingModeImage,
					PerRequestPrice: &imagePrice,
				}, {
					Platform:        "openai",
					Models:          []string{"gpt-video-public"},
					BillingMode:     BillingModeVideo,
					PerRequestPrice: &videoPrice,
				}},
			},
			{ID: 11, Status: StatusDisabled, GroupIDs: []int64{1}, ModelMapping: map[string]map[string]string{
				"openai": {"disabled-model": "disabled-upstream"},
			}},
		}, nil
	}}

	snapshot, err := NewPublicGroupSyncService(groups, channels).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.Equal(t, int64(1), snapshot[0].GroupID)
	require.True(t, snapshot[0].PublicEnabled)
	require.Equal(t, 1.5, snapshot[0].GroupRatio)
	require.ElementsMatch(t, []string{"gpt-public", "gpt-image-public"}, snapshot[0].Models)
	require.Equal(t, "gpt-upstream", snapshot[0].ModelMapping["gpt-public"])
	require.Equal(t, 0.003, *snapshot[0].ModelPricing["gpt-public"].InputPrice)
	require.Equal(t, 0.015, *snapshot[0].ModelPricing["gpt-public"].OutputPrice)
	require.Contains(t, snapshot[0].Models, "gpt-image-public")
	require.NotContains(t, snapshot[0].Models, "gpt-video-public")
	require.Equal(t, string(BillingModeImage), snapshot[0].ModelPricing["gpt-image-public"].BillingMode)
	require.Equal(t, 0.04, *snapshot[0].ModelPricing["gpt-image-public"].PerRequestPrice)
	require.NotContains(t, snapshot[0].ModelMapping, "disabled-model")
}

type stubAccountRepositoryForPublicGroupSync struct {
	AccountRepository
	accounts           []Account
	schedulableByGroup map[int64][]Account
}

func (s *stubAccountRepositoryForPublicGroupSync) ListByGroup(context.Context, int64) ([]Account, error) {
	return s.accounts, nil
}

func (s *stubAccountRepositoryForPublicGroupSync) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]Account, error) {
	return s.schedulableByGroup[groupID], nil
}

func TestPublicGroupSyncSnapshotUsesGroupSchedulableModels(t *testing.T) {
	inputPrice := 0.003
	outputPrice := 0.015
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{{
		ID:       84,
		Name:     "ccmax",
		Platform: PlatformAnthropic,
		Status:   StatusActive,
	}}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
		return []Channel{{
			ID:       14,
			Status:   StatusActive,
			GroupIDs: []int64{84},
			ModelPricing: []ChannelModelPricing{{
				Platform:    PlatformAnthropic,
				Models:      []string{"ccmax-model-a", "ccmax-model-b", "unavailable-shared-model"},
				BillingMode: BillingModeToken,
				InputPrice:  &inputPrice,
				OutputPrice: &outputPrice,
			}},
			ModelMapping: map[string]map[string]string{PlatformAnthropic: {
				"ccmax-model-a":            "upstream-a",
				"ccmax-model-b":            "upstream-b",
				"unavailable-shared-model": "upstream-shared",
			}},
		}}, nil
	}}
	accounts := &stubAccountRepositoryForPublicGroupSync{schedulableByGroup: map[int64][]Account{
		84: {{
			Platform: PlatformAnthropic,
			Credentials: map[string]any{"model_mapping": map[string]any{
				"ccmax-model-a": "upstream-a",
				"ccmax-model-b": "upstream-b",
			}},
		}},
	}}

	snapshot, err := NewPublicGroupSyncService(groups, channels, accounts).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.ElementsMatch(t, []string{"ccmax-model-a", "ccmax-model-b"}, snapshot[0].Models)
	require.NotContains(t, snapshot[0].Models, "unavailable-shared-model")
}

func TestPublicGroupSyncSnapshotIncludesImageModelsFromGroupAccounts(t *testing.T) {
	imagePrice := 0.375
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{{
		ID:                   7,
		Name:                 "image-group",
		Status:               StatusActive,
		AllowImageGeneration: true,
		ImagePrice1K:         &imagePrice,
	}}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
		return nil, nil
	}}
	accounts := &stubAccountRepositoryForPublicGroupSync{accounts: []Account{{
		Platform: "openai",
		Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-image-2":       "gpt-image-2",
			"claude-sonnet-4-6": "claude-sonnet-4-6",
		}},
	}}}

	snapshot, err := NewPublicGroupSyncService(groups, channels, accounts).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.Contains(t, snapshot[0].Models, "gpt-image-2")
	require.NotContains(t, snapshot[0].Models, "claude-sonnet-4-6")
	require.Equal(t, string(BillingModeImage), snapshot[0].ModelPricing["gpt-image-2"].BillingMode)
	require.Equal(t, imagePrice, *snapshot[0].ModelPricing["gpt-image-2"].PerRequestPrice)
}

func TestPublicGroupSyncSnapshotScopesModelsToGroupPlatform(t *testing.T) {
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{
		{ID: 1, Name: "claude-group", Platform: PlatformAnthropic, Status: StatusActive},
		{ID: 2, Name: "gemini-group", Platform: PlatformOpenAI, Status: StatusActive},
	}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
		return []Channel{{
			ID:       10,
			Status:   StatusActive,
			GroupIDs: []int64{1, 2},
			ModelPricing: []ChannelModelPricing{
				{Platform: PlatformAnthropic, Models: []string{"claude-model"}, BillingMode: BillingModePerRequest},
				{Platform: PlatformOpenAI, Models: []string{"gemini-model"}, BillingMode: BillingModePerRequest},
			},
			ModelMapping: map[string]map[string]string{
				PlatformAnthropic: {"claude-model": "claude-upstream"},
				PlatformOpenAI:    {"gemini-model": "gemini-upstream"},
			},
		}}, nil
	}}

	snapshot, err := NewPublicGroupSyncService(groups, channels).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 2)

	byName := make(map[string]PublicGroupSyncRequest, len(snapshot))
	for _, item := range snapshot {
		byName[item.GroupName] = item
	}
	require.Equal(t, []string{"claude-model"}, byName["claude-group"].Models)
	require.Equal(t, []string{"gemini-model"}, byName["gemini-group"].Models)
	require.NotContains(t, byName["claude-group"].ModelMapping, "gemini-model")
	require.NotContains(t, byName["gemini-group"].ModelMapping, "claude-model")
}
