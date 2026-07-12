//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"
)

func TestAffiliateService_ResolveTierAwareRateCustomPriority(t *testing.T) {
	repo := &affiliateTierServiceRepoStub{qualifiedCount: 10}
	settings := newAffiliateTierServiceSettingRepo()
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	rate := 50.0
	got, err := svc.ResolveTierAwareRate(context.Background(), &AffiliateSummary{UserID: 1, AffRebateRatePercent: &rate})
	require.NoError(t, err)
	require.Equal(t, 50.0, got)

	for _, dirty := range []float64{math.NaN(), math.Inf(1), -5, 250} {
		dirty := dirty
		got, err = svc.ResolveTierAwareRate(context.Background(), &AffiliateSummary{UserID: 1, AffRebateRatePercent: &dirty})
		require.NoError(t, err)
		require.Equal(t, 12.0, got, "dirty custom values must fall back to automatic silver rate")
	}

	got, err = svc.ResolveTierAwareRate(context.Background(), &AffiliateSummary{UserID: 1})
	require.NoError(t, err)
	require.Equal(t, 12.0, got, "clearing custom rate must restore automatic tier")
}

func TestAffiliateService_TierSnapshotBoundaries(t *testing.T) {
	tests := []struct {
		count     int
		level     AffiliateTier
		rate      float64
		next      int
		remaining int
	}{
		{0, AffiliateTierStandard, 8, 3, 3},
		{2, AffiliateTierStandard, 8, 3, 1},
		{3, AffiliateTierBronze, 10, 10, 7},
		{9, AffiliateTierBronze, 10, 10, 1},
		{10, AffiliateTierSilver, 12, 30, 20},
		{29, AffiliateTierSilver, 12, 30, 1},
		{30, AffiliateTierGold, 15, 0, 0},
	}
	for _, tt := range tests {
		t.Run(strconv.Itoa(tt.count), func(t *testing.T) {
			repo := &affiliateTierServiceRepoStub{qualifiedCount: tt.count}
			svc := NewAffiliateService(repo, NewSettingService(newAffiliateTierServiceSettingRepo(), nil), nil, nil)
			snapshot, err := svc.ResolveAffiliateTierSnapshot(context.Background(), 42)
			require.NoError(t, err)
			require.Equal(t, tt.level, snapshot.Level)
			require.Equal(t, tt.rate, snapshot.AutomaticRatePercent)
			require.Equal(t, tt.count, snapshot.QualifiedInviteeCount)
			require.Equal(t, tt.next, snapshot.NextTierThreshold)
			require.Equal(t, tt.remaining, snapshot.RemainingToNextTier)
			require.Equal(t, 50.0, repo.countThreshold)
		})
	}
}

func TestAffiliateService_TierStrictConfigErrorPropagates(t *testing.T) {
	wantErr := errors.New("settings unavailable")
	settings := newAffiliateTierServiceSettingRepo()
	settings.getMultipleErr = wantErr
	svc := NewAffiliateService(&affiliateTierServiceRepoStub{}, NewSettingService(settings, nil), nil, nil)

	_, err := svc.ResolveAffiliateTierSnapshot(context.Background(), 42)
	require.ErrorIs(t, err, wantErr)
}

func TestAffiliateService_QualificationReconcileMarkerClearsOnlyAfterSuccess(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		settings := newAffiliateTierServiceSettingRepo()
		settings.values[SettingKeyAffiliateTierReconcileRequired] = "true"
		repo := &affiliateTierServiceRepoStub{qualifiedCount: 3}
		settings.beforeSet = func(key, value string) {
			require.Equal(t, SettingKeyAffiliateTierReconcileRequired, key)
			require.Equal(t, "false", value)
			require.True(t, repo.reconcileFinished, "marker may clear only after full reconcile returns")
		}
		svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

		err := svc.ReconcilePendingAffiliateQualifications(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, repo.reconcileCalls)
		require.Equal(t, "false", settings.values[SettingKeyAffiliateTierReconcileRequired])
	})

	t.Run("failure", func(t *testing.T) {
		settings := newAffiliateTierServiceSettingRepo()
		settings.values[SettingKeyAffiliateTierReconcileRequired] = "true"
		wantErr := errors.New("reconcile failed")
		repo := &affiliateTierServiceRepoStub{reconcileErr: wantErr}
		svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

		err := svc.ReconcilePendingAffiliateQualifications(context.Background())
		require.ErrorIs(t, err, wantErr)
		require.Equal(t, "true", settings.values[SettingKeyAffiliateTierReconcileRequired])
	})

	t.Run("marker clear failure", func(t *testing.T) {
		settings := newAffiliateTierServiceSettingRepo()
		settings.values[SettingKeyAffiliateTierReconcileRequired] = "true"
		wantErr := errors.New("setting write failed")
		settings.setErr = wantErr
		repo := &affiliateTierServiceRepoStub{}
		svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

		err := svc.ReconcilePendingAffiliateQualifications(context.Background())
		require.ErrorIs(t, err, wantErr)
		require.Equal(t, 1, repo.reconcileCalls)
		require.Equal(t, "true", settings.values[SettingKeyAffiliateTierReconcileRequired])
	})
}

