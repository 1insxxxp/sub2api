//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupTagCreateUpdateAndClear(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 71}
	svc := &adminServiceImpl{groupRepo: repo}
	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "tagged", Platform: PlatformAnthropic, RateMultiplier: 1, Tag: "chat",
	})
	require.NoError(t, err)
	require.Equal(t, "chat", repo.created.Tag)
	repo.getByID = group

	for _, tag := range []string{"image", "airp", ""} {
		updated, err := svc.UpdateGroup(context.Background(), group.ID, &UpdateGroupInput{Tag: &tag})
		require.NoError(t, err)
		require.Equal(t, tag, updated.Tag)
		require.Equal(t, tag, repo.updated.Tag)
		repo.getByID = updated
	}
}

func TestGroupTagOmissionPreservesExistingTag(t *testing.T) {
	repo := &groupRepoStubForAdmin{getByID: &Group{ID: 72, Name: "tagged", Platform: PlatformAnthropic, Tag: "airp"}}
	svc := &adminServiceImpl{groupRepo: repo}
	updated, err := svc.UpdateGroup(context.Background(), 72, &UpdateGroupInput{})
	require.NoError(t, err)
	require.Equal(t, "airp", updated.Tag)
}

func TestGroupTagRejectsUnknownValues(t *testing.T) {
	for _, tag := range []string{"unknown", "AIRP", "chat,image"} {
		require.Error(t, ValidateGroupTag(tag))
	}
}
