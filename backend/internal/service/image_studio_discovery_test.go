//go:build unit

package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type imageStudioDiscoveryAccounts struct {
	accounts  []Account
	err       error
	groupID   int64
	platforms []string
}

func TestImageStudioAutomaticModelsUseChannelMappingsWithoutHidingAccountDefaults(t *testing.T) {
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{9}, RestrictModels: true,
		ModelMapping: map[string]map[string]string{PlatformOpenAI: {"gpt-image-alias": "gpt-image-2", "gpt-image-invalid": "gpt-5"}},
		ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"gpt-image-alias", "gpt-image-invalid"}}},
	}
	channels := newTestChannelService(makeStandardRepo(channel, map[int64]string{9: PlatformOpenAI}))
	svc := NewImageStudioService(nil, nil)
	svc.SetModelDiscovery(&imageStudioDiscoveryAccounts{accounts: []Account{{Platform: PlatformOpenAI}}}, channels)
	models, err := svc.discoverImageModels(context.Background(), &Group{ID: 9, Platform: PlatformOpenAI}, nil)
	require.NoError(t, err)
	want := dedupeImageStudioModelsForGroup(&Group{Platform: PlatformOpenAI}, append(DefaultModelIDsForPlatform(PlatformOpenAI), "gpt-image-alias"))
	sort.Strings(want)
	require.Equal(t, want, models)
}

type imageStudioDiscoveryKeyResolver struct {
	imageStudioGroupResolverStub
	key *APIKey
}

func (r *imageStudioDiscoveryKeyResolver) GetImageStudioAPIKeyByID(_ context.Context, userID, keyID int64) (*APIKey, error) {
	if r.key == nil || r.key.ID != keyID || r.key.UserID != userID || r.key.Status != StatusAPIKeyActive {
		return nil, ErrAPIKeyNotFound
	}
	return r.key, nil
}

func TestImageStudioAutomaticModelsValidateSelectedKeyBeforeEnqueue(t *testing.T) {
	groupID, otherGroupID, keyID := int64(9), int64(10), int64(15)
	resolver := &imageStudioDiscoveryKeyResolver{
		imageStudioGroupResolverStub: imageStudioGroupResolverStub{groups: []Group{{ID: groupID, Status: StatusActive, Platform: PlatformOpenAI, AllowImageGeneration: true}}},
		key:                          &APIKey{ID: keyID, UserID: 7, GroupID: &groupID, Status: StatusAPIKeyActive},
	}
	svc := NewImageStudioService(nil, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true}})
	svc.SetGroupResolver(resolver)
	repo := &imageStudioDiscoveryAccounts{accounts: []Account{{Platform: PlatformOpenAI}}}
	svc.SetModelDiscovery(repo, nil)
	input := ImageStudioGenerateInput{UserID: 7, APIKeyID: &keyID, Model: "gpt-image-2", Prompt: "test"}
	_, prepared, err := svc.prepareGenerateInput(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, groupID, *prepared.GroupID)
	input.GroupID = &otherGroupID
	_, _, err = svc.prepareGenerateInput(context.Background(), input)
	require.ErrorIs(t, err, ErrGroupNotAllowed)
	input.GroupID = nil
	input.UserID = 8
	_, _, err = svc.prepareGenerateInput(context.Background(), input)
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	input.UserID = 7
	expired := time.Now().Add(-time.Hour)
	resolver.key.ExpiresAt = &expired
	_, _, err = svc.prepareGenerateInput(context.Background(), input)
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
	resolver.key.ExpiresAt = nil
	repo.accounts = nil
	_, _, err = svc.prepareGenerateInput(context.Background(), input)
	require.ErrorContains(t, err, "image model is not allowed")
	_, _, err = svc.prepareEditInput(context.Background(), ImageStudioEditInput{UserID: 7, APIKeyID: &keyID, Model: "gpt-image-2", Prompt: "test"})
	require.ErrorContains(t, err, "image model is not allowed")
}

func (r *imageStudioDiscoveryAccounts) ListModelAvailabilityCandidates(_ context.Context, groupID *int64, platforms []string, includeGrouped bool) ([]Account, error) {
	r.groupID = *groupID
	r.platforms = platforms
	return r.accounts, r.err
}