func TestAffiliateService_QualificationReconcileRejectsTransactionContext(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	settings.values[SettingKeyAffiliateTierReconcileRequired] = "true"
	repo := &affiliateTierServiceRepoStub{}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)
	txCtx := dbent.NewTxContext(context.Background(), &dbent.Tx{})

	err := svc.ReconcilePendingAffiliateQualifications(txCtx)

	require.Error(t, err)
	require.Contains(t, err.Error(), "transaction")
	require.Equal(t, 0, repo.reconcileCallCount())
	require.Equal(t, "true", settings.values[SettingKeyAffiliateTierReconcileRequired])
}

func TestAffiliateService_AccrueKeepsFlatRateWithoutPendingReconcile(t *testing.T) {
	inviterID := int64(42)
	settings := &affiliateTierServiceSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:               "true",
		SettingKeyAffiliateRebateRate:            "15",
		SettingKeyAffiliateTierReconcileRequired: "true",
	}}
	repo := &affiliateTierServiceRepoStub{
		inviteeSummary: &AffiliateSummary{UserID: 7, InviterID: &inviterID},
		inviterSummary: &AffiliateSummary{UserID: inviterID},
	}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	rebate, err := svc.AccrueInviteRebateForOrder(context.Background(), 7, 100, nil)

	require.NoError(t, err)
	require.Equal(t, 15.0, rebate)
	require.Equal(t, 15.0, repo.accruedAmount)
	require.Equal(t, 0, repo.reconcileCallCount(), "legacy accrue must not consume pending tier reconcile")
	require.Equal(t, 0, repo.countCallCount(), "legacy accrue must not resolve an automatic tier")
	require.Equal(t, "true", settings.values[SettingKeyAffiliateTierReconcileRequired])
}

func TestAffiliateService_LegacyRateRemainsFlatCustomCompatible(t *testing.T) {
	settings := &affiliateTierServiceSettingRepo{values: map[string]string{
		SettingKeyAffiliateRebateRate: "15",
	}}
	repo := &affiliateTierServiceRepoStub{qualifiedCount: 30}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	custom := 50.0
	require.Equal(t, 50.0, svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &custom}))
	dirty := math.NaN()
	require.Equal(t, 15.0, svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &dirty}))
	tooHigh := 250.0
	require.Equal(t, AffiliateRebateRateMax, svc.resolveRebateRatePercent(context.Background(), &AffiliateSummary{AffRebateRatePercent: &tooHigh}))
	require.Equal(t, 0, repo.countCallCount())
}

func TestAffiliateDetailJSONDoesNotExposeTierQualificationBeforeTask5(t *testing.T) {
	qualifiedAt := time.Now()
	payload, err := json.Marshal(AffiliateDetail{
		Invitees: []AffiliateInvitee{{
			UserID:                  1,
			QualifyingPaymentAmount: 50,
			QualifiedAt:             &qualifiedAt,
		}},
	})
	require.NoError(t, err)
	require.NotContains(t, string(payload), `"tier"`)
	require.NotContains(t, string(payload), "qualifying_payment_amount")
	require.NotContains(t, string(payload), "qualified_at")

	qualificationPayload, err := json.Marshal(AffiliateQualification{
		InviteeUserID:           1,
		QualifyingPaymentAmount: 50,
		QualifiedAt:             &qualifiedAt,
	})
	require.NoError(t, err)
	require.Equal(t, `{}`, string(qualificationPayload))
}

