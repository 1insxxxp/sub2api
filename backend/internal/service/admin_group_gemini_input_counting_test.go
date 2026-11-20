//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestGroupInputProtectionDefaultsOnForEveryPlatform(t *testing.T) {
	for _, platform := range append(domain.ConcretePlatformIDs(), PlatformComposite) {
		t.Run(platform, func(t *testing.T) {
			for _, setting := range []*bool{nil, testPtrBool(false), testPtrBool(true)} {
				repo := &groupRepoStubForAdmin{createID: 73}
				svc := &adminServiceImpl{groupRepo: repo}
				group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
					Name: "protected", Platform: platform, RateMultiplier: 1, GeminiInputCountingEnabled: setting,
				})
				require.NoError(t, err)
				require.Equal(t, setting == nil || *setting, group.GeminiInputCountingEnabled)
				require.Equal(t, group.GeminiInputCountingEnabled, repo.created.GeminiInputCountingEnabled)
			}
		})
	}
}

func TestGroupInputProtectionSurvivesPlatformChanges(t *testing.T) {
	for _, saved := range []bool{false, true} {
		repo := &groupRepoStubForAdmin{getByID: &Group{
			ID: 76, Platform: PlatformGemini, RateMultiplier: 1, Status: StatusActive,
			SubscriptionType: SubscriptionTypeStandard, GeminiInputCountingEnabled: saved,
		}}
		svc := &adminServiceImpl{groupRepo: repo}
		_, err := svc.UpdateGroup(context.Background(), 76, &UpdateGroupInput{Platform: PlatformOpenAI})
		require.NoError(t, err)
		require.Equal(t, saved, repo.updated.GeminiInputCountingEnabled)
		_, err = svc.UpdateGroup(context.Background(), 76, &UpdateGroupInput{GeminiInputCountingEnabled: testPtrBool(!saved)})
		require.NoError(t, err)
		require.Equal(t, !saved, repo.updated.GeminiInputCountingEnabled)
	}
}

func TestSimpleModeGroupInputProtectionDefaultsAndPreservation(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 75}
	svc := &adminServiceImpl{groupRepo: repo, cfg: &config.Config{RunMode: config.RunModeSimple}}
	_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "new"})
	require.NoError(t, err)
	require.True(t, repo.created.GeminiInputCountingEnabled)
	repo.getByID = repo.created
	repo.getByID.GeminiInputCountingEnabled = false
	_, err = svc.UpdateGroup(context.Background(), 75, &UpdateGroupInput{Name: "renamed"})
	require.NoError(t, err)
	require.False(t, repo.updated.GeminiInputCountingEnabled)
}

func TestGroupServiceInputProtectionDefaultsAndOverride(t *testing.T) {
	repo := &keyDisplayCategoryGroupRepoStub{groupRepoStubForAdmin: &groupRepoStubForAdmin{createID: 84}}
	svc := NewGroupService(repo, nil)
	for _, setting := range []*bool{nil, testPtrBool(false)} {
		created, err := svc.Create(context.Background(), CreateGroupRequest{Name: "direct", RateMultiplier: 1, GeminiInputCountingEnabled: setting})
		require.NoError(t, err)
		require.Equal(t, setting == nil, created.GeminiInputCountingEnabled)
		repo.getByID = created
		updated, err := svc.Update(context.Background(), created.ID, UpdateGroupRequest{})
		require.NoError(t, err)
		require.Equal(t, setting == nil, updated.GeminiInputCountingEnabled)
		updated, err = svc.Update(context.Background(), created.ID, UpdateGroupRequest{GeminiInputCountingEnabled: testPtrBool(false)})
		require.NoError(t, err)
		require.False(t, updated.GeminiInputCountingEnabled)
	}
}

func TestDuplicateGroupPreservesGeminiInputCountingPolicy(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		clone := cloneGroupForDuplicate(&Group{Name: "input-discount", Platform: PlatformGemini, GeminiInputCountingEnabled: enabled}, "test-discount-copy")
		require.Equal(t, enabled, clone.GeminiInputCountingEnabled)
	}
}
