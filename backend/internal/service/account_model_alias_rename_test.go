package service

import (
	"context"
	"errors"
	"net/http"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestNormalizeAccountModelAliasRenames(t *testing.T) {
	input := []AccountModelAliasRenameInput{
		{OldModel: " claude-sonnet-4-5 ", NewModel: " claude-sonnet-4-5-latest "},
		{OldModel: "CLAUDE-SONNET-4-5", NewModel: "CLAUDE-SONNET-4-5-LATEST"},
		{OldModel: " gemini-2.5-pro", NewModel: ""},
		{OldModel: "", NewModel: "gemini-2.5-pro-latest"},
		{OldModel: " gpt-5 ", NewModel: " GPT-5 "},
		{OldModel: " claude-opus-4-1 ", NewModel: " claude-opus-4-1-latest "},
	}

	got := normalizeAccountModelAliasRenames(input)

	require.Equal(t, []AccountModelAliasRename{
		{OldModel: "claude-sonnet-4-5", NewModel: "claude-sonnet-4-5-latest"},
		{OldModel: "claude-opus-4-1", NewModel: "claude-opus-4-1-latest"},
	}, got)
}

func TestAdminService_CascadeAccountModelAliasRenames(t *testing.T) {
	ctx := context.Background()

	t.Run("returns zero result when account has no bound groups", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity}}
		groupRepo := &accountAliasRenameGroupRepoStub{groups: []Group{{ID: 99, Platform: PlatformAntigravity}}}
		cascadeRepo := &accountAliasRenameCascadeRepoStub{}
		svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo, channelRepo: cascadeRepo}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: "claude-sonnet-4-5", NewModel: "claude-sonnet-4-5-latest"},
		})

		require.NoError(t, err)
		require.Equal(t, &AccountModelAliasRenameCascadeResult{}, got)
		require.Empty(t, groupRepo.platforms)
		require.Empty(t, cascadeRepo.calls)
	})

	t.Run("returns zero result when renames normalize to no-ops", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity, GroupIDs: []int64{1}}}
		groupRepo := &accountAliasRenameGroupRepoStub{groups: []Group{{ID: 1, Platform: PlatformAntigravity}}}
		cascadeRepo := &accountAliasRenameCascadeRepoStub{}
		svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo, channelRepo: cascadeRepo}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: " claude-sonnet-4-5 ", NewModel: " CLAUDE-SONNET-4-5 "},
			{OldModel: "   ", NewModel: "claude-opus-4-1"},
		})

		require.NoError(t, err)
		require.Equal(t, &AccountModelAliasRenameCascadeResult{}, got)
		require.Empty(t, groupRepo.platforms)
		require.Empty(t, cascadeRepo.calls)
	})

	t.Run("rejects non-antigravity accounts", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformOpenAI}}
		groupRepo := &accountAliasRenameGroupRepoStub{groups: []Group{{ID: 1, Platform: PlatformAntigravity}}}
		svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: "claude-sonnet-4-5", NewModel: "claude-sonnet-4-5-latest"},
		})

		require.Nil(t, got)
		var appErr *infraerrors.ApplicationError
		require.ErrorAs(t, err, &appErr)
		require.EqualValues(t, http.StatusBadRequest, appErr.Code)
		require.Equal(t, "ACCOUNT_MODEL_ALIAS_RENAME_INVALID_PLATFORM", appErr.Reason)
		require.Empty(t, groupRepo.platforms)
	})

	t.Run("delegates to optional repositories and merges counts", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity, GroupIDs: []int64{2, 1, 2, 0}}}
		groupRepo := &accountAliasRenameGroupRepoStub{groups: []Group{
			{ID: 99, Platform: PlatformAntigravity},
			{ID: 100, Platform: PlatformAntigravity},
		}}
		channelRepo := &accountAliasRenameCascadeRepoStub{
			result: AccountModelAliasRenameCascadeResult{
				ChannelPricingUpdated:  3,
				ChannelMappingsUpdated: 4,
				Skipped: []AccountModelAliasRenameSkipItem{{
					Scope: "channel_pricing", OldModel: "claude-sonnet-4-5", NewModel: "claude-sonnet-4-5-latest", Reason: "ambiguous",
				}},
			},
		}
		userRouteRepo := &accountAliasRenameCascadeRepoStub{
			result: AccountModelAliasRenameCascadeResult{
				UserCustomRoutesUpdated: 5,
				Skipped: []AccountModelAliasRenameSkipItem{{
					Scope: "user_custom_route", OwnerID: 42, OldModel: "claude-opus-4-1", NewModel: "claude-opus-4-1-latest", Reason: "missing",
				}},
			},
		}
		systemRouteRepo := &accountAliasRenameCascadeRepoStub{
			result: AccountModelAliasRenameCascadeResult{SystemCustomRoutesUpdated: 6},
		}
		svc := &adminServiceImpl{
			accountRepo:           accountRepo,
			groupRepo:             groupRepo,
			channelRepo:           channelRepo,
			userCustomGroupRepo:   userRouteRepo,
			systemCustomGroupRepo: systemRouteRepo,
		}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: " claude-sonnet-4-5 ", NewModel: " claude-sonnet-4-5-latest "},
			{OldModel: "claude-opus-4-1", NewModel: "claude-opus-4-1-latest"},
		})

		require.NoError(t, err)
		require.Equal(t, 3, got.ChannelPricingUpdated)
		require.Equal(t, 4, got.ChannelMappingsUpdated)
		require.Equal(t, 5, got.UserCustomRoutesUpdated)
		require.Equal(t, 6, got.SystemCustomRoutesUpdated)
		require.Len(t, got.Skipped, 2)

		wantRenames := []AccountModelAliasRename{
			{OldModel: "claude-sonnet-4-5", NewModel: "claude-sonnet-4-5-latest"},
			{OldModel: "claude-opus-4-1", NewModel: "claude-opus-4-1-latest"},
		}
		wantGroupIDs := []int64{2, 1}
		require.Equal(t, []accountAliasRenameCascadeCall{{accountID: 7, groupIDs: wantGroupIDs, renames: wantRenames}}, channelRepo.calls)
		require.Equal(t, channelRepo.calls, userRouteRepo.calls)
		require.Equal(t, channelRepo.calls, systemRouteRepo.calls)
		require.Empty(t, groupRepo.platforms)
	})

	t.Run("invalidates channel cache immediately after successful channel updates", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity, GroupIDs: []int64{1}}}
		spy := &accountAliasRenameChannelCacheInvalidatorSpy{}
		channelRepo := &accountAliasRenameCascadeRepoStub{
			result: AccountModelAliasRenameCascadeResult{ChannelPricingUpdated: 1},
			event:  "channel",
			events: &spy.events,
		}
		userRouteRepo := &accountAliasRenameCascadeRepoStub{
			result: AccountModelAliasRenameCascadeResult{UserCustomRoutesUpdated: 1},
			event:  "user",
			events: &spy.events,
		}
		svc := &adminServiceImpl{
			accountRepo:             accountRepo,
			channelRepo:             channelRepo,
			userCustomGroupRepo:     userRouteRepo,
			channelCacheInvalidator: spy,
		}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: "old-model", NewModel: "new-model"},
		})

		require.NoError(t, err)
		require.Equal(t, 1, got.ChannelPricingUpdated)
		require.Equal(t, 1, got.UserCustomRoutesUpdated)
		require.Equal(t, []string{"channel", "invalidate", "user"}, spy.events)
	})

	t.Run("invalidates channel cache for mapping updates", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity, GroupIDs: []int64{1}}}
		spy := &accountAliasRenameChannelCacheInvalidatorSpy{}
		channelRepo := &accountAliasRenameCascadeRepoStub{
			result: AccountModelAliasRenameCascadeResult{ChannelMappingsUpdated: 1},
			event:  "channel",
			events: &spy.events,
		}
		svc := &adminServiceImpl{
			accountRepo:             accountRepo,
			channelRepo:             channelRepo,
			channelCacheInvalidator: spy,
		}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: "old-model", NewModel: "new-model"},
		})

		require.NoError(t, err)
		require.Equal(t, 1, got.ChannelMappingsUpdated)
		require.Equal(t, []string{"channel", "invalidate"}, spy.events)
	})

	t.Run("does not invalidate channel cache when channel delegate has no updates", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity, GroupIDs: []int64{1}}}
		spy := &accountAliasRenameChannelCacheInvalidatorSpy{}
		channelRepo := &accountAliasRenameCascadeRepoStub{event: "channel", events: &spy.events}
		svc := &adminServiceImpl{
			accountRepo:             accountRepo,
			channelRepo:             channelRepo,
			channelCacheInvalidator: spy,
		}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: "old-model", NewModel: "new-model"},
		})

		require.NoError(t, err)
		require.Equal(t, &AccountModelAliasRenameCascadeResult{}, got)
		require.Equal(t, []string{"channel"}, spy.events)
	})

	t.Run("does not invalidate channel cache when channel delegate errors", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity, GroupIDs: []int64{1}}}
		spy := &accountAliasRenameChannelCacheInvalidatorSpy{}
		channelRepo := &accountAliasRenameCascadeRepoStub{err: errors.New("channel cascade failed"), event: "channel", events: &spy.events}
		svc := &adminServiceImpl{
			accountRepo:             accountRepo,
			channelRepo:             channelRepo,
			channelCacheInvalidator: spy,
		}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: "old-model", NewModel: "new-model"},
		})

		require.Nil(t, got)
		require.ErrorContains(t, err, "channel cascade failed")
		require.Equal(t, []string{"channel"}, spy.events)
	})
}

