//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestModelProbeAdminGroupsMatchOrdinaryAPIKeyPage(t *testing.T) {
	for _, restricted := range []bool{false, true} {
		for _, role := range []string{RoleAdmin, RoleSubAdmin} {
			t.Run(role+map[bool]string{true: "/restricted", false: "/public"}[restricted], func(t *testing.T) {
				groups := []Group{
					{ID: 1}, {ID: 2, IsExclusive: true}, {ID: 3, IsExclusive: true},
					{ID: 4, IsExclusive: true, SubscriptionType: "subscription"}, {ID: 5},
					{ID: 6, IsExclusive: true, SubscriptionType: "subscription"},
				}
				for i := range groups {
					groups[i].Platform, groups[i].Status, groups[i].Hydrated = PlatformGemini, StatusActive, true
				}
				user := &User{ID: 4, Role: RoleUser, AllowedGroups: []int64{2, 5, 6}, RestrictPublicGroups: restricted}
				apiKeys := &APIKeyService{
					userRepo:  &visibilityUserRepo{user: user},
					groupRepo: &visibilityGroupRepo{groups: groups},
					userSubRepo: &visibilitySubRepo{subscriptions: []UserSubscription{
						{UserID: 4, GroupID: 4, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(time.Hour)},
						{UserID: 4, GroupID: 6, Status: SubscriptionStatusActive, ExpiresAt: time.Now().Add(-time.Hour)},
					}},
				}
				ordinary, err := apiKeys.GetAvailableGroups(context.Background(), 4)
				require.NoError(t, err)
				user.Role = role // A staff role must not widen the ordinary directory.
				_, repo, access, gateway, runner := modelProbeFixture()
				svc := NewModelProbeService(repo, access, gateway, runner, apiKeys)
				got, err := svc.Groups(context.Background(), 4)
				require.NoError(t, err)
				ids := make([]int64, 0, len(got))
				for i := range got {
					ids = append(ids, got[i].ID)
				}
				expected := []int64{1, 2, 4, 5}
				if restricted {
					expected = []int64{2, 4, 5}
				}
				require.Equal(t, expected, ids)
				require.Len(t, ordinary, len(got))
				for i := range ordinary {
					require.Equal(t, ordinary[i].ID, got[i].ID)
				}
			})
		}
	}
}
