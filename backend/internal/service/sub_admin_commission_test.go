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

func TestSubAdminCommissionReplaceGrantsUsesGroupUsageDate(t *testing.T) {
	repo := &subAdminCommissionRepoStub{}
	now := time.Date(2026, 8, 22, 2, 30, 0, 0, time.UTC)
	svc := NewSubAdminCommissionService(
		repo,
		nil,
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

type monthlyReportCommissionRepoStub struct {
	SubAdminCommissionRepository
	subAdminID int64
	month      string
	rate       float64
	now        time.Time
	report     SubAdminCommissionMonthlyReport
	err        error
}

func (r *monthlyReportCommissionRepoStub) ListMonthlyReport(ctx context.Context, id int64, month string, rate float64, now time.Time) (SubAdminCommissionMonthlyReport, error) {
	r.subAdminID = id
	r.month = month
	r.rate = rate
	r.now = now
	return r.report, r.err
}

func TestSubAdminCommissionMonthlyReportUsesConfiguredRateAndPropagatesReport(t *testing.T) {
	now := time.Date(2026, 8, 23, 10, 0, 0, 0, time.UTC)
	report := SubAdminCommissionMonthlyReport{Days: []SubAdminCommissionCalendarDay{{Date: "2026-08-23", ActualCost: 12.5, CommissionAmount: 1.0}}, Groups: []SubAdminCommissionMonthlyGroup{{GroupID: 9, GroupName: "Group"}}}
	repo := &monthlyReportCommissionRepoStub{report: report}
	svc := NewSubAdminCommissionService(repo, nil, NewSettingService(&subAdminCommissionSettingRepoStub{values: map[string]string{SettingKeySubAdminCommissionRate: "0.08"}}, nil))
	got, err := svc.ListMonthlyReport(context.Background(), 17, "2026-08", now)
	require.NoError(t, err)
	require.Equal(t, report, got)
	require.Equal(t, int64(17), repo.subAdminID)
	require.Equal(t, "2026-08", repo.month)
	require.Equal(t, 0.08, repo.rate)
	require.Equal(t, now, repo.now)
	repo.err = ErrSubAdminCommissionForbidden
	_, err = svc.ListMonthlyReport(context.Background(), 17, "2026-08", time.Time{})
	require.ErrorIs(t, err, repo.err)
	require.False(t, repo.now.IsZero())
	_, err = (*SubAdminCommissionService)(nil).ListMonthlyReport(context.Background(), 17, "2026-08", now)
	require.ErrorIs(t, err, ErrSubAdminCommissionRepoMissing)
}
