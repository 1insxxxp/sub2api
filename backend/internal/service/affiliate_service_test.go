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
		repo := &affiliateTierServiceRepoStub{qualifiedCount: 3, reconcileRequired: true, generation: 1}
		svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

		err := svc.ReconcilePendingAffiliateQualifications(context.Background())
		require.NoError(t, err)
		require.Equal(t, 1, repo.reconcileCalls)
		snapshot, snapshotErr := repo.ReadReconcilePendingSnapshot(context.Background())
		require.NoError(t, snapshotErr)
		require.False(t, snapshot.Required)
		require.Equal(t, int64(1), snapshot.Generation)
		require.True(t, repo.reconcileFinished)
	})

	t.Run("failure", func(t *testing.T) {
		settings := newAffiliateTierServiceSettingRepo()
		wantErr := errors.New("reconcile failed")
		repo := &affiliateTierServiceRepoStub{reconcileErr: wantErr, reconcileRequired: true, generation: 1}
		svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

		err := svc.ReconcilePendingAffiliateQualifications(context.Background())
		require.ErrorIs(t, err, wantErr)
		snapshot, snapshotErr := repo.ReadReconcilePendingSnapshot(context.Background())
		require.NoError(t, snapshotErr)
		require.True(t, snapshot.Required)
		require.Equal(t, int64(2), snapshot.Generation)
	})

	t.Run("marker clear failure", func(t *testing.T) {
		settings := newAffiliateTierServiceSettingRepo()
		wantErr := errors.New("generation clear failed")
		repo := &affiliateTierServiceRepoStub{reconcileRequired: true, generation: 1, clearReconcileErr: wantErr}
		svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

		err := svc.ReconcilePendingAffiliateQualifications(context.Background())
		require.ErrorIs(t, err, wantErr)
		require.Equal(t, 1, repo.reconcileCalls)
		require.True(t, repo.reconcileRequired)
		require.Equal(t, int64(2), repo.generation)
	})
}

func TestAffiliateService_QualificationReconcileDrainsDirtyBeforeGlobalMarker(t *testing.T) {
	event := AffiliateQualificationDirtyEvent{
		OrderID:     "42",
		UserID:      7,
		OrderStatus: OrderStatusCompleted,
		EventType:   "payment_completed",
		Detail:      `{"userID":7,"orderStatus":"COMPLETED","eventType":"payment_completed"}`,
	}
	repo := &affiliateTierServiceRepoStub{dirtyEvents: []AffiliateQualificationDirtyEvent{event}}
	svc := NewAffiliateService(repo, NewSettingService(newAffiliateTierServiceSettingRepo(), nil), nil, nil)

	err := svc.ReconcilePendingAffiliateQualifications(context.Background())

	require.NoError(t, err)
	require.Empty(t, repo.dirtyEvents)
	require.Equal(t, []AffiliateQualificationDirtyEvent{event}, repo.deletedDirtyEvents)
	require.False(t, repo.reconcileRequired)
	require.Zero(t, repo.generation)
}

func TestAffiliateService_QualificationReconcileRetainsFailedDirtyEvent(t *testing.T) {
	event := AffiliateQualificationDirtyEvent{
		OrderID:     "43",
		UserID:      8,
		OrderStatus: OrderStatusRefunded,
		EventType:   "refund_completed",
		Detail:      `{"userID":8,"orderStatus":"REFUNDED","eventType":"refund_completed"}`,
	}
	repo := &affiliateTierServiceRepoStub{
		dirtyEvents:         []AffiliateQualificationDirtyEvent{event},
		reconcileInviteeErr: errors.New("qualification unavailable"),
	}
	svc := NewAffiliateService(repo, NewSettingService(newAffiliateTierServiceSettingRepo(), nil), nil, nil)

	err := svc.ReconcilePendingAffiliateQualifications(context.Background())

	require.ErrorContains(t, err, "qualification unavailable")
	require.Equal(t, []AffiliateQualificationDirtyEvent{event}, repo.dirtyEvents)
	require.Empty(t, repo.deletedDirtyEvents)
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(1), repo.generation)
}

func TestAffiliateService_DirtyEventSuccessDoesNotClearGlobalMarker(t *testing.T) {
	event := AffiliateQualificationDirtyEvent{
		OrderID:     "44",
		UserID:      9,
		OrderStatus: OrderStatusPartiallyRefunded,
		EventType:   "refund_completed",
		Detail:      `{"userID":9,"orderStatus":"PARTIALLY_REFUNDED","eventType":"refund_completed"}`,
	}
	repo := &affiliateTierServiceRepoStub{
		dirtyEvents:       []AffiliateQualificationDirtyEvent{event},
		reconcileRequired: true,
		generation:        10,
	}
	svc := NewAffiliateService(repo, NewSettingService(newAffiliateTierServiceSettingRepo(), nil), nil, nil)

	err := svc.ReconcileAffiliateQualificationDirtyEvent(context.Background(), event)

	require.NoError(t, err)
	require.Empty(t, repo.dirtyEvents)
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(10), repo.generation)
}

