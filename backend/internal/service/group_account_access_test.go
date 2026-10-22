//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/stretchr/testify/require"
)

func TestGroupAccountAccessScopesRulesAndClonesSnapshot(t *testing.T) {
	groupID, otherGroup := int64(10), int64(20)
	rules := GroupAccountExclusions{groupID: {101}}
	ctx := WithGroupAccountExclusions(context.Background(), 7, rules)
	require.False(t, IsGroupAccountAllowedForUser(ctx, &groupID, 101))
	require.True(t, IsGroupAccountAllowedForUser(ctx, &groupID, 102))
	require.True(t, IsGroupAccountAllowedForUser(ctx, &otherGroup, 101))
	require.True(t, IsGroupAccountAllowedForUser(ctx, nil, 101))
	rules[groupID][0] = 102
	delete(rules, groupID)
	require.False(t, IsGroupAccountAllowedForUser(ctx, &groupID, 101), "shared auth snapshots must not mutate an in-flight policy")
	require.True(t, IsGroupAccountAllowedForUser(ctx, &groupID, 102))
	otherUser := WithGroupAccountExclusions(ctx, 8, nil)
	require.True(t, IsGroupAccountAllowedForUser(otherUser, &groupID, 101))
}

func TestGroupAccountAccessUsesResolvedSchedulingGroup(t *testing.T) {
	groupID, sourceID, fallbackID := int64(99), int64(10), int64(20)
	ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{sourceID: {101}, groupID: {102}})
	ctx = context.WithValue(ctx, ctxkey.Group, &Group{ID: groupID})
	ctx = WithSystemCustomGroupResolution(ctx, SystemCustomGroupResolution{
		BillingGroupID: groupID, SourceGroupID: sourceID, PublicModel: "alias", SourceModel: "model",
		SourcePlatform: PlatformAnthropic, AllowedAccounts: NewSystemCustomGroupAccountAllowlist([]int64{101, 102}),
	})
	require.False(t, IsGroupAccountAllowedForUser(ctx, &sourceID, 101))
	require.True(t, IsGroupAccountAllowedForUser(ctx, &sourceID, 102), "billing container rules must not replace source-group rules")
	scoped := withGroupAccountAccessScope(ctx, &fallbackID)
	require.True(t, groupAccountAllowedInSchedulingContext(scoped, 101), "fallback must replace the previous source scope")
}

func TestGroupAccountAccessDefaultAndRebinding(t *testing.T) {
	groupID := int64(10)
	require.True(t, IsGroupAccountAllowedForUser(context.Background(), &groupID, 101))
	require.True(t, IsGroupAccountAllowedForUser(nil, &groupID, 101))
	ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{groupID: {101}})
	require.True(t, IsGroupAccountAllowedForUser(WithGroupAccountExclusions(ctx, 0, nil), &groupID, 101))
	// Scheduling metadata cannot replace the policy bound by authentication.
	ctx = context.WithValue(ctx, ctxkey.UserID, int64(999))
	require.False(t, IsGroupAccountAllowedForUser(ctx, &groupID, 101))
}