func TestImageStudioAutomaticModels(t *testing.T) {
	for _, tc := range []struct {
		name      string
		platform  string
		accounts  []Account
		allowlist GroupModelAllowlist
		want      []string
	}{
		{name: "discovers account models absent from studio settings", platform: PlatformOpenAI, accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-image-new": "gpt-image-1", "gpt-5": "gpt-5"}}}}, want: []string{"gpt-image-new"}},
		{name: "no accounts has no default model", platform: PlatformOpenAI, want: []string{}},
		{name: "text-only accounts have no default image model", platform: PlatformOpenAI, accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-5": "gpt-5"}}}}, want: []string{}},
		{name: "group allowlist does not hide account models", platform: PlatformOpenAI, accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-image-1": "gpt-image-1", "gpt-image-2": "gpt-image-2"}}}}, allowlist: GroupModelAllowlist{Enabled: true, Models: []string{"gpt-image-1"}}, want: []string{"gpt-image-1", "gpt-image-2"}},
		{name: "empty allowlist does not hide platform defaults", platform: PlatformOpenAI, accounts: []Account{{Platform: PlatformOpenAI}}, allowlist: GroupModelAllowlist{Enabled: true}, want: dedupeImageStudioModelsForGroup(&Group{Platform: PlatformOpenAI}, DefaultModelIDsForPlatform(PlatformOpenAI))},
		{name: "gemini mappings filter chat models", platform: PlatformGemini, accounts: []Account{{Platform: PlatformGemini, Credentials: map[string]any{"model_mapping": map[string]any{"gemini-3.1-flash-image": "gemini-3.1-flash-image", "gemini-2.5-pro": "gemini-2.5-pro"}}}}, want: []string{"gemini-3.1-flash-image"}},
		{name: "grok mapping cannot leak gpt fallback", platform: PlatformGrok, accounts: []Account{{Platform: PlatformGrok, Credentials: map[string]any{"model_mapping": map[string]any{"grok-imagine-image": "grok-imagine-image", "grok-4.6": "grok-4.6"}}}}, want: []string{"grok-imagine-image"}},
		{name: "image name mapped to chat is excluded", platform: PlatformOpenAI, accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{"gpt-image-2": "gpt-5"}}}}, want: []string{}},
		{name: "other platform accounts cannot contribute", platform: PlatformOpenAI, accounts: []Account{{Platform: PlatformGemini}}, want: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &imageStudioDiscoveryAccounts{accounts: tc.accounts}
			svc := NewImageStudioService(nil, nil)
			svc.SetModelDiscovery(repo, nil)
			group := &Group{ID: 9, Platform: tc.platform, ModelAllowlist: tc.allowlist}
			models, err := svc.discoverImageModels(context.Background(), group, &ImageStudioSettings{AllowedModels: []string{"gpt-image-2"}})
			require.NoError(t, err)
			require.Equal(t, tc.want, models)
			require.Equal(t, int64(9), repo.groupID)
			require.Equal(t, []string{tc.platform}, repo.platforms)
		})
	}
}

func TestImageStudioAutomaticModelsIgnoreConfiguredImageModelLists(t *testing.T) {
	repo := &imageStudioDiscoveryAccounts{accounts: []Account{{
		Platform: PlatformOpenAI,
		Credentials: map[string]any{"model_mapping": map[string]any{
			"gpt-image-live":  "gpt-image-live",
			"gpt-image-other": "gpt-image-other",
		}},
	}}}
	svc := NewImageStudioService(nil, nil)
	svc.SetModelDiscovery(repo, nil)

	models, err := svc.discoverImageModels(context.Background(), &Group{
		ID:       9,
		Platform: PlatformOpenAI,
		ModelAllowlist: GroupModelAllowlist{
			Enabled: true,
			Models:  []string{"gpt-image-live"},
		},
	}, &ImageStudioSettings{AllowedModels: []string{"gpt-image-configured"}})

	require.NoError(t, err)
	require.Equal(t, []string{"gpt-image-live", "gpt-image-other"}, models)
}

func TestImageStudioAutomaticModelsShareSubmissionValidation(t *testing.T) {
	repo := &imageStudioDiscoveryAccounts{accounts: []Account{{Platform: PlatformOpenAI, Credentials: map[string]any{"model_mapping": map[string]any{
		"gpt-image-live":   "gpt-image-live",
		"gpt-image-custom": "gpt-image-custom",
	}}}}}
	svc := NewImageStudioService(nil, &imageStudioConfigReaderStub{cfg: &ImageStudioSettings{Enabled: true, AllowedModels: []string{"gpt-image-configured"}, DefaultModel: "gpt-image-configured"}})
	svc.SetModelDiscovery(repo, nil)
	svc.SetGroupResolver(&imageStudioGroupResolverStub{groups: []Group{{ID: 9, Status: StatusActive, Platform: PlatformOpenAI, AllowImageGeneration: true}}})
	options, err := svc.GetOptions(context.Background(), 7)
	require.NoError(t, err)
	require.Len(t, options.Groups, 1)
	for _, model := range options.Groups[0].Models {
		require.NotContains(t, model.Model, "*")
		_, _, err = svc.prepareGenerateInput(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: model.Model, Prompt: "test"})
		require.NoError(t, err, model.Model)
	}
	require.Contains(t, options.Groups[0].Models, ImageStudioModelOption{Model: "gpt-image-custom", Label: "gpt-image-custom", Capabilities: []string{"generation", "edit"}})
	require.NotContains(t, options.Groups[0].Models, ImageStudioModelOption{Model: "gpt-image-configured", Label: "gpt-image-configured", Capabilities: []string{"generation", "edit"}})
	_, _, err = svc.prepareGenerateInput(context.Background(), ImageStudioGenerateInput{UserID: 7, Model: "grok-imagine-image", Prompt: "test"})
	require.Error(t, err)
	repo.err = errors.New("catalog unavailable")
	_, err = svc.GetOptions(context.Background(), 7)
	require.ErrorContains(t, err, "catalog unavailable")
}