func TestAffiliateService_GetDetailDoesNotReadStrictTierConfig(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	settings.getMultipleErr = errors.New("tier settings unavailable")
	repo := &affiliateTierServiceRepoStub{}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	detail, err := svc.GetAffiliateDetail(context.Background(), 42)

	require.NoError(t, err)
	require.NotNil(t, detail)
	require.Equal(t, AffiliateRebateRateDefault, detail.EffectiveRebateRatePercent)
	require.Equal(t, 0, repo.countCallCount())
	require.Equal(t, 0, repo.reconcileCallCount())
}

func TestAffiliateService_QualificationReconcileMarkerConcurrentReadsSingleRun(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	settings.values[SettingKeyAffiliateTierReconcileRequired] = "true"
	repo := &affiliateTierServiceRepoStub{reconcileStarted: make(chan struct{}), releaseReconcile: make(chan struct{})}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	const readers = 8
	errs := make(chan error, readers)
	for i := 0; i < readers; i++ {
		go func() {
			err := svc.ReconcilePendingAffiliateQualifications(context.Background())
			errs <- err
		}()
	}
	<-repo.reconcileStarted
	close(repo.releaseReconcile)
	for i := 0; i < readers; i++ {
		require.NoError(t, <-errs)
	}
	require.Equal(t, 1, repo.reconcileCallCount())
}

type affiliateTierServiceRepoStub struct {
	AffiliateRepository
	mu                sync.Mutex
	qualifiedCount    int
	countThreshold    float64
	reconcileCalls    int
	reconcileErr      error
	reconcileStarted  chan struct{}
	releaseReconcile  chan struct{}
	reconcileFinished bool
	countCalls        int
	inviteeSummary    *AffiliateSummary
	inviterSummary    *AffiliateSummary
	accruedAmount     float64
}

func (r *affiliateTierServiceRepoStub) CountQualifiedInvitees(_ context.Context, _ int64, threshold float64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.countCalls++
	r.countThreshold = threshold
	return r.qualifiedCount, nil
}

func (r *affiliateTierServiceRepoStub) ReconcileAllAffiliateQualifications(context.Context, float64, int) error {
	r.mu.Lock()
	r.reconcileCalls++
	call := r.reconcileCalls
	r.mu.Unlock()
	if call == 1 && r.reconcileStarted != nil {
		close(r.reconcileStarted)
		<-r.releaseReconcile
	}
	r.mu.Lock()
	r.reconcileFinished = true
	r.mu.Unlock()
	return r.reconcileErr
}

func (r *affiliateTierServiceRepoStub) EnsureUserAffiliate(_ context.Context, userID int64) (*AffiliateSummary, error) {
	if r.inviteeSummary != nil && r.inviteeSummary.UserID == userID {
		return r.inviteeSummary, nil
	}
	if r.inviterSummary != nil && r.inviterSummary.UserID == userID {
		return r.inviterSummary, nil
	}
	return &AffiliateSummary{UserID: userID}, nil
}

func (r *affiliateTierServiceRepoStub) AccrueQuota(_ context.Context, _, _ int64, amount float64, _ int, _ *int64) (bool, error) {
	r.accruedAmount = amount
	return true, nil
}

func (r *affiliateTierServiceRepoStub) ThawFrozenQuota(context.Context, int64) (float64, error) {
	return 0, nil
}

func (r *affiliateTierServiceRepoStub) ListInvitees(context.Context, int64, int) ([]AffiliateInvitee, error) {
	return nil, nil
}

func (r *affiliateTierServiceRepoStub) reconcileCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.reconcileCalls
}

func (r *affiliateTierServiceRepoStub) countCallCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.countCalls
}

type affiliateTierServiceSettingRepo struct {
	SettingRepository
	mu             sync.Mutex
	values         map[string]string
	getMultipleErr error
	setErr         error
	beforeSet      func(key, value string)
}

func newAffiliateTierServiceSettingRepo() *affiliateTierServiceSettingRepo {
	return &affiliateTierServiceSettingRepo{values: map[string]string{
		SettingKeyAffiliateRebateRate:          "8",
		SettingKeyAffiliateQualificationAmount: "50",
		SettingKeyAffiliateBronzeInvitees:      "3",
		SettingKeyAffiliateBronzeRate:          "10",
		SettingKeyAffiliateSilverInvitees:      "10",
		SettingKeyAffiliateSilverRate:          "12",
		SettingKeyAffiliateGoldInvitees:        "30",
		SettingKeyAffiliateGoldRate:            "15",
	}}
}

