//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
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
		{ID: 1, Name: "public", SortOrder: 17, Status: StatusActive, RateMultiplier: 1.5, UpdatedAt: time.Unix(10, 0)},
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

	snapshot, err := NewPublicGroupSyncService(groups, channels, nil, nil).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.Equal(t, int64(1), snapshot[0].GroupID)
	payload, err := json.Marshal(snapshot[0])
	require.NoError(t, err)
	var fields map[string]any
	require.NoError(t, json.Unmarshal(payload, &fields))
	require.Equal(t, float64(17), fields["sort_order"])
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
	err                error
}

func (s *stubAccountRepositoryForPublicGroupSync) ListByGroup(context.Context, int64) ([]Account, error) {
	return s.accounts, nil
}

func (s *stubAccountRepositoryForPublicGroupSync) ListSchedulableByGroupID(_ context.Context, groupID int64) ([]Account, error) {
	return s.schedulableByGroup[groupID], s.err
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

	snapshot, err := NewPublicGroupSyncService(groups, channels, accounts, nil).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.ElementsMatch(t, []string{"ccmax-model-a", "ccmax-model-b"}, snapshot[0].Models)
	require.NotContains(t, snapshot[0].Models, "unavailable-shared-model")
}

func TestPublicGroupSyncSnapshotEmptyGroupDoesNotInheritSharedChannelModels(t *testing.T) {
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{{ID: 102, Name: "aws", Platform: PlatformAnthropic, Status: StatusActive}}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
		return []Channel{{Status: StatusActive, GroupIDs: []int64{102},
			ModelPricing: []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"aws-model", "other-group-model"}, BillingMode: BillingModeToken}},
			ModelMapping: map[string]map[string]string{PlatformAnthropic: {"other-mapped-model": "upstream"}},
		}}, nil
	}}
	accounts := &stubAccountRepositoryForPublicGroupSync{schedulableByGroup: map[int64][]Account{}}
	svc := NewPublicGroupSyncService(groups, channels, accounts, nil)
	snapshot, err := svc.Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.Empty(t, snapshot[0].Models)
	require.Empty(t, snapshot[0].ModelMapping)
	require.Empty(t, snapshot[0].ModelPricing)

	accounts.schedulableByGroup[102] = []Account{{Platform: PlatformAnthropic, Credentials: map[string]any{
		"model_mapping": map[string]any{"aws-model": "aws-model"},
	}}}
	snapshot, err = svc.Snapshot(context.Background())
	require.NoError(t, err)
	require.Equal(t, []string{"aws-model"}, snapshot[0].Models)
}

func TestPublicGroupSyncSnapshotIncludesAccountModelsWithoutChannel(t *testing.T) {
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{{ID: 44, Name: "no-cache", Platform: PlatformAnthropic, Status: StatusActive}}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	accounts := &stubAccountRepositoryForPublicGroupSync{schedulableByGroup: map[int64][]Account{
		44: {{Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{
			"claude-opus-4-6": "claude-opus-4-6", "claude-sonnet-4-6": "claude-sonnet-4-6", "claude-*": "claude-*",
		}}}},
	}}
	pricing := newStubPricingServiceFromJSON(t, `{
		"claude-opus-4-6": {"input_cost_per_token": 0.000005, "output_cost_per_token": 0.000025,
			"cache_read_input_token_cost": 0.0000005, "cache_creation_input_token_cost": 0.00000625}
	}`)
	snapshot, err := NewPublicGroupSyncService(groups, channels, accounts, pricing).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.Equal(t, []string{"claude-opus-4-6", "claude-sonnet-4-6"}, snapshot[0].Models)
	require.Equal(t, string(BillingModeToken), snapshot[0].ModelPricing["claude-opus-4-6"].BillingMode)
	model := snapshot[0].ModelPricing["claude-opus-4-6"]
	require.NotNil(t, model.InputPrice)
	require.Equal(t, 0.000005, *model.InputPrice)
	require.Equal(t, 0.000025, *model.OutputPrice)
	require.Equal(t, 0.0000005, *model.CacheReadPrice)
	require.Equal(t, 0.00000625, *model.CacheWritePrice)
}

