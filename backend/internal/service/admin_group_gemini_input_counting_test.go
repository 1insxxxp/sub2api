//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateGroupPreservesGeminiInputCountingPolicy(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 73}
	svc := &adminServiceImpl{groupRepo: repo}
	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "input-discount", Platform: PlatformGemini, RateMultiplier: 1, GeminiInputCountingEnabled: true,
	})
	require.NoError(t, err)
	require.True(t, group.GeminiInputCountingEnabled)
	require.True(t, repo.created.GeminiInputCountingEnabled)
}

func TestCreateGroupRejectsUnsupportedGeminiInputCountingPlatform(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformAntigravity, PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			repo := &groupRepoStubForAdmin{createID: 74}
			svc := &adminServiceImpl{groupRepo: repo}
			_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
				Name: "invalid-input-discount", Platform: platform, RateMultiplier: 1, GeminiInputCountingEnabled: true,
			})
			require.Error(t, err)
			require.Nil(t, repo.created)
		})
	}
}

func TestCreateGroupGeminiInputCountingDefaultAndComposite(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		repo := &groupRepoStubForAdmin{createID: 75}
		svc := &adminServiceImpl{groupRepo: repo}
		group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
			Name: "composite-input-discount", Platform: PlatformComposite, RateMultiplier: 1, GeminiInputCountingEnabled: enabled,
		})
		require.NoError(t, err)
		require.Equal(t, enabled, group.GeminiInputCountingEnabled)
	}
}

func TestUpdateGroupGeminiInputCountingOmissionAndExplicitDisable(t *testing.T) {
	repo := &groupRepoStubForAdmin{getByID: &Group{
		ID: 76, Name: "input-discount", Platform: PlatformGemini, RateMultiplier: 1,
		Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, GeminiInputCountingEnabled: true,
	}}
	svc := &adminServiceImpl{groupRepo: repo}
	group, err := svc.UpdateGroup(context.Background(), 76, &UpdateGroupInput{Name: "input-discount-renamed"})
	require.NoError(t, err)
	require.True(t, group.GeminiInputCountingEnabled, "an unrelated edit must not disable the policy")
	disabled := false
	group, err = svc.UpdateGroup(context.Background(), 76, &UpdateGroupInput{GeminiInputCountingEnabled: &disabled})
	require.NoError(t, err)
	require.False(t, group.GeminiInputCountingEnabled)
	require.False(t, repo.updated.GeminiInputCountingEnabled)
}

func TestUpdateGroupRejectsUnsupportedGeminiInputCountingPlatform(t *testing.T) {
	enabled := true
	repo := &groupRepoStubForAdmin{getByID: &Group{
		ID: 77, Name: "no-input-discount", Platform: PlatformAnthropic, RateMultiplier: 1,
		Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
	}}
	svc := &adminServiceImpl{groupRepo: repo}
	_, err := svc.UpdateGroup(context.Background(), 77, &UpdateGroupInput{GeminiInputCountingEnabled: &enabled})
	require.Error(t, err)
	require.Nil(t, repo.updated)
}

func TestUpdateGroupPlatformChangeRequiresDisablingGeminiInputCounting(t *testing.T) {
	repo := &groupRepoStubForAdmin{getByID: &Group{
		ID: 78, Name: "input-discount", Platform: PlatformGemini, RateMultiplier: 1,
		Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, GeminiInputCountingEnabled: true,
	}}
	svc := &adminServiceImpl{groupRepo: repo}
	_, err := svc.UpdateGroup(context.Background(), 78, &UpdateGroupInput{Platform: PlatformOpenAI})
	require.Error(t, err)
	require.Nil(t, repo.updated)
	disabled := false
	group, err := svc.UpdateGroup(context.Background(), 78, &UpdateGroupInput{Platform: PlatformOpenAI, GeminiInputCountingEnabled: &disabled})
	require.NoError(t, err)
	require.False(t, group.GeminiInputCountingEnabled)
}

func TestDuplicateGroupPreservesGeminiInputCountingPolicy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		clone := cloneGroupForDuplicate(&Group{Name: "input-discount", Platform: PlatformGemini, GeminiInputCountingEnabled: enabled}, "test-discount-copy")
		require.Equal(t, enabled, clone.GeminiInputCountingEnabled)
	}
}