func TestGroupAccountAccessFiltersGatewayAndOpenAISelection(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			groupID := int64(10)
			blocked := schedulableSystemCustomTestAccount(101, platform, map[string]any{"shared": "gpt-5.4"})
			allowed := schedulableSystemCustomTestAccount(102, platform, map[string]any{"shared": "gpt-5.4"})
			blocked.Priority, allowed.Priority = 1, 2
			repo := &systemCustomDispatchAccountRepoStub{accounts: []Account{blocked, allowed}}
			ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{groupID: {101}})
			var selectAccount func(context.Context, map[int64]struct{}) (*Account, error)
			if platform == PlatformOpenAI {
				svc := &OpenAIGatewayService{accountRepo: repo, billingService: newTestBillingService(), cfg: testConfig()}
				selectAccount = func(c context.Context, failed map[int64]struct{}) (*Account, error) {
					return svc.SelectAccountForModelWithExclusions(c, &groupID, "", "shared", failed)
				}
			} else {
				svc := &GatewayService{accountRepo: repo, groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: directSourceGroup(groupID, platform)}}, billingService: newTestBillingService(), cfg: testConfig()}
				selectAccount = func(c context.Context, failed map[int64]struct{}) (*Account, error) {
					return svc.SelectAccountForModelWithExclusions(c, &groupID, "", "shared", failed)
				}
			}
			selected, err := selectAccount(ctx, nil)
			require.NoError(t, err)
			require.Equal(t, int64(102), selected.ID)
			// Resetting transient failover state must never clear access rules.
			selected, err = selectAccount(ctx, map[int64]struct{}{})
			require.NoError(t, err)
			require.Equal(t, int64(102), selected.ID)
			_, err = selectAccount(ctx, map[int64]struct{}{102: {}})
			require.ErrorIs(t, err, ErrNoAvailableAccounts)
			require.NotContains(t, err.Error(), "101")
			selected, err = selectAccount(WithGroupAccountExclusions(ctx, 8, nil), nil)
			require.NoError(t, err)
			require.Equal(t, int64(101), selected.ID)
		})
	}
}

type groupAccountAccessStickyCache struct {
	GatewayCache
	pinned int64
}

func (c *groupAccountAccessStickyCache) GetSessionAccountID(context.Context, int64, string) (int64, error) {
	return c.pinned, nil
}
func (c *groupAccountAccessStickyCache) SetSessionAccountID(_ context.Context, _ int64, _ string, accountID int64, _ time.Duration) error {
	c.pinned = accountID
	return nil
}
func (c *groupAccountAccessStickyCache) DeleteSessionAccountID(context.Context, int64, string) error {
	c.pinned = 0
	return nil
}
func (c *groupAccountAccessStickyCache) RefreshSessionTTL(context.Context, int64, string, time.Duration) error {
	return nil
}

func TestGroupAccountAccessRejectsStickyAndHydration(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI, PlatformGemini} {
		t.Run(platform, func(t *testing.T) {
			groupID := int64(10)
			blocked := schedulableSystemCustomTestAccount(101, platform, map[string]any{"shared": "gpt-5.4"})
			allowed := schedulableSystemCustomTestAccount(102, platform, map[string]any{"shared": "gpt-5.4"})
			blocked.Priority, allowed.Priority = 1, 2
			blocked.GroupIDs, allowed.GroupIDs = []int64{groupID}, []int64{groupID}
			repo := &systemCustomDispatchAccountRepoStub{accounts: []Account{blocked, allowed}}
			cache := &groupAccountAccessStickyCache{pinned: blocked.ID}
			groups := &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: directSourceGroup(groupID, platform)}}
			ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{groupID: {blocked.ID}})
			var selected *Account
			var err error
			if platform == PlatformOpenAI {
				svc := &OpenAIGatewayService{accountRepo: repo, cache: cache, billingService: newTestBillingService(), cfg: testConfig()}
				selected, err = svc.SelectAccountForModelWithExclusions(ctx, &groupID, "sticky", "shared", nil)
				_, hydrateErr := svc.hydrateSelectedAccount(withGroupAccountAccessScope(ctx, &groupID), &blocked)
				require.ErrorIs(t, hydrateErr, ErrNoAvailableAccounts)
			} else if platform == PlatformGemini {
				svc := &GeminiMessagesCompatService{accountRepo: repo, groupRepo: groups, cache: cache, cfg: testConfig()}
				selected, err = svc.SelectAccountForModelWithExclusions(ctx, &groupID, "sticky", "shared", nil)
				_, hydrateErr := svc.hydrateSelectedAccount(withGroupAccountAccessScope(ctx, &groupID), &blocked)
				require.ErrorIs(t, hydrateErr, ErrNoAvailableAccounts)
			} else {
				svc := &GatewayService{accountRepo: repo, groupRepo: groups, cache: cache, billingService: newTestBillingService(), cfg: testConfig()}
				selected, err = svc.SelectAccountForModelWithExclusions(ctx, &groupID, "sticky", "shared", nil)
				_, hydrateErr := svc.hydrateSelectedAccount(withGroupAccountAccessScope(ctx, &groupID), &blocked)
				require.ErrorIs(t, hydrateErr, ErrNoAvailableAccounts)
			}
			require.NoError(t, err)
			require.NotNil(t, selected)
			require.Equal(t, allowed.ID, selected.ID)
		})
	}
}