func TestPublicGroupSyncSnapshotAccountCatalogUsesGatewayRules(t *testing.T) {
	for _, tc := range []struct {
		name     string
		accounts []Account
		selected GroupModelsListConfig
		want     []string
	}{
		{name: "other platform cannot activate a group", accounts: []Account{{Platform: PlatformOpenAI}}},
		{name: "unmapped account uses platform defaults", accounts: []Account{{Platform: PlatformAnthropic}}, want: DefaultModelIDsForPlatform(PlatformAnthropic)},
		{name: "custom list filters account models", accounts: []Account{{Platform: PlatformAnthropic, Credentials: map[string]any{"model_mapping": map[string]any{
			"claude-opus-4-6": "claude-opus-4-6", "claude-sonnet-4-6": "claude-sonnet-4-6",
		}}}}, selected: GroupModelsListConfig{Enabled: true, Models: []string{"claude-opus-4-6"}}, want: []string{"claude-opus-4-6"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groups := &stubGroupRepoForAvailable{activeGroups: []Group{{ID: 44, Name: "public", Platform: PlatformAnthropic, Status: StatusActive, ModelsListConfig: tc.selected}}}
			channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
			accounts := &stubAccountRepositoryForPublicGroupSync{schedulableByGroup: map[int64][]Account{44: tc.accounts}}
			snapshot, err := NewPublicGroupSyncService(groups, channels, accounts, nil).Snapshot(context.Background())
			require.NoError(t, err)
			require.ElementsMatch(t, tc.want, snapshot[0].Models)
		})
	}
}

func TestPublicGroupSyncSnapshotAccountLookupFailureDoesNotPublishEmptyCatalog(t *testing.T) {
	groups := &stubGroupRepoForAvailable{activeGroups: []Group{{ID: 44, Name: "public", Platform: PlatformAnthropic}}}
	channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) { return nil, nil }}
	wantErr := errors.New("database unavailable")
	accounts := &stubAccountRepositoryForPublicGroupSync{err: wantErr}
	snapshot, err := NewPublicGroupSyncService(groups, channels, accounts, nil).Snapshot(context.Background())
	require.ErrorIs(t, err, wantErr)
	require.Nil(t, snapshot)
}

