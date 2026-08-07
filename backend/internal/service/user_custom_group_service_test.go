package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type customGroupValidationUserRepoStub struct {
	UserRepository
	user *User
}

func (s customGroupValidationUserRepoStub) GetByID(context.Context, int64) (*User, error) {
	if s.user == nil {
		return nil, errors.New("user not found")
	}
	return s.user, nil
}

type customGroupValidationGroupRepoStub struct {
	GroupRepository
	groups map[int64]*Group
}

func (s customGroupValidationGroupRepoStub) GetByIDLite(_ context.Context, id int64) (*Group, error) {
	group, ok := s.groups[id]
	if !ok {
		return nil, ErrGroupNotFound
	}
	return group, nil
}

func newCustomGroupValidationService() *UserCustomGroupService {
	return &UserCustomGroupService{
		userRepo: customGroupValidationUserRepoStub{user: &User{ID: 9, Status: StatusActive}},
		groupRepo: customGroupValidationGroupRepoStub{groups: map[int64]*Group{
			10: {ID: 10, Status: StatusActive, Platform: PlatformAnthropic},
			20: {ID: 20, Status: StatusActive, Platform: PlatformAnthropic},
		}},
	}
}

func TestUserCustomGroupValidateModelsAllowsAliasesForSameRealModelAcrossGroups(t *testing.T) {
	svc := newCustomGroupValidationService()
	models := []UserCustomGroupModelInput{
		{PublicModel: "  claude-opus-4-6-balance  ", SourceGroupID: 10, SourceModel: "claude-opus-4-6"},
		{PublicModel: "claude-opus-4-6-discount", SourceGroupID: 20, SourceModel: "claude-opus-4-6"},
	}

	err := svc.validateModels(context.Background(), 9, models)

	require.NoError(t, err)
	require.Equal(t, "claude-opus-4-6-balance", models[0].PublicModel)
}

func TestUserCustomGroupValidateModelsRejectsCaseInsensitiveCallNameConflict(t *testing.T) {
	svc := newCustomGroupValidationService()
	models := []UserCustomGroupModelInput{
		{PublicModel: "Claude-Balance", SourceGroupID: 10, SourceModel: "claude-opus-4-6"},
		{PublicModel: "claude-balance", SourceGroupID: 20, SourceModel: "claude-opus-4-6"},
	}

	require.ErrorIs(t, svc.validateModels(context.Background(), 9, models), ErrUserCustomGroupInvalidModel)
}

func TestUserCustomGroupValidateModelsRejectsDuplicateSourceMapping(t *testing.T) {
	svc := newCustomGroupValidationService()
	models := []UserCustomGroupModelInput{
		{PublicModel: "claude-primary", SourceGroupID: 10, SourceModel: "claude-opus-4-6"},
		{PublicModel: "claude-secondary", SourceGroupID: 10, SourceModel: "CLAUDE-OPUS-4-6"},
	}

	require.ErrorIs(t, svc.validateModels(context.Background(), 9, models), ErrUserCustomGroupInvalidModel)
}