func TestAffiliateService_QualificationReconcileRejectsTransactionContext(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	repo := &affiliateTierServiceRepoStub{}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)
	txCtx := dbent.NewTxContext(context.Background(), &dbent.Tx{})

	err := svc.ReconcilePendingAffiliateQualifications(txCtx)

	require.Error(t, err)
	require.Contains(t, err.Error(), "transaction")
	require.Equal(t, 0, repo.reconcileCallCount())
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(1), repo.generation)
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

func TestAffiliateService_TierAwareAccrueUsesResolvedTierRate(t *testing.T) {
	inviterID := int64(42)
	settings := &affiliateTierServiceSettingRepo{values: map[string]string{
		SettingKeyAffiliateEnabled:             "true",
		SettingKeyAffiliateQualificationAmount: "50",
		SettingKeyAffiliateBronzeInvitees:      "3",
		SettingKeyAffiliateBronzeRate:          "10",
		SettingKeyAffiliateSilverInvitees:      "10",
		SettingKeyAffiliateSilverRate:          "12",
		SettingKeyAffiliateGoldInvitees:        "30",
		SettingKeyAffiliateGoldRate:            "15",
	}}
	repo := &affiliateTierServiceRepoStub{
		qualifiedCount: 10,
		inviteeSummary: &AffiliateSummary{UserID: 7, InviterID: &inviterID},
		inviterSummary: &AffiliateSummary{UserID: inviterID},
	}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	rebate, err := svc.AccrueTierAwareInviteRebateForOrder(context.Background(), 7, 100, nil)

	require.NoError(t, err)
	require.Equal(t, 12.0, rebate)
	require.Equal(t, 12.0, repo.accruedAmount)
	require.Equal(t, 1, repo.countCallCount())
}

func TestAffiliateService_ReconcileInviteeFailureKeepsRecoveryMarker(t *testing.T) {
	wantErr := errors.New("invitee reconcile failed")
	settings := newAffiliateTierServiceSettingRepo()
	repo := &affiliateTierServiceRepoStub{reconcileInviteeErr: wantErr}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	err := svc.ReconcileInviteeQualification(context.Background(), 7)

	require.ErrorIs(t, err, wantErr)
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(1), repo.generation)
}

func TestAffiliateService_StrictTierFailureSetsRecoveryMarker(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	settings.getMultipleErr = errors.New("tier settings unavailable")
	repo := &affiliateTierServiceRepoStub{}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	err := svc.ReconcileInviteeQualification(context.Background(), 7)

	require.Error(t, err)
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(1), repo.generation)
}

func TestAffiliateService_ReconcilePendingBusyReturnsStableError(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	repo := &affiliateTierServiceRepoStub{advisoryLockHeld: true, reconcileRequired: true, generation: 1}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	err := svc.ReconcilePendingAffiliateQualifications(context.Background())

	require.ErrorIs(t, err, ErrAffiliateQualificationReconcileBusy)
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(1), repo.generation)
}

func TestAffiliateService_ReconcilePendingDoesNotClearConcurrentGeneration(t *testing.T) {
	repo := &affiliateTierServiceRepoStub{reconcileRequired: true, generation: 10}
	repo.duringReconcile = func() {
		repo.mu.Lock()
		repo.generation++
		repo.reconcileRequired = true
		repo.mu.Unlock()
	}
	svc := NewAffiliateService(repo, NewSettingService(newAffiliateTierServiceSettingRepo(), nil), nil, nil)

	err := svc.ReconcilePendingAffiliateQualifications(context.Background())

	require.ErrorIs(t, err, ErrAffiliateQualificationReconcileStale)
	snapshot, snapshotErr := repo.ReadReconcilePendingSnapshot(context.Background())
	require.NoError(t, snapshotErr)
	require.True(t, snapshot.Required)
	require.Equal(t, int64(11), snapshot.Generation)
}

func TestAffiliateService_PendingMarkerReadFailureSetsRecoveryMarker(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	repo := &affiliateTierServiceRepoStub{readSnapshotErr: errors.New("snapshot read failed")}
	svc := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	err := svc.ReconcilePendingAffiliateQualifications(context.Background())

	require.Error(t, err)
	require.True(t, repo.reconcileRequired)
	require.Equal(t, int64(1), repo.generation)
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
	repo := &affiliateTierServiceRepoStub{reconcileStarted: make(chan struct{}), releaseReconcile: make(chan struct{}), reconcileRequired: true, generation: 1}
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
		err := <-errs
		require.True(t, err == nil || errors.Is(err, ErrAffiliateQualificationReconcileBusy))
	}
	require.Equal(t, 1, repo.reconcileCallCount())
}

