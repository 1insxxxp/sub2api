//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateGroupPreservesEmptyResponseCompensationPolicy(t *testing.T) {
	repo := &groupRepoStubForAdmin{createID: 71}
	svc := &adminServiceImpl{groupRepo: repo}

	group, err := svc.CreateGroup(context.Background(), &CreateGroupInput{
		Name: "refund-policy", Platform: PlatformAnthropic, RateMultiplier: 1,
		EmptyResponseCompensationEnabled: true,
	})

	require.NoError(t, err)
	require.True(t, group.EmptyResponseCompensationEnabled)
	require.NotNil(t, repo.created)
	require.True(t, repo.created.EmptyResponseCompensationEnabled)
}

func TestUpdateGroupPreservesExplicitEmptyResponseCompensationPolicy(t *testing.T) {
	enabled := true
	repo := &groupRepoStubForAdmin{getByID: &Group{
		ID: 72, Name: "refund-policy", Platform: PlatformAnthropic, RateMultiplier: 1,
		Status: StatusActive, SubscriptionType: SubscriptionTypeStandard,
	}}
	svc := &adminServiceImpl{groupRepo: repo}

	group, err := svc.UpdateGroup(context.Background(), 72, &UpdateGroupInput{EmptyResponseCompensationEnabled: &enabled})

	require.NoError(t, err)
	require.True(t, group.EmptyResponseCompensationEnabled)
	require.True(t, repo.updated.EmptyResponseCompensationEnabled)
}