func TestUpdateAccountCascadesAntigravityModelAliasRenames(t *testing.T) {
	accountID := int64(7)
	accountRepo := &accountAliasRenameUpdateAccountRepoStub{account: &Account{
		ID:       accountID,
		Platform: PlatformAntigravity,
		Type:     AccountTypeAPIKey,
		Status:   StatusActive,
		GroupIDs: []int64{99},
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"old-public-model": "upstream-model",
				"stable-model":     "stable-upstream",
			},
		},
	}}
	userRouteRepo := &accountAliasRenameCascadeRepoStub{
		result: AccountModelAliasRenameCascadeResult{UserCustomRoutesUpdated: 1},
	}
	svc := &adminServiceImpl{
		accountRepo:         accountRepo,
		userCustomGroupRepo: userRouteRepo,
	}

	updated, err := svc.UpdateAccount(context.Background(), accountID, &UpdateAccountInput{
		Credentials: map[string]any{
			"model_mapping": map[string]any{
				"new-public-model": "upstream-model",
				"stable-model":     "stable-upstream",
			},
		},
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, userRouteRepo.calls, 1)
	require.Equal(t, accountAliasRenameCascadeCall{
		accountID: accountID,
		groupIDs:  []int64{99},
		renames: []AccountModelAliasRename{{
			OldModel: "old-public-model",
			NewModel: "new-public-model",
		}},
	}, userRouteRepo.calls[0])
}

