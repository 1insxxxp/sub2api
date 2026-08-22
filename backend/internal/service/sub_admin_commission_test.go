package service

import (
	"context"
	"math"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type subAdminCommissionRepoStub struct {
	SubAdminCommissionRepository

	replaceInput       ReplaceSubAdminCommissionGrantsInput
	replaceGrantedDate string
	logsErr            error
}

func (r *subAdminCommissionRepoStub) ReplaceGrants(ctx context.Context, input ReplaceSubAdminCommissionGrantsInput, grantedDate string) ([]SubAdminCommissionGrant, error) {
	r.replaceInput = input
	r.replaceGrantedDate = grantedDate
	return []SubAdminCommissionGrant{
		{SubAdminID: input.SubAdminID, GroupID: input.GroupIDs[0], GrantedDate: grantedDate, Enabled: true},
	}, nil
}

func (r *subAdminCommissionRepoStub) ListDayGroupLogs(ctx context.Context, subAdminID, groupID int64, date string, params pagination.PaginationParams) ([]SubAdminCommissionUsageLog, pagination.PaginationResult, error) {
	return nil, pagination.PaginationResult{}, r.logsErr
}

type subAdminCommissionUserRepoStub struct {
	UserRepository

	user *User
	err  error
}

func (r *subAdminCommissionUserRepoStub) GetByID(ctx context.Context, id int64) (*User, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.user, nil
}

type subAdminCommissionSettingRepoStub struct {
	SettingRepository

	values map[string]string
	sets   map[string]string
}

func (r *subAdminCommissionSettingRepoStub) GetValue(ctx context.Context, key string) (string, error) {
	if value, ok := r.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (r *subAdminCommissionSettingRepoStub) Set(ctx context.Context, key, value string) error {
	if r.sets == nil {
		r.sets = make(map[string]string)
	}
	r.sets[key] = value
	return nil
}

func TestSubAdminCommissionSetSettingsRejectsOutOfRangeRates(t *testing.T) {
	svc := NewSubAdminCommissionService(nil, nil, NewSettingService(&subAdminCommissionSettingRepoStub{}, nil))

	_, err := svc.SetSettings(context.Background(), -0.01)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))

	_, err = svc.SetSettings(context.Background(), 1.01)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))

	_, err = svc.SetSettings(context.Background(), math.NaN())
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))

	_, err = svc.SetSettings(context.Background(), math.Inf(1))
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestSubAdminCommissionSetSettingsPersistsRate(t *testing.T) {
	repo := &subAdminCommissionSettingRepoStub{}
	svc := NewSubAdminCommissionService(nil, nil, NewSettingService(repo, nil))

	rate, err := svc.SetSettings(context.Background(), 0.075)

	require.NoError(t, err)
	require.Equal(t, 0.075, rate)
	require.Equal(t, "0.075", repo.sets[SettingKeySubAdminCommissionRate])
}

func TestSubAdminCommissionReplaceGrantsRejectsNonSubAdmin(t *testing.T) {
	svc := NewSubAdminCommissionService(
		&subAdminCommissionRepoStub{},
		&subAdminCommissionUserRepoStub{user: &User{ID: 12, Role: RoleUser}},
		NewSettingService(&subAdminCommissionSettingRepoStub{}, nil),
	)

	_, err := svc.ReplaceGrants(context.Background(), ReplaceSubAdminCommissionGrantsInput{
		SubAdminID: 12,
		GroupIDs:   []int64{101},
		OperatorID: 1,
		Now:        time.Date(2026, 8, 22, 10, 30, 0, 0, time.UTC),
	})

	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
}

func TestSubAdminCommissionReplaceGrantsUsesGroupUsageDate(t *testing.T) {
	repo := &subAdminCommissionRepoStub{}
	now := time.Date(2026, 8, 22, 2, 30, 0, 0, time.UTC)
	svc := NewSubAdminCommissionService(
		repo,
		&subAdminCommissionUserRepoStub{user: &User{ID: 12, Role: RoleSubAdmin}},
		NewSettingService(&subAdminCommissionSettingRepoStub{}, nil),
	)

	grants, err := svc.ReplaceGrants(context.Background(), ReplaceSubAdminCommissionGrantsInput{
		SubAdminID: 12,
		GroupIDs:   []int64{101},
		OperatorID: 1,
		Now:        now,
	})

	require.NoError(t, err)
	require.Len(t, grants, 1)
	require.Equal(t, GroupUsageDate(now), repo.replaceGrantedDate)
	require.Equal(t, []int64{101}, repo.replaceInput.GroupIDs)
}

func TestSubAdminCommissionDayGroupLogsPropagatesForbiddenGrant(t *testing.T) {
	svc := NewSubAdminCommissionService(
		&subAdminCommissionRepoStub{logsErr: ErrSubAdminCommissionForbidden},
		nil,
		NewSettingService(&subAdminCommissionSettingRepoStub{}, nil),
	)

	_, _, err := svc.ListDayGroupLogs(context.Background(), 12, 101, "2026-08-22", pagination.PaginationParams{Page: 1, PageSize: 20})

	require.ErrorIs(t, err, ErrSubAdminCommissionForbidden)
	require.True(t, infraerrors.IsForbidden(err))
}