func TestAffiliateService_QualificationReconcileDatabaseLockCoordinatesInstances(t *testing.T) {
	settings := newAffiliateTierServiceSettingRepo()
	repo := &affiliateTierServiceRepoStub{reconcileStarted: make(chan struct{}), releaseReconcile: make(chan struct{}), reconcileRequired: true, generation: 1}
	first := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)
	second := NewAffiliateService(repo, NewSettingService(settings, nil), nil, nil)

	firstErr := make(chan error, 1)
	go func() { firstErr <- first.ReconcilePendingAffiliateQualifications(context.Background()) }()
	<-repo.reconcileStarted

	secondErr := second.ReconcilePendingAffiliateQualifications(context.Background())
	callsWhileLocked := repo.reconcileCallCount()
	snapshotWhileLocked, snapshotErr := repo.ReadReconcilePendingSnapshot(context.Background())

	require.ErrorIs(t, secondErr, ErrAffiliateQualificationReconcileBusy)
	require.NoError(t, snapshotErr)
	require.True(t, snapshotWhileLocked.Required, "lock loser must not clear marker")
	require.Equal(t, 1, callsWhileLocked)
	close(repo.releaseReconcile)
	require.NoError(t, <-firstErr)
	finalSnapshot, snapshotErr := repo.ReadReconcilePendingSnapshot(context.Background())
	require.NoError(t, snapshotErr)
	require.False(t, finalSnapshot.Required)
}

type affiliateTierServiceRepoStub struct {
	AffiliateRepository
	mu                  sync.Mutex
	qualifiedCount      int
	countThreshold      float64
	reconcileCalls      int
	reconcileErr        error
	reconcileInviteeErr error
	reconcileStarted    chan struct{}
	releaseReconcile    chan struct{}
	reconcileFinished   bool
	countCalls          int
	inviteeSummary      *AffiliateSummary
	inviterSummary      *AffiliateSummary
	accruedAmount       float64
	advisoryLockHeld    bool
	reconcileRequired   bool
	generation          int64
	duringReconcile     func()
	markReconcileErr    error
	readSnapshotErr     error
	clearReconcileErr   error
	dirtyEvents         []AffiliateQualificationDirtyEvent
	deletedDirtyEvents  []AffiliateQualificationDirtyEvent
}

func (r *affiliateTierServiceRepoStub) CountQualifiedInvitees(_ context.Context, _ int64, threshold float64) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.countCalls++
	r.countThreshold = threshold
	return r.qualifiedCount, nil
}

func (r *affiliateTierServiceRepoStub) ReconcileInviteeQualification(context.Context, int64, float64) (*AffiliateQualification, error) {
	return nil, r.reconcileInviteeErr
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
	duringReconcile := r.duringReconcile
	r.mu.Unlock()
	if duringReconcile != nil {
		duringReconcile()
	}
	return r.reconcileErr
}

func (r *affiliateTierServiceRepoStub) TryWithAffiliateQualificationReconcileLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	r.mu.Lock()
	if r.advisoryLockHeld {
		r.mu.Unlock()
		return false, nil
	}
	r.advisoryLockHeld = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		r.advisoryLockHeld = false
		r.mu.Unlock()
	}()
	return true, fn(ctx)
}

func (r *affiliateTierServiceRepoStub) MarkReconcileRequired(context.Context) (AffiliateReconcileToken, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.markReconcileErr != nil {
		return AffiliateReconcileToken{}, r.markReconcileErr
	}
	wasPending := r.reconcileRequired
	r.generation++
	if r.generation <= 0 {
		r.generation = 1
	}
	r.reconcileRequired = true
	return AffiliateReconcileToken{Generation: r.generation, WasPendingBefore: wasPending}, nil
}

func (r *affiliateTierServiceRepoStub) ReadReconcilePendingSnapshot(context.Context) (AffiliateReconcilePendingSnapshot, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.readSnapshotErr != nil {
		err := r.readSnapshotErr
		r.readSnapshotErr = nil
		return AffiliateReconcilePendingSnapshot{}, err
	}
	return AffiliateReconcilePendingSnapshot{Required: r.reconcileRequired, Generation: r.generation}, nil
}

func (r *affiliateTierServiceRepoStub) ClearReconcileRequiredIfGeneration(_ context.Context, expected int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.clearReconcileErr != nil {
		return false, r.clearReconcileErr
	}
	if !r.reconcileRequired || r.generation != expected {
		return false, nil
	}
	r.reconcileRequired = false
	return true, nil
}

func (r *affiliateTierServiceRepoStub) ListAffiliateQualificationDirtyEvents(context.Context, int) ([]AffiliateQualificationDirtyEvent, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]AffiliateQualificationDirtyEvent(nil), r.dirtyEvents...), nil
}

func (r *affiliateTierServiceRepoStub) DeleteAffiliateQualificationDirtyEvent(_ context.Context, event AffiliateQualificationDirtyEvent) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, pending := range r.dirtyEvents {
		if pending.OrderID == event.OrderID && pending.Detail == event.Detail {
			r.dirtyEvents = append(r.dirtyEvents[:i], r.dirtyEvents[i+1:]...)
			r.deletedDirtyEvents = append(r.deletedDirtyEvents, event)
			return true, nil
		}
	}
	return false, nil
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
	getValueErr    error
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
	if r.getValueErr != nil {
		return "", r.getValueErr
	}
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