func TestGroupAccountAccessRejectsAdvancedSchedulerCandidates(t *testing.T) {
	groupID, otherGroup := int64(10), int64(20)
	ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{groupID: {101}})
	account := schedulableSystemCustomTestAccount(101, PlatformOpenAI, map[string]any{"shared": "gpt-5.4"})
	scheduler := &defaultOpenAIAccountScheduler{}
	compatible, _ := scheduler.isAccountRequestCompatibleReason(ctx, &account, OpenAIAccountScheduleRequest{GroupID: &groupID})
	require.False(t, compatible)
	// An account attached to multiple groups is still allowed through other groups.
	compatible, _ = scheduler.isAccountRequestCompatibleReason(ctx, &account, OpenAIAccountScheduleRequest{GroupID: &otherGroup})
	require.True(t, compatible)
}

func TestGroupAccountAccessUsesGatewayFallbackGroup(t *testing.T) {
	originalID, fallbackID := int64(10), int64(20)
	original := directSourceGroup(originalID, PlatformAnthropic)
	original.ClaudeCodeOnly, original.FallbackGroupID = true, &fallbackID
	blocked := schedulableSystemCustomTestAccount(101, PlatformAnthropic, nil)
	allowed := schedulableSystemCustomTestAccount(102, PlatformAnthropic, nil)
	blocked.Priority, allowed.Priority = 1, 2
	repo := &systemCustomDispatchAccountRepoStub{accounts: []Account{blocked, allowed}}
	svc := &GatewayService{accountRepo: repo, groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{
		originalID: original, fallbackID: directSourceGroup(fallbackID, PlatformAnthropic),
	}}, cfg: testConfig()}
	ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{originalID: {allowed.ID}, fallbackID: {blocked.ID}})
	selected, err := svc.SelectAccountForModelWithExclusions(ctx, &originalID, "", "", nil)
	require.NoError(t, err)
	require.Equal(t, allowed.ID, selected.ID, "only the resolved fallback group's rule applies")
}

func TestGroupAccountAccessUsesSystemCustomSourceGroup(t *testing.T) {
	for _, platform := range []string{PlatformAnthropic, PlatformOpenAI} {
		t.Run(platform, func(t *testing.T) {
			billingID, sourceID := int64(99), int64(10)
			first := schedulableSystemCustomTestAccount(101, platform, map[string]any{"shared": "gpt-5.4"})
			second := schedulableSystemCustomTestAccount(102, platform, map[string]any{"shared": "gpt-5.4"})
			first.Priority, second.Priority = 1, 2
			ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{billingID: {second.ID}, sourceID: {first.ID}})
			ctx = WithSystemCustomGroupResolution(ctx, SystemCustomGroupResolution{BillingGroupID: billingID, SourceGroupID: sourceID,
				PublicModel: "shared", SourceModel: "shared", SourcePlatform: platform, AllowedAccounts: NewSystemCustomGroupAccountAllowlist([]int64{first.ID, second.ID})})
			repo := &systemCustomDispatchAccountRepoStub{accounts: []Account{first, second}}
			var selected *Account
			var err error
			if platform == PlatformOpenAI {
				svc := &OpenAIGatewayService{accountRepo: repo, billingService: newTestBillingService(), cfg: testConfig()}
				selected, err = svc.SelectAccountForModelWithExclusions(ctx, &sourceID, "", "shared", nil)
			} else {
				svc := &GatewayService{accountRepo: repo, groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{sourceID: directSourceGroup(sourceID, platform)}}, billingService: newTestBillingService(), cfg: testConfig()}
				selected, err = svc.SelectAccountForModelWithExclusions(ctx, &sourceID, "", "shared", nil)
			}
			require.NoError(t, err)
			require.Equal(t, second.ID, selected.ID)
		})
	}
}

