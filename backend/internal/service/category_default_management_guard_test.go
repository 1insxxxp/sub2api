//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type categoryDefaultManagementGroupRepo struct{ GroupRepository }

func (categoryDefaultManagementGroupRepo) GetByID(context.Context, int64) (*Group, error) {
	return &Group{ID: 91, Name: "Gemini 默认分组", Platform: PlatformComposite, Status: StatusActive, SubscriptionType: SubscriptionTypeStandard, IsCategoryDefault: true, Hydrated: true}, nil
}
func (r categoryDefaultManagementGroupRepo) GetByIDLite(ctx context.Context, id int64) (*Group, error) {
	return r.GetByID(ctx, id)
}
func (categoryDefaultManagementGroupRepo) Update(context.Context, *Group) error { return nil }
func (categoryDefaultManagementGroupRepo) DeleteCascade(context.Context, int64) ([]int64, error) {
	return nil, nil
}

func TestCategoryDefaultEntryCannotBeMutatedThroughOrdinaryGroupManagement(t *testing.T) {
	svc := &adminServiceImpl{groupRepo: categoryDefaultManagementGroupRepo{}}
	ctx := context.Background()
	for name, operation := range map[string]func() error{
		"update":           func() error { _, err := svc.UpdateGroup(ctx, 91, &UpdateGroupInput{Name: "bypass"}); return err },
		"delete":           func() error { return svc.DeleteGroup(ctx, 91) },
		"account-binding":  func() error { return svc.ValidateAccountGroupBindings(ctx, []int64{91}) },
		"composite-routes": func() error { return svc.requireCompositeGroup(ctx, 91) },
		"duplicate":        func() error { _, err := svc.DuplicateGroup(ctx, 91, "admin-1", "copy-default"); return err },
	} {
		t.Run(name, func(t *testing.T) {
			err := operation()
			require.Error(t, err)
			require.Equal(t, "CATEGORY_DEFAULT_MANAGED_ONLY", infraerrors.Reason(err))
		})
	}
}