func TestPublicGroupSyncSnapshotCatalogHonorsRoutingRules(t *testing.T) {
	price := 0.25
	for _, tc := range []struct {
		name      string
		platform  string
		mapping   map[string]any
		allowlist GroupModelAllowlist
		channel   Channel
		want      []string
		priced    bool
	}{
		{name: "wildcard group allowlist", platform: PlatformAnthropic,
			mapping:   map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6", "custom-other": "custom-other"},
			allowlist: GroupModelAllowlist{Enabled: true, Models: []string{"claude-*"}}, want: []string{"claude-sonnet-4-6"}},
		{name: "wildcard account mapping preserves channel catalog", platform: PlatformAnthropic,
			mapping: map[string]any{"claude-*": "claude-sonnet-4-6"},
			channel: Channel{ModelPricing: []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"claude-sonnet-4-6"}}}},
			want:    []string{"claude-sonnet-4-6"}},
		{name: "wildcard channel price and billing mode", platform: PlatformAnthropic,
			mapping: map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6"},
			channel: Channel{ModelPricing: []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"claude-*"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price}}},
			want:    []string{"claude-sonnet-4-6"}, priced: true},
		{name: "restricted channel excludes unpriced account models", platform: PlatformAnthropic,
			mapping: map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6", "claude-opus-4-6": "claude-opus-4-6"},
			channel: Channel{RestrictModels: true, BillingModelSource: BillingModelSourceRequested,
				ModelPricing: []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"claude-sonnet-4-6"}}}},
			want: []string{"claude-sonnet-4-6"}},
		{name: "fixed price channel does not gain token fallback models", platform: PlatformAnthropic,
			mapping: map[string]any{"claude-sonnet-4-6": "claude-sonnet-4-6", "claude-opus-4-6": "claude-opus-4-6"},
			channel: Channel{ModelPricing: []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"claude-sonnet-4-6"}, BillingMode: BillingModePerRequest, PerRequestPrice: &price}}},
			want:    []string{"claude-sonnet-4-6"}, priced: true},
		{name: "deepseek empty mapping preserves own platform", platform: PlatformDeepseek,
			channel: Channel{ModelPricing: []ChannelModelPricing{{Platform: PlatformDeepseek, Models: []string{"deepseek-flash"}}}},
			want:    []string{"deepseek-flash"}},
		{name: "openai channel alias uses forwarded model", platform: PlatformOpenAI,
			mapping: map[string]any{"gpt-5.4": "gpt-5.4"},
			channel: Channel{ModelMapping: map[string]map[string]string{PlatformOpenAI: {"public-gpt": "gpt-5.4"}}},
			want:    []string{"public-gpt"}},
		{name: "oauth allowlist alias uses gateway normalization", platform: PlatformAnthropic,
			mapping:   map[string]any{"claude-sonnet-4-5-20250929": "claude-sonnet-4-5-20250929"},
			allowlist: GroupModelAllowlist{Enabled: true, Models: []string{"claude-sonnet-4-5"}},
			want:      []string{"claude-sonnet-4-5"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groups := &stubGroupRepoForAvailable{activeGroups: []Group{{ID: 44, Platform: tc.platform, ModelAllowlist: tc.allowlist}}}
			tc.channel.GroupIDs, tc.channel.Status = []int64{44}, StatusActive
			channels := &mockChannelRepository{listAllFn: func(context.Context) ([]Channel, error) {
				if len(tc.channel.ModelPricing) == 0 && len(tc.channel.ModelMapping) == 0 && !tc.channel.RestrictModels {
					return nil, nil
				}
				return []Channel{tc.channel}, nil
			}}
			accounts := &stubAccountRepositoryForPublicGroupSync{schedulableByGroup: map[int64][]Account{44: {{Platform: tc.platform, Credentials: map[string]any{"model_mapping": tc.mapping}}}}}
			snapshot, err := NewPublicGroupSyncService(groups, channels, accounts, nil).Snapshot(context.Background())
			require.NoError(t, err)
			require.ElementsMatch(t, tc.want, snapshot[0].Models)
			if tc.priced {
				model := snapshot[0].ModelPricing[tc.want[0]]
				require.Equal(t, string(BillingModePerRequest), model.BillingMode)
				require.Equal(t, &price, model.PerRequestPrice)
			}
		})
	}
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

	snapshot, err := NewPublicGroupSyncService(groups, channels, accounts, nil).Snapshot(context.Background())
	require.NoError(t, err)
	require.Len(t, snapshot, 1)
	require.Contains(t, snapshot[0].Models, "gpt-image-2")
	require.NotContains(t, snapshot[0].Models, "claude-sonnet-4-6")
	require.Equal(t, string(BillingModeImage), snapshot[0].ModelPricing["gpt-image-2"].BillingMode)
	require.Equal(t, imagePrice, *snapshot[0].ModelPricing["gpt-image-2"].PerRequestPrice)

	accounts.schedulableByGroup = map[int64][]Account{7: accounts.accounts}
	snapshot, err = NewPublicGroupSyncService(groups, channels, accounts, nil).Snapshot(context.Background())
	require.NoError(t, err)
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

	snapshot, err := NewPublicGroupSyncService(groups, channels, nil, nil).Snapshot(context.Background())
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
