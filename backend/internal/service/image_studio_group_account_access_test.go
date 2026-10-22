//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type imageStudioFreshAccessRepo struct {
	imageStudioAPIKeyRepoStub
	fresh *APIKey
	err   error
}

func (r *imageStudioFreshAccessRepo) GetByID(context.Context, int64) (*APIKey, error) {
	return r.fresh, r.err
}

func TestGroupAccountAccessImageStudioLoadsFreshOwnedKey(t *testing.T) {
	groupID := int64(3)
	for _, mode := range []string{"default", "group"} {
		t.Run(mode, func(t *testing.T) {
			repo := &imageStudioFreshAccessRepo{
				imageStudioAPIKeyRepoStub: imageStudioAPIKeyRepoStub{keys: []APIKey{{
					ID: 22, UserID: 7, Status: StatusAPIKeyActive, GroupID: &groupID,
				}}},
				fresh: &APIKey{ID: 22, UserID: 7, Status: StatusAPIKeyActive, GroupID: &groupID,
					User: &User{ID: 7, GroupAccountExclusions: GroupAccountExclusions{groupID: {11}}}},
			}
			svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)
			load := func() (*APIKey, error) {
				if mode == "group" {
					return svc.GetImageStudioAPIKeyForGroup(context.Background(), 7, groupID)
				}
				return svc.GetDefaultImageStudioAPIKey(context.Background(), 7)
			}
			key, err := load()
			require.NoError(t, err)
			require.Equal(t, GroupAccountExclusions{groupID: {11}}, key.User.GroupAccountExclusions)
			repo.err = errors.New("authorization snapshot unavailable")
			key, err = load()
			require.Error(t, err, "list projections must not become an authorization fallback")
			require.Nil(t, key)
			repo.err = nil
			repo.fresh.UserID = 8
			key, err = load()
			require.ErrorIs(t, err, ErrAPIKeyNotFound)
			require.Nil(t, key)
			repo.fresh.UserID = 7
			repo.fresh.User = nil
			key, err = load()
			require.ErrorIs(t, err, ErrAPIKeyNotFound, "missing private user snapshot must fail closed")
			require.Nil(t, key)
			if mode == "group" {
				repo.fresh.User = &User{ID: 7}
				otherGroup := int64(9)
				repo.fresh.GroupID = &otherGroup
				key, err = load()
				require.ErrorIs(t, err, ErrAPIKeyNotFound, "a changed key group must not replace the selected group")
				require.Nil(t, key)
			}
		})
	}
}

type imageStudioAccessGateway struct{ imageStudioGatewayStub }

func (s *imageStudioAccessGateway) SelectAccountWithSchedulerForImages(ctx context.Context, groupID *int64, sessionHash, model string, excluded map[int64]struct{}, capability OpenAIImagesCapability) (*AccountSelectionResult, OpenAIAccountScheduleDecision, error) {
	if !IsGroupAccountAllowedForUser(ctx, groupID, 11) {
		return nil, OpenAIAccountScheduleDecision{}, ErrNoAvailableAccounts
	}
	return s.imageStudioGatewayStub.SelectAccountWithSchedulerForImages(ctx, groupID, sessionHash, model, excluded, capability)
}

type imageStudioAccessGeminiGateway struct{ imageStudioGeminiGatewayStub }

func (s *imageStudioAccessGeminiGateway) SelectAccountForModelWithExclusions(ctx context.Context, groupID *int64, sessionHash, model string, excluded map[int64]struct{}) (*Account, error) {
	if !IsGroupAccountAllowedForUser(ctx, groupID, 44) {
		return nil, ErrNoAvailableAccounts
	}
	return s.imageStudioGeminiGatewayStub.SelectAccountForModelWithExclusions(ctx, groupID, sessionHash, model, excluded)
}

func TestGroupAccountAccessImageStudioBindsOwnedPolicyForBothProviders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(3)
	for _, platform := range []string{PlatformOpenAI, PlatformGemini} {
		for _, scopedToSelectedGroup := range []bool{true, false} {
			name := platform + "/other-group"
			ruleGroupID := int64(9)
			if scopedToSelectedGroup {
				name, ruleGroupID = platform+"/excluded", groupID
			}
			t.Run(name, func(t *testing.T) {
				key := &APIKey{ID: 22, UserID: 7, Status: StatusAPIKeyActive, GroupID: &groupID,
					User:  &User{ID: 7, Balance: 10, GroupAccountExclusions: GroupAccountExclusions{ruleGroupID: {11, 44}}},
					Group: &Group{ID: groupID, Platform: platform, AllowImageGeneration: true}}
				openAI := &imageStudioAccessGateway{}
				gemini := &imageStudioAccessGeminiGateway{}
				executor := NewImageStudioGatewayExecutor(&imageStudioAPIKeyProviderStub{apiKey: key}, nil, nil, openAI, gemini)
				model := "gpt-image-1"
				if platform == PlatformGemini {
					model = "gemini-3.1-flash-image-preview"
				}
				// JWT/worker contexts may have no API-key authentication policy or
				// stale metadata; only the owned key's freshly loaded policy applies.
				ctx := WithGroupAccountExclusions(context.Background(), 999, nil)
				result, err := executor.Generate(ctx, ImageStudioGenerateInput{UserID: 7, Model: model,
					Prompt: "draw it", AspectRatio: "16:9", Size: "1536x864", BillingTier: ImageBillingSize2K})
				if scopedToSelectedGroup {
					require.ErrorIs(t, err, ErrNoAvailableAccounts)
					require.Nil(t, result)
					require.Empty(t, openAI.forwardBody)
					require.Empty(t, gemini.forwardBody)
				} else {
					require.NoError(t, err)
					require.NotEmpty(t, result.ImageBytes)
				}
			})
		}
	}
}
