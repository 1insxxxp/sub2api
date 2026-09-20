//go:build unit

package service

import (
	"context"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestAdminService_ModelStatusVisibilityRejectsInvalidConfiguration(t *testing.T) {
	for _, models := range [][]string{nil, {"  "}, {"gpt-*-5"}, {"gpt-**"}} {
		for _, update := range []bool{false, true} {
			repo := &groupRepoStubForAdmin{createID: 51, getByID: &Group{ID: 51, Name: "existing", Platform: PlatformOpenAI, Status: StatusActive}}
			svc := &adminServiceImpl{groupRepo: repo}
			cfg := GroupModelStatusVisibility{Enabled: true, Models: models}
			var err error
			if update {
				_, err = svc.UpdateGroup(context.Background(), 51, &UpdateGroupInput{ModelStatusVisibility: &cfg})
			} else {
				_, err = svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "visibility", Platform: PlatformOpenAI, RateMultiplier: 1, ModelStatusVisibility: cfg})
			}
			require.Error(t, err, "update=%v models=%v", update, models)
			require.Equal(t, int32(http.StatusBadRequest), infraerrors.FromError(err).Code)
			require.Equal(t, "INVALID_MODEL_STATUS_VISIBILITY", infraerrors.FromError(err).Reason)
			require.Nil(t, repo.created)
			require.Nil(t, repo.updated)
		}
	}
}

func TestAdminService_ModelStatusVisibilityIndependentOfRequestAllowlist(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 52}
	svc := &adminServiceImpl{groupRepo: repo}
	allowlist := GroupModelAllowlist{Enabled: true, Models: []string{"gpt-*"}}
	created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "visibility", Platform: PlatformOpenAI, RateMultiplier: 1,
		ModelAllowlist:        allowlist,
		ModelStatusVisibility: GroupModelStatusVisibility{Enabled: true, Models: []string{" GPT-5.4 ", "gpt-5.4", "claude-*", " "}},
	})
	require.NoError(t, err)
	require.Equal(t, []string{"GPT-5.4", "claude-*"}, repo.created.ModelStatusVisibility.Models)
	require.Equal(t, allowlist, repo.created.ModelAllowlist)
	require.True(t, created.EffectiveModelAllowlist().Allows("gpt-image-1"))
	require.False(t, created.ModelStatusModelVisible("gpt-image-1"))

	repo.getByID = created
	description := "unrelated change"
	_, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{Description: &description})
	require.NoError(t, err)
	require.True(t, repo.updated.ModelStatusVisibility.Enabled)
	require.Equal(t, []string{"GPT-5.4", "claude-*"}, repo.updated.ModelStatusVisibility.Models)

	updated := GroupModelStatusVisibility{Enabled: true, Models: []string{" gpt-5.4 ", "GPT-5.4"}}
	_, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{ModelStatusVisibility: &updated})
	require.NoError(t, err)
	require.Equal(t, []string{"gpt-5.4"}, repo.updated.ModelStatusVisibility.Models)
	require.Equal(t, allowlist, repo.updated.ModelAllowlist)

	disabled := GroupModelStatusVisibility{}
	_, err = svc.UpdateGroup(context.Background(), created.ID, &UpdateGroupInput{ModelStatusVisibility: &disabled})
	require.NoError(t, err)
	require.True(t, repo.updated.ModelStatusModelVisible("gpt-image-1"))
	require.Equal(t, allowlist, repo.updated.ModelAllowlist)
}

func TestAdminService_ModelStatusVisibilityDefaultsToAll(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 53}
	svc := &adminServiceImpl{groupRepo: repo}
	created, err := svc.CreateGroup(context.Background(), &CreateGroupInput{Name: "default", Platform: PlatformOpenAI, RateMultiplier: 1})
	require.NoError(t, err)
	require.False(t, created.ModelStatusVisibility.Enabled)
	require.True(t, created.ModelStatusModelVisible("gpt-image-1"))
}