func TestGroupAccountAccessFiltersSnapshotCandidates(t *testing.T) {
	groupID := int64(10)
	first, second := &Account{ID: 101}, &Account{ID: 102}
	snapshot := NewSchedulerSnapshotService(&snapshotHydrationCache{snapshot: []*Account{first, second}}, nil, nil, nil, nil)
	ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{groupID: {first.ID}})
	gw := &GatewayService{schedulerSnapshot: snapshot}
	accounts, _, err := gw.listSchedulableAccounts(ctx, &groupID, PlatformAnthropic, false)
	require.NoError(t, err)
	require.Equal(t, []Account{*second}, accounts)
	oa := &OpenAIGatewayService{schedulerSnapshot: snapshot}
	accounts, err = oa.listSchedulableAccounts(ctx, &groupID, PlatformOpenAI)
	require.NoError(t, err)
	require.Equal(t, []Account{*second}, accounts)
}

func TestGroupAccountAccessRejectsPreviousResponseAccount(t *testing.T) {
	groupID := int64(10)
	account := schedulableSystemCustomTestAccount(101, PlatformOpenAI, nil)
	account.Extra = map[string]any{"openai_apikey_responses_websockets_v2_enabled": true}
	cfg := testConfig()
	cfg.Gateway.OpenAIWS.Enabled, cfg.Gateway.OpenAIWS.APIKeyEnabled, cfg.Gateway.OpenAIWS.ResponsesWebsocketsV2 = true, true, true
	cfg.Gateway.OpenAIWS.StickyResponseIDTTLSeconds = 3600
	svc := &OpenAIGatewayService{accountRepo: schedulerTestOpenAIAccountRepo{accounts: []Account{account}}, cache: &schedulerTestGatewayCache{}, cfg: cfg}
	store := svc.getOpenAIWSStateStore()
	ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{groupID: {account.ID}})
	require.NoError(t, store.BindResponseAccount(ctx, groupID, "resp_private", account.ID, time.Hour))
	// Direct continuation resolution must use its explicit scheduling group,
	// including callers without an auth group value in the context.
	require.Zero(t, svc.ResolveAccountIDByPreviousResponseIDForScheduler(ctx, &groupID, "resp_private", "", nil, "", false))
	boundID, bindErr := store.GetResponseAccount(ctx, groupID, "resp_private")
	require.NoError(t, bindErr)
	require.Equal(t, account.ID, boundID)
	selection, _, err := svc.SelectAccountWithScheduler(ctx, &groupID, "resp_private", "", "", nil, OpenAIUpstreamTransportAny, false)
	require.ErrorIs(t, err, ErrNoAvailableAccounts)
	require.Nil(t, selection)
}

type groupAccountForcedGeminiRepo struct {
	*systemCustomDispatchAccountRepoStub
}

func (r *groupAccountForcedGeminiRepo) ListSchedulableByGroupIDAndPlatforms(context.Context, int64, []string) ([]Account, error) {
	return nil, nil
}
func (r *groupAccountForcedGeminiRepo) ListSchedulableUngroupedByPlatforms(context.Context, []string) ([]Account, error) {
	return append([]Account(nil), r.accounts...), nil
}

func TestGroupAccountAccessGeminiForcedPlatformFallbackKeepsRequestGroup(t *testing.T) {
	groupID := int64(10)
	first := schedulableSystemCustomTestAccount(101, PlatformGemini, nil)
	second := schedulableSystemCustomTestAccount(102, PlatformGemini, nil)
	first.Priority, second.Priority = 1, 2
	svc := &GeminiMessagesCompatService{accountRepo: &groupAccountForcedGeminiRepo{&systemCustomDispatchAccountRepoStub{accounts: []Account{first, second}}}, cfg: testConfig()}
	ctx := WithGroupAccountExclusions(context.Background(), 7, GroupAccountExclusions{groupID: {first.ID}})
	ctx = context.WithValue(ctx, ctxkey.ForcePlatform, PlatformGemini)
	selected, err := svc.SelectAccountForModelWithExclusions(ctx, &groupID, "", "", nil)
	require.NoError(t, err)
	require.Equal(t, second.ID, selected.ID)
}
