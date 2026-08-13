package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type systemCustomGroupBatchAccountRepoStub struct {
	AccountRepository
	accounts  []SystemCustomGroupSchedulableAccount
	calls     int
	requested []int64
}

func (s *systemCustomGroupBatchAccountRepoStub) ListSchedulableByGroupIDs(_ context.Context, groupIDs []int64) ([]SystemCustomGroupSchedulableAccount, error) {
	s.calls++
	s.requested = append([]int64(nil), groupIDs...)
	return append([]SystemCustomGroupSchedulableAccount(nil), s.accounts...), nil
}

func TestGatewayListSystemCustomGroupModelAvailabilityUsesSchedulerSupportInOneBatch(t *testing.T) {
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{
			GroupID: 10,
			Account: Account{
				ID: 101, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{
					"claude-*": "upstream-claude",
				}},
			},
		},
		{
			GroupID: 20,
			Account: Account{
				ID: 202, Platform: PlatformAnthropic, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{
					"Claude-*": "upstream-claude",
				}},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	availability, err := svc.ListSystemCustomGroupModelAvailability(context.Background(), []SystemCustomGroupModelListSource{
		{Group: *directSourceGroup(10, PlatformAnthropic), Models: []string{"claude-sonnet-4-6"}},
		{Group: *directSourceGroup(20, PlatformAnthropic), Models: []string{"claude-sonnet-4-6"}},
	})

	require.NoError(t, err)
	require.True(t, availability[10]["claude-sonnet-4-6"], "scheduler wildcard support must make the alias visible")
	require.False(t, availability[20]["claude-sonnet-4-6"], "display normalization must not broaden case-sensitive scheduler support")
	require.Equal(t, 1, repo.calls, "all source groups must share one account snapshot query")
	require.ElementsMatch(t, []int64{10, 20}, repo.requested)
}

func TestGatewayListSystemCustomGroupModelAvailabilityIncludesRouteServedByEmptyMappingAccount(t *testing.T) {
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{
			GroupID: 10,
			Account: Account{
				ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{
					"foo": "upstream-foo",
				}},
			},
		},
		{
			GroupID: 10,
			Account: Account{
				ID: 102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Credentials: map[string]any{},
			},
		},
	}}
	svc := &GatewayService{accountRepo: repo}

	availability, err := svc.ListSystemCustomGroupModelAvailability(context.Background(), []SystemCustomGroupModelListSource{
		{Group: *directSourceGroup(10, PlatformOpenAI), Models: []string{"gpt-5.5"}},
	})

	require.NoError(t, err)
	require.True(t, availability[10]["gpt-5.5"], "an explicit route must stay visible when any real scheduler candidate can serve it")
	require.Equal(t, 1, repo.calls)
}

func TestGatewayListSystemCustomGroupModelAvailabilityKeepsSourceAndRuntimeControls(t *testing.T) {
	customNotSelected := directSourceGroup(30, PlatformOpenAI)
	customNotSelected.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.4"}}
	customSelected := directSourceGroup(31, PlatformOpenAI)
	customSelected.ModelsListConfig = GroupModelsListConfig{Enabled: true, Models: []string{"gpt-5.5"}}
	repo := &systemCustomGroupBatchAccountRepoStub{accounts: []SystemCustomGroupSchedulableAccount{
		{GroupID: 20, Account: schedulableSystemCustomTestAccount(201, PlatformOpenAI, map[string]any{"foo": "upstream-foo"})},
		{GroupID: 30, Account: schedulableSystemCustomTestAccount(301, PlatformOpenAI, nil)},
		{GroupID: 31, Account: schedulableSystemCustomTestAccount(311, PlatformOpenAI, map[string]any{"foo": "upstream-foo"})},
		{GroupID: 31, Account: schedulableSystemCustomTestAccount(312, PlatformOpenAI, nil)},
		{GroupID: 40, Account: schedulableSystemCustomTestAccount(401, PlatformGemini, nil)},
		{GroupID: 50, Account: schedulableSystemCustomTestAccount(501, PlatformOpenAI, nil)},
	}}
	svc := &GatewayService{accountRepo: repo}

	availability, err := svc.ListSystemCustomGroupModelAvailability(context.Background(), []SystemCustomGroupModelListSource{
		{Group: *directSourceGroup(20, PlatformOpenAI), Models: []string{"gpt-5.5"}},
		{Group: *customNotSelected, Models: []string{"gpt-5.5"}},
		{Group: *customSelected, Models: []string{"gpt-5.5"}},
		{Group: *directSourceGroup(40, PlatformGemini), Models: []string{"gemini-2.0-flash"}},
		{Group: *directSourceGroup(50, PlatformOpenAI), Models: []string{"gpt-5.4"}},
	})

	require.NoError(t, err)
	require.False(t, availability[20]["gpt-5.5"], "non-empty mappings that all reject the route must keep it hidden")
	require.False(t, availability[30]["gpt-5.5"], "an enabled custom model list must remain a source-group whitelist")
	require.True(t, availability[31]["gpt-5.5"], "a selected custom model may use any real scheduler candidate")
	require.False(t, availability[40]["gemini-2.0-flash"], "provider lifecycle restrictions must still hide unavailable models")
	require.True(t, availability[50]["gpt-5.4"], "default models on empty mappings must remain visible")
	require.Equal(t, 1, repo.calls)
}

func schedulableSystemCustomTestAccount(id int64, platform string, mapping map[string]any) Account {
	credentials := map[string]any{}
	if mapping != nil {
		credentials["model_mapping"] = mapping
	}
	return Account{
		ID: id, Platform: platform, Type: AccountTypeAPIKey,
		Status: StatusActive, Schedulable: true, Credentials: credentials,
	}
}
