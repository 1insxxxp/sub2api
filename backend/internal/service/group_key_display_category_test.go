//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestGroupKeyDisplayCategoryLifecycle(t *testing.T) {
	for _, mode := range []string{"standard", config.RunModeSimple} {
		t.Run(mode, func(t *testing.T) {
			repo := &groupRepoStubForAdmin{createID: 81}
			svc := &adminServiceImpl{groupRepo: repo, cfg: &config.Config{RunMode: mode}}
			created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "protocol group", Platform: PlatformOpenAI, RateMultiplier: 1})
			require.NoError(t, err)
			require.Empty(t, created.KeyDisplayCategory)
			repo.getByID = created
			for _, category := range []string{"domestic", "anthropic", "openai", "other", ""} {
				updated, err := svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{KeyDisplayCategory: &category})
				require.NoError(t, err)
				require.Equal(t, category, updated.KeyDisplayCategory)
				require.Equal(t, category, repo.updated.KeyDisplayCategory)
				require.Equal(t, PlatformOpenAI, updated.Platform)
				require.Equal(t, float64(1), updated.RateMultiplier)
				omitted, err := svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{})
				require.NoError(t, err)
				require.Equal(t, category, omitted.KeyDisplayCategory)
			}
		})
	}
}

func TestGroupKeyDisplayCategoryCreateAcceptsAllowedValues(t *testing.T) {
	for _, category := range []string{"", "anthropic", "openai", "domestic", "other"} {
		repo := &groupRepoStubForAdmin{}
		svc := &adminServiceImpl{groupRepo: repo}
		created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "group", Platform: PlatformOpenAI, RateMultiplier: 1, KeyDisplayCategory: category})
		require.NoError(t, err)
		require.Equal(t, category, created.KeyDisplayCategory)
		require.Equal(t, category, repo.created.KeyDisplayCategory)
	}
}

func TestGroupKeyDisplayCategoryRejectsUnknownBeforePersistence(t *testing.T) {
	for _, category := range []string{"unknown", "DOMESTIC", " domestic ", "gemini"} {
		repo := &groupRepoStubForAdmin{getByID: &Group{ID: 82, Name: "group", Platform: PlatformOpenAI, KeyDisplayCategory: "domestic"}}
		svc := &adminServiceImpl{groupRepo: repo}
		_, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "group", RateMultiplier: 1, KeyDisplayCategory: category})
		require.Error(t, err)
		require.Nil(t, repo.created)
		_, err = svc.UpdateGroup(context.Background(), 82, &UpdateGroupInput{KeyDisplayCategory: &category})
		require.Error(t, err)
		require.Nil(t, repo.updated)
		require.Equal(t, "domestic", repo.getByID.KeyDisplayCategory)
	}
}

func TestDuplicateGroupPreservesKeyDisplayCategory(t *testing.T) {
	source := &Group{ID: 83, Name: "group", Platform: PlatformOpenAI, KeyDisplayCategory: "domestic", RateMultiplier: 1.5}
	duplicate := cloneGroupForDuplicate(source, "category-copy")
	require.Equal(t, source.KeyDisplayCategory, duplicate.KeyDisplayCategory)
	require.Equal(t, source.Platform, duplicate.Platform)
	require.Equal(t, source.RateMultiplier, duplicate.RateMultiplier)
}

func TestSystemCustomGroupKeyDisplayCategoryLifecycle(t *testing.T) {
	repo := &systemCustomGroupRepositoryStub{}
	svc := NewSystemCustomGroupService(repo, systemCustomSourceGroupRepositoryStub{groups: map[int64]*Group{
		10: activeDirectSystemCustomSource(10, PlatformAnthropic),
	}}, nil)
	created, err := svc.Create(context.Background(), CreateSystemCustomGroupRequest{
		Name: "display metadata", SourceGroupIDs: []int64{10}, KeyDisplayCategory: "domestic",
	})
	require.NoError(t, err)
	require.Equal(t, "domestic", created.Group.KeyDisplayCategory)
	repo.stored = created
	for _, step := range []struct {
		category *string
		want     string
	}{{nil, "domestic"}, {ptrString("other"), "other"}, {ptrString(""), ""}} {
		updated, err := svc.Update(context.Background(), created.Group.ID, UpdateSystemCustomGroupRequest{
			Name: created.Group.Name, SourceGroupIDs: []int64{10}, KeyDisplayCategory: step.category,
		})
		require.NoError(t, err)
		require.Equal(t, step.want, updated.Group.KeyDisplayCategory)
		require.Equal(t, step.want, repo.updatedGroup.KeyDisplayCategory)
		require.Equal(t, PlatformComposite, updated.Group.Platform)
	}
	_, err = svc.Create(context.Background(), CreateSystemCustomGroupRequest{KeyDisplayCategory: "invalid"})
	require.Error(t, err)
	_, err = svc.Update(context.Background(), created.Group.ID, UpdateSystemCustomGroupRequest{KeyDisplayCategory: ptrString("invalid")})
	require.Error(t, err)
}

func TestGroupServiceKeyDisplayCategoryLifecycle(t *testing.T) {
	repo := &keyDisplayCategoryGroupRepoStub{groupRepoStubForAdmin: &groupRepoStubForAdmin{createID: 84}}
	svc := NewGroupService(repo, nil)
	created, err := svc.Create(context.Background(), CreateGroupRequest{Name: "display metadata", RateMultiplier: 1, KeyDisplayCategory: "domestic"})
	require.NoError(t, err)
	require.Equal(t, "domestic", created.KeyDisplayCategory)
	repo.getByID = created
	for _, step := range []struct {
		category *string
		want     string
	}{{nil, "domestic"}, {ptrString("other"), "other"}, {ptrString(""), ""}} {
		updated, err := svc.Update(context.Background(), created.ID, UpdateGroupRequest{KeyDisplayCategory: step.category})
		require.NoError(t, err)
		require.Equal(t, step.want, updated.KeyDisplayCategory)
		require.Equal(t, PlatformAnthropic, updated.Platform)
	}
	_, err = svc.Create(context.Background(), CreateGroupRequest{KeyDisplayCategory: "invalid"})
	require.Error(t, err)
	_, err = svc.Update(context.Background(), created.ID, UpdateGroupRequest{KeyDisplayCategory: ptrString("invalid")})
	require.Error(t, err)
}

type keyDisplayCategoryGroupRepoStub struct {
	*groupRepoStubForAdmin
}

func (*keyDisplayCategoryGroupRepoStub) ExistsByName(context.Context, string) (bool, error) {
	return false, nil
}
