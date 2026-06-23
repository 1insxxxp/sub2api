//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestAPIKeyServiceGetDefaultImageStudioAPIKeySelectsFirstActiveKey(t *testing.T) {
	repo := &imageStudioAPIKeyRepoStub{
		keys: []APIKey{
			{ID: 1, UserID: 7, Status: StatusAPIKeyDisabled},
			{ID: 2, UserID: 7, Status: StatusAPIKeyActive, User: &User{ID: 7}, Group: &Group{ID: 3}},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	key, err := svc.GetDefaultImageStudioAPIKey(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, int64(2), key.ID)
	require.NotNil(t, key.User)
	require.NotNil(t, key.Group)
}

func TestAPIKeyServiceGetDefaultImageStudioAPIKeyReturnsNotFoundWithoutActiveKey(t *testing.T) {
	repo := &imageStudioAPIKeyRepoStub{
		keys: []APIKey{{ID: 1, UserID: 7, Status: StatusAPIKeyDisabled}},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetDefaultImageStudioAPIKey(context.Background(), 7)
	require.ErrorIs(t, err, ErrAPIKeyNotFound)
}

func TestAPIKeyServiceGetImageStudioAPIKeyForGroupSelectsActiveKeyBoundToGroup(t *testing.T) {
	selectedGroupID := int64(9)
	otherGroupID := int64(3)
	repo := &imageStudioAPIKeyRepoStub{
		keys: []APIKey{
			{ID: 1, UserID: 7, Status: StatusAPIKeyActive, GroupID: &otherGroupID, User: &User{ID: 7}, Group: &Group{ID: otherGroupID}},
			{ID: 2, UserID: 7, Status: StatusAPIKeyDisabled, GroupID: &selectedGroupID, User: &User{ID: 7}, Group: &Group{ID: selectedGroupID}},
			{ID: 3, UserID: 7, Status: StatusAPIKeyActive, GroupID: &selectedGroupID, User: &User{ID: 7}, Group: &Group{ID: selectedGroupID}},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	key, err := svc.GetImageStudioAPIKeyForGroup(context.Background(), 7, selectedGroupID)

	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, int64(3), key.ID)
	require.NotNil(t, key.GroupID)
	require.Equal(t, selectedGroupID, *key.GroupID)
}

func TestAPIKeyServiceGetImageStudioAPIKeyForGroupReturnsNotFoundWithoutBoundActiveKey(t *testing.T) {
	selectedGroupID := int64(9)
	otherGroupID := int64(3)
	repo := &imageStudioAPIKeyRepoStub{
		keys: []APIKey{{ID: 1, UserID: 7, Status: StatusAPIKeyActive, GroupID: &otherGroupID}},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetImageStudioAPIKeyForGroup(context.Background(), 7, selectedGroupID)

	require.ErrorIs(t, err, ErrAPIKeyNotFound)
}

func TestAPIKeyServiceGetImageStudioAPIKeyByIDSelectsOwnedActiveKey(t *testing.T) {
	groupID := int64(9)
	repo := &imageStudioAPIKeyRepoStub{
		keys: []APIKey{
			{ID: 3, UserID: 7, Status: StatusAPIKeyActive, GroupID: &groupID, User: &User{ID: 7}, Group: &Group{ID: groupID}},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	key, err := svc.GetImageStudioAPIKeyByID(context.Background(), 7, 3)

	require.NoError(t, err)
	require.NotNil(t, key)
	require.Equal(t, int64(3), key.ID)
	require.NotNil(t, key.GroupID)
	require.Equal(t, groupID, *key.GroupID)
}

func TestAPIKeyServiceGetImageStudioAPIKeyByIDRejectsOtherUsersKey(t *testing.T) {
	groupID := int64(9)
	repo := &imageStudioAPIKeyRepoStub{
		keys: []APIKey{
			{ID: 3, UserID: 8, Status: StatusAPIKeyActive, GroupID: &groupID, User: &User{ID: 8}, Group: &Group{ID: groupID}},
		},
	}
	svc := NewAPIKeyService(repo, nil, nil, nil, nil, nil, nil)

	_, err := svc.GetImageStudioAPIKeyByID(context.Background(), 7, 3)

	require.ErrorIs(t, err, ErrAPIKeyNotFound)
}

type imageStudioAPIKeyRepoStub struct {
	APIKeyRepository
	keys []APIKey
}

func (s *imageStudioAPIKeyRepoStub) GetByID(ctx context.Context, id int64) (*APIKey, error) {
	for i := range s.keys {
		if s.keys[i].ID == id {
			key := s.keys[i]
			return &key, nil
		}
	}
	return nil, ErrAPIKeyNotFound
}

func (s *imageStudioAPIKeyRepoStub) ListByUserID(ctx context.Context, userID int64, params pagination.PaginationParams, filters APIKeyListFilters) ([]APIKey, *pagination.PaginationResult, error) {
	out := make([]APIKey, 0, len(s.keys))
	for _, key := range s.keys {
		if key.UserID != userID {
			continue
		}
		if filters.Status != "" && key.Status != filters.Status {
			continue
		}
		out = append(out, key)
	}
	return out, &pagination.PaginationResult{Total: int64(len(out)), Page: 1, PageSize: params.Limit(), Pages: 1}, nil
}