func (r *affiliateTierServiceSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.getMultipleErr != nil {
		return nil, r.getMultipleErr
	}
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *affiliateTierServiceSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *affiliateTierServiceSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.setErr != nil {
		return r.setErr
	}
	if r.beforeSet != nil {
		r.beforeSet(key, value)
	}
	r.values[key] = value
	return nil
}

// TestIsEnabled_NilSettingServiceReturnsDefault verifies that IsEnabled
// safely handles a nil settingService dependency by returning the default
// (off). This protects callers from nil-pointer crashes in misconfigured
// environments.
func TestIsEnabled_NilSettingServiceReturnsDefault(t *testing.T) {
	t.Parallel()
	svc := &AffiliateService{}
	require.False(t, svc.IsEnabled(context.Background()))
	require.Equal(t, AffiliateEnabledDefault, svc.IsEnabled(context.Background()))
}

// TestValidateExclusiveRate_BoundaryAndInvalid covers the validator used by
// admin-facing rate setters: nil is always valid (clear), in-range values
// are accepted, NaN/Inf and out-of-range values produce a typed BadRequest.
func TestValidateExclusiveRate_BoundaryAndInvalid(t *testing.T) {
	t.Parallel()
	require.NoError(t, validateExclusiveRate(nil))

	for _, v := range []float64{0, 0.01, 50, 99.99, 100} {
		v := v
		require.NoError(t, validateExclusiveRate(&v), "value %v should be valid", v)
	}

	for _, v := range []float64{-0.01, 100.01, -100, 200} {
		v := v
		require.Error(t, validateExclusiveRate(&v), "value %v should be rejected", v)
	}

	nan := math.NaN()
	require.Error(t, validateExclusiveRate(&nan))
	posInf := math.Inf(1)
	require.Error(t, validateExclusiveRate(&posInf))
	negInf := math.Inf(-1)
	require.Error(t, validateExclusiveRate(&negInf))
}

func TestMaskEmail(t *testing.T) {
	t.Parallel()
	require.Equal(t, "a***@g***.com", maskEmail("alice@gmail.com"))
	require.Equal(t, "x***@d***", maskEmail("x@domain"))
	require.Equal(t, "", maskEmail(""))
}

func TestIsValidAffiliateCodeFormat(t *testing.T) {
	t.Parallel()

	// 邀请码格式校验同时服务于：
	// 1) 系统自动生成的 12 位随机码（A-Z 去 I/O，2-9 去 0/1）
	// 2) 管理员设置的自定义专属码（如 "VIP2026"、"NEW_USER-1"）
	// 因此校验放宽到 [A-Z0-9_-]{4,32}（要求调用方先 ToUpper）。
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"valid canonical 12-char", "ABCDEFGHJKLM", true},
		{"valid all digits 2-9", "234567892345", true},
		{"valid mixed", "A2B3C4D5E6F7", true},
		{"valid admin custom short", "VIP1", true},
		{"valid admin custom with hyphen", "NEW-USER", true},
		{"valid admin custom with underscore", "VIP_2026", true},
		{"valid 32-char max", "ABCDEFGHIJKLMNOPQRSTUVWXYZ012345", true},
		// Previously-excluded chars (I/O/0/1) are now allowed since admins may use them.
		{"letter I now allowed", "IBCDEFGHJKLM", true},
		{"letter O now allowed", "OBCDEFGHJKLM", true},
		{"digit 0 now allowed", "0BCDEFGHJKLM", true},
		{"digit 1 now allowed", "1BCDEFGHJKLM", true},
		{"too short (3 chars)", "ABC", false},
		{"too long (33 chars)", "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456", false},
		{"lowercase rejected (caller must ToUpper first)", "abcdefghjklm", false},
		{"empty", "", false},
		{"utf8 non-ascii", "ÄÄÄÄÄÄ", false}, // bytes out of charset
		{"ascii punctuation .", "ABCDEFGHJK.M", false},
		{"whitespace", "ABCDEFGHJK M", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, isValidAffiliateCodeFormat(tc.in))
		})
	}
}
