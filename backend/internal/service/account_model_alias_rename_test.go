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

	t.Run("returns zero result when no active antigravity groups", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity}}
		groupRepo := &accountAliasRenameGroupRepoStub{}
		cascadeRepo := &accountAliasRenameCascadeRepoStub{}
		svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo, channelRepo: cascadeRepo}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: "claude-sonnet-4-5", NewModel: "claude-sonnet-4-5-latest"},
		})

		require.NoError(t, err)
		require.Equal(t, &AccountModelAliasRenameCascadeResult{}, got)
		require.Equal(t, []string{PlatformAntigravity}, groupRepo.platforms)
		require.Empty(t, cascadeRepo.calls)
	})

	t.Run("returns zero result when renames normalize to no-ops", func(t *testing.T) {
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity}}
		groupRepo := &accountAliasRenameGroupRepoStub{groups: []Group{{ID: 1, Platform: PlatformAntigravity}}}
		cascadeRepo := &accountAliasRenameCascadeRepoStub{}
		svc := &adminServiceImpl{accountRepo: accountRepo, groupRepo: groupRepo, channelRepo: cascadeRepo}

		got, err := svc.CascadeAccountModelAliasRenames(ctx, 7, []AccountModelAliasRenameInput{
			{OldModel: " claude-sonnet-4-5 ", NewModel: " CLAUDE-SONNET-4-5 "},
			{OldModel: "   ", NewModel: "claude-opus-4-1"},
		})

		require.NoError(t, err)
		require.Equal(t, &AccountModelAliasRenameCascadeResult{}, got)
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
		accountRepo := &accountAliasRenameAccountRepoStub{account: &Account{ID: 7, Platform: PlatformAntigravity}}
		groupRepo := &accountAliasRenameGroupRepoStub{groups: []Group{
			{ID: 1, Platform: PlatformAntigravity},
			{ID: 2, Platform: PlatformAntigravity},
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
		wantGroupIDs := []int64{1, 2}
		require.Equal(t, []accountAliasRenameCascadeCall{{accountID: 7, groupIDs: wantGroupIDs, renames: wantRenames}}, channelRepo.calls)
		require.Equal(t, channelRepo.calls, userRouteRepo.calls)
		require.Equal(t, channelRepo.calls, systemRouteRepo.calls)
	})
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
}

func (s *accountAliasRenameCascadeRepoStub) CascadeAccountModelAliasRenames(ctx context.Context, accountID int64, groupIDs []int64, renames []AccountModelAliasRename) (*AccountModelAliasRenameCascadeResult, error) {
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