func TestNewAdminServiceWiresAliasCascadeRepositories(t *testing.T) {
	channelRepo := &accountAliasRenameChannelRepoWithCascade{}
	userCustomGroupRepo := &accountAliasRenameUserCustomGroupRepoWithCascade{}
	systemCustomGroupRepo := &accountAliasRenameSystemCustomGroupRepoWithCascade{}

	svc := NewAdminService(
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		nil,
		channelRepo,
		userCustomGroupRepo,
		systemCustomGroupRepo,
		nil,
	)

	impl, ok := svc.(*adminServiceImpl)
	require.True(t, ok)
	require.Same(t, channelRepo, impl.channelRepo)
	require.Same(t, userCustomGroupRepo, impl.userCustomGroupRepo)
	require.Same(t, systemCustomGroupRepo, impl.systemCustomGroupRepo)
}

type accountAliasRenameAccountRepoStub struct {
	AccountRepository
	account *Account
	err     error
}

func (s *accountAliasRenameAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if s.err != nil {
		return nil, s.err
	}
	if s.account == nil || s.account.ID != id {
		return nil, errors.New("account not found")
	}
	return s.account, nil
}

type accountAliasRenameUpdateAccountRepoStub struct {
	AccountRepository
	account     *Account
	updateCalls int
}

