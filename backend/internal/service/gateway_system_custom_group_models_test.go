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
