package service

import (
	"context"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AccountModelAliasRenameInput struct {
	OldModel string `json:"old_model"`
	NewModel string `json:"new_model"`
}

type AccountModelAliasRename struct {
	OldModel string `json:"old_model"`
	NewModel string `json:"new_model"`
}

type AccountModelAliasRenameSkipItem struct {
	Scope    string `json:"scope"`
	OwnerID  int64  `json:"owner_id,omitempty"`
	OldModel string `json:"old_model"`
	NewModel string `json:"new_model"`
	Reason   string `json:"reason"`
}

type AccountModelAliasRenameCascadeResult struct {
	ChannelPricingUpdated     int                               `json:"channel_pricing_updated"`
	ChannelMappingsUpdated    int                               `json:"channel_mappings_updated"`
	UserCustomRoutesUpdated   int                               `json:"user_custom_routes_updated"`
	SystemCustomRoutesUpdated int                               `json:"system_custom_routes_updated"`
	Skipped                   []AccountModelAliasRenameSkipItem `json:"skipped"`
}

type accountModelAliasRenameCascadeRepository interface {
	CascadeAccountModelAliasRenames(ctx context.Context, accountID int64, groupIDs []int64, renames []AccountModelAliasRename) (*AccountModelAliasRenameCascadeResult, error)
}

func normalizeAccountModelAliasRenames(input []AccountModelAliasRenameInput) []AccountModelAliasRename {
	renames := make([]AccountModelAliasRename, 0, len(input))
	seen := make(map[string]struct{}, len(input))
	for _, item := range input {
		oldModel := strings.TrimSpace(item.OldModel)
		newModel := strings.TrimSpace(item.NewModel)
		if oldModel == "" || newModel == "" || strings.EqualFold(oldModel, newModel) {
			continue
		}

		key := strings.ToLower(oldModel) + "\x00" + strings.ToLower(newModel)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		renames = append(renames, AccountModelAliasRename{
			OldModel: oldModel,
			NewModel: newModel,
		})
	}
	return renames
}

func (s *adminServiceImpl) CascadeAccountModelAliasRenames(ctx context.Context, accountID int64, input []AccountModelAliasRenameInput) (*AccountModelAliasRenameCascadeResult, error) {
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if account.Platform != PlatformAntigravity {
		return nil, infraerrors.BadRequest("ACCOUNT_MODEL_ALIAS_RENAME_INVALID_PLATFORM", "account model alias renames are only supported for Antigravity accounts")
	}

	renames := normalizeAccountModelAliasRenames(input)
	if len(renames) == 0 {
		return &AccountModelAliasRenameCascadeResult{}, nil
	}

	groupIDs := normalizeAccountModelAliasRenameGroupIDs(account.GroupIDs)
	if len(groupIDs) == 0 {
		return &AccountModelAliasRenameCascadeResult{}, nil
	}

	result := &AccountModelAliasRenameCascadeResult{}
	for _, repo := range []accountModelAliasRenameCascadeRepository{
		s.channelRepo,
		s.userCustomGroupRepo,
		s.systemCustomGroupRepo,
	} {
		if repo == nil {
			continue
		}
		partial, err := repo.CascadeAccountModelAliasRenames(ctx, accountID, groupIDs, renames)
		if err != nil {
			return nil, err
		}
		mergeAccountModelAliasRenameCascadeResult(result, partial)
	}

	return result, nil
}

func mergeAccountModelAliasRenameCascadeResult(dst, src *AccountModelAliasRenameCascadeResult) {
	if dst == nil || src == nil {
		return
	}
	dst.ChannelPricingUpdated += src.ChannelPricingUpdated
	dst.ChannelMappingsUpdated += src.ChannelMappingsUpdated
	dst.UserCustomRoutesUpdated += src.UserCustomRoutesUpdated
	dst.SystemCustomRoutesUpdated += src.SystemCustomRoutesUpdated
	dst.Skipped = append(dst.Skipped, src.Skipped...)
}

func normalizeAccountModelAliasRenameGroupIDs(groupIDs []int64) []int64 {
	normalized := make([]int64, 0, len(groupIDs))
	seen := make(map[int64]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		if groupID <= 0 {
			continue
		}
		if _, ok := seen[groupID]; ok {
			continue
		}
		seen[groupID] = struct{}{}
		normalized = append(normalized, groupID)
	}
	return normalized
}