func (s *accountAliasRenameUpdateAccountRepoStub) GetByID(_ context.Context, id int64) (*Account, error) {
	if s.account == nil || s.account.ID != id {
		return nil, errors.New("account not found")
	}
	return cloneAliasRenameTestAccount(s.account), nil
}

func (s *accountAliasRenameUpdateAccountRepoStub) Update(_ context.Context, account *Account) error {
	s.account = cloneAliasRenameTestAccount(account)
	s.updateCalls++
	return nil
}

func (s *accountAliasRenameUpdateAccountRepoStub) BindGroups(_ context.Context, accountID int64, groupIDs []int64) error {
	if s.account == nil || s.account.ID != accountID {
		return errors.New("account not found")
	}
	s.account.GroupIDs = append([]int64(nil), groupIDs...)
	return nil
}

func (s *accountAliasRenameUpdateAccountRepoStub) ListShadowsByParent(context.Context, int64) ([]*Account, error) {
	return nil, nil
}

func cloneAliasRenameTestAccount(account *Account) *Account {
	if account == nil {
		return nil
	}
	clone := *account
	clone.GroupIDs = append([]int64(nil), account.GroupIDs...)
	clone.Credentials = mergeMap(nil, account.Credentials)
	clone.Extra = mergeMap(nil, account.Extra)
	return &clone
}

type accountAliasRenameGroupRepoStub struct {
	GroupRepository
	groups    []Group
	err       error
	platforms []string
}

func (s *accountAliasRenameGroupRepoStub) ListActiveByPlatform(_ context.Context, platform string) ([]Group, error) {
	s.platforms = append(s.platforms, platform)
	return s.groups, s.err
}

type accountAliasRenameCascadeCall struct {
	accountID int64
	groupIDs  []int64
	renames   []AccountModelAliasRename
}

type accountAliasRenameCascadeRepoStub struct {
	result AccountModelAliasRenameCascadeResult
	err    error
	calls  []accountAliasRenameCascadeCall
	event  string
	events *[]string
}

func (s *accountAliasRenameCascadeRepoStub) CascadeAccountModelAliasRenames(ctx context.Context, accountID int64, groupIDs []int64, renames []AccountModelAliasRename) (*AccountModelAliasRenameCascadeResult, error) {
	if s.events != nil && s.event != "" {
		*s.events = append(*s.events, s.event)
	}
	s.calls = append(s.calls, accountAliasRenameCascadeCall{
		accountID: accountID,
		groupIDs:  append([]int64(nil), groupIDs...),
		renames:   append([]AccountModelAliasRename(nil), renames...),
	})
	if s.err != nil {
		return nil, s.err
	}
	return &s.result, nil
}

type accountAliasRenameChannelRepoWithCascade struct {
	ChannelRepository
	accountAliasRenameCascadeRepoStub
}

type accountAliasRenameUserCustomGroupRepoWithCascade struct {
	UserCustomGroupRepository
	accountAliasRenameCascadeRepoStub
}

type accountAliasRenameSystemCustomGroupRepoWithCascade struct {
	SystemCustomGroupRepository
	accountAliasRenameCascadeRepoStub
}

type accountAliasRenameChannelCacheInvalidatorSpy struct {
	events []string
}

func (s *accountAliasRenameChannelCacheInvalidatorSpy) InvalidateCache() {
	s.events = append(s.events, "invalidate")
}
