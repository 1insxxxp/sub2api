//go:build integration

package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func querySingleFloat(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) float64 {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value float64
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func querySingleInt(t *testing.T, ctx context.Context, client *dbent.Client, query string, args ...any) int {
	t.Helper()
	rows, err := client.QueryContext(ctx, query, args...)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()

	require.True(t, rows.Next(), "expected one row")
	var value int
	require.NoError(t, rows.Scan(&value))
	require.NoError(t, rows.Err())
	return value
}

func TestAffiliateRepository_TransferQuotaToBalance_UsesClaimedQuotaBeforeClear(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-transfer-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      5.5,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, $3, $3, NOW(), NOW())`, u.ID, affCode, 12.34)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.NoError(t, err)
	require.InDelta(t, 12.34, transferred, 1e-9)
	require.InDelta(t, 17.84, balance, 1e-9)

	affQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", u.ID)
	require.InDelta(t, 0.0, affQuota, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 17.84, persistedBalance, 1e-9)

	ledgerCount := querySingleInt(t, txCtx, client,
		"SELECT COUNT(*) FROM user_affiliate_ledger WHERE user_id = $1 AND action = 'transfer'", u.ID)
	require.Equal(t, 1, ledgerCount)

	rows, err := client.QueryContext(txCtx, `
SELECT amount::double precision,
       balance_after::double precision,
       aff_quota_after::double precision,
       aff_frozen_quota_after::double precision,
       aff_history_quota_after::double precision
FROM user_affiliate_ledger
WHERE user_id = $1 AND action = 'transfer'
LIMIT 1`, u.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next(), "expected transfer ledger")
	var amount, balanceAfter, quotaAfter, frozenAfter, historyAfter float64
	require.NoError(t, rows.Scan(&amount, &balanceAfter, &quotaAfter, &frozenAfter, &historyAfter))
	require.InDelta(t, 12.34, amount, 1e-9)
	require.InDelta(t, 17.84, balanceAfter, 1e-9)
	require.InDelta(t, 0.0, quotaAfter, 1e-9)
	require.InDelta(t, 0.0, frozenAfter, 1e-9)
	require.InDelta(t, 12.34, historyAfter, 1e-9)
}

func TestAffiliateRepository_QualificationReconcileAndCount(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()
	repo := NewAffiliateRepository(client, integrationDB)
	qualificationRepo := repo.(service.AffiliateQualificationRepository)

	inviter := mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-tier-inviter-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive})
	invitees := make([]*service.User, 3)
	for i := range invitees {
		invitees[i] = mustCreateUser(t, client, &service.User{Email: fmt.Sprintf("affiliate-tier-invitee-%d-%d@example.com", time.Now().UnixNano(), i), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive})
		require.NoError(t, insertAffiliateRelationship(txCtx, client, invitees[i].ID, inviter.ID, fmt.Sprintf("TIER%07d", invitees[i].ID%10_000_000)))
	}

	insertQualificationOrder(t, txCtx, client, invitees[0].ID, 49, 0, "COMPLETED", "balance")
	insertQualificationOrder(t, txCtx, client, invitees[1].ID, 30, 0, "COMPLETED", "balance")
	insertQualificationOrder(t, txCtx, client, invitees[1].ID, 20, 0, "REFUND_PENDING", "subscription")
	insertQualificationOrder(t, txCtx, client, invitees[1].ID, 999, 0, "PENDING", "balance")
	insertQualificationOrder(t, txCtx, client, invitees[2].ID, 100, 30, "PARTIALLY_REFUNDED", "subscription")
	insertQualificationOrder(t, txCtx, client, invitees[2].ID, 500, 0, "COMPLETED", "other")

	for _, invitee := range invitees {
		_, err := qualificationRepo.ReconcileInviteeQualification(txCtx, invitee.ID, 50)
		require.NoError(t, err)
	}

	assertQualification := func(userID int64, amount float64, qualified bool) time.Time {
		t.Helper()
		rows, err := client.QueryContext(txCtx, "SELECT qualifying_payment_amount::double precision, qualified_at FROM user_affiliates WHERE user_id = $1", userID)
		require.NoError(t, err)
		defer func() { _ = rows.Close() }()
		require.True(t, rows.Next())
		var gotAmount float64
		var qualifiedAt *time.Time
		require.NoError(t, rows.Scan(&gotAmount, &qualifiedAt))
		require.InDelta(t, amount, gotAmount, 1e-9)
		if qualified {
			require.NotNil(t, qualifiedAt)
			return *qualifiedAt
		}
		require.Nil(t, qualifiedAt)
		return time.Time{}
	}

	assertQualification(invitees[0].ID, 49, false)
	qualifiedAt := assertQualification(invitees[1].ID, 50, true)
	assertQualification(invitees[2].ID, 70, true)

	count, err := qualificationRepo.CountQualifiedInvitees(txCtx, inviter.ID, 50)
	require.NoError(t, err)
	require.Equal(t, 2, count)
	count, err = qualificationRepo.CountQualifiedInvitees(txCtx, inviter.ID, 60)
	require.NoError(t, err)
	require.Equal(t, 1, count, "count must react immediately to a configured threshold change")

	_, err = qualificationRepo.ReconcileInviteeQualification(txCtx, invitees[1].ID, 50)
	require.NoError(t, err)
	require.Equal(t, qualifiedAt, assertQualification(invitees[1].ID, 50, true), "idempotent reconcile must preserve qualified_at")

	_, err = qualificationRepo.ReconcileInviteeQualification(txCtx, invitees[1].ID, 60)
	require.NoError(t, err)
	assertQualification(invitees[1].ID, 50, false)
}

func TestAffiliateRepository_QualificationAdvisoryLockIndependentConnections(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	repo1 := NewAffiliateRepository(integrationEntClient, integrationDB).(*affiliateRepository)
	repo2 := NewAffiliateRepository(integrationEntClient, integrationDB).(*affiliateRepository)
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseFirst := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseFirst()
	firstResult := make(chan error, 1)

	go func() {
		acquired, err := repo1.TryWithAffiliateQualificationReconcileLock(ctx, func(lockCtx context.Context) error {
			close(started)
			select {
			case <-release:
				return nil
			case <-lockCtx.Done():
				return lockCtx.Err()
			}
		})
		if err == nil && !acquired {
			err = errors.New("first repository did not acquire advisory lock")
		}
		firstResult <- err
	}()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal("timed out waiting for first advisory lock holder")
	}

	secondCalled := false
	acquired, err := repo2.TryWithAffiliateQualificationReconcileLock(ctx, func(context.Context) error {
		secondCalled = true
		return nil
	})
	require.NoError(t, err)
	require.False(t, acquired)
	require.False(t, secondCalled)

	releaseFirst()
	select {
	case err := <-firstResult:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for first advisory lock holder to finish")
	}
}

func TestAffiliateRepository_QualificationReconcileGenerationCAS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	repo := NewAffiliateRepository(integrationEntClient, integrationDB).(service.AffiliateQualificationRepository)
	clearAffiliateReconcilePending(t, ctx, repo)

	first, err := repo.MarkReconcileRequired(ctx)
	require.NoError(t, err)
	require.False(t, first.WasPendingBefore)
	snapshot, err := repo.ReadReconcilePendingSnapshot(ctx)
	require.NoError(t, err)
	require.True(t, snapshot.Required)
	require.Equal(t, first.Generation, snapshot.Generation)

	second, err := repo.MarkReconcileRequired(ctx)
	require.NoError(t, err)
	require.True(t, second.WasPendingBefore)
	require.Greater(t, second.Generation, first.Generation)
	cleared, err := repo.ClearReconcileRequiredIfGeneration(ctx, first.Generation)
	require.NoError(t, err)
	require.False(t, cleared, "stale generation must not clear a newer dirty marker")
	cleared, err = repo.ClearReconcileRequiredIfGeneration(ctx, second.Generation)
	require.NoError(t, err)
	require.True(t, cleared)
}

func TestAffiliateRepository_QualificationClearWaitsForMarkAndKeepsCommittedGeneration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base1 := NewAffiliateRepository(integrationEntClient, integrationDB)
	base2 := NewAffiliateRepository(integrationEntClient, integrationDB)
	repo1 := base1.(service.AffiliateQualificationRepository)
	repo2 := base2.(service.AffiliateQualificationRepository)
	clearAffiliateReconcilePending(t, ctx, repo1)

	before, err := repo1.ReadReconcilePendingSnapshot(ctx)
	require.NoError(t, err)
	tx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err)
	txFinished := false
	defer func() {
		if !txFinished {
			_ = tx.Rollback()
		}
	}()
	token, err := repo1.MarkReconcileRequired(dbent.NewTxContext(ctx, tx))
	require.NoError(t, err)
	require.Equal(t, before.Generation+1, token.Generation)

	type clearResult struct {
		cleared bool
		err     error
	}
	result := make(chan clearResult, 1)
	go func() {
		cleared, clearErr := repo2.ClearReconcileRequiredIfGeneration(ctx, before.Generation)
		result <- clearResult{cleared: cleared, err: clearErr}
	}()

	select {
	case got := <-result:
		t.Fatalf("clear returned before mark committed: cleared=%v err=%v", got.cleared, got.err)
	case <-time.After(150 * time.Millisecond):
	}
	require.NoError(t, tx.Commit())
	txFinished = true

	select {
	case got := <-result:
		require.NoError(t, got.err)
		require.False(t, got.cleared)
	case <-ctx.Done():
		t.Fatal("timed out waiting for clear after mark commit")
	}
	finalSnapshot, err := repo2.ReadReconcilePendingSnapshot(ctx)
	require.NoError(t, err)
	require.True(t, finalSnapshot.Required)
	require.Equal(t, token.Generation, finalSnapshot.Generation)
}

func TestAffiliateRepository_QualificationDirtyAuditDeleteRequiresCurrentDetail(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	repo := NewAffiliateRepository(integrationEntClient, integrationDB).(service.AffiliateQualificationRepository)
	orderID := fmt.Sprintf("task4-dirty-%d", time.Now().UnixNano())
	oldDetail := `{"userID":101,"orderStatus":"COMPLETED","eventType":"payment_completed"}`
	newDetail := `{"userID":101,"orderStatus":"REFUNDED","eventType":"refund_completed"}`
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), `
DELETE FROM payment_audit_logs WHERE order_id = $1 AND action = $2`, orderID, service.AffiliateQualificationDirtyAuditAction)
	})
	_, err := integrationDB.ExecContext(ctx, `
INSERT INTO payment_audit_logs (order_id, action, detail, operator, created_at)
VALUES ($1, $2, $3, 'test', NOW())`, orderID, service.AffiliateQualificationDirtyAuditAction, oldDetail)
	require.NoError(t, err)

	events, err := repo.ListAffiliateQualificationDirtyEvents(ctx, 500)
	require.NoError(t, err)
	oldEvent := findAffiliateQualificationDirtyEvent(t, events, orderID)
	_, err = integrationDB.ExecContext(ctx, `
UPDATE payment_audit_logs SET detail = $3 WHERE order_id = $1 AND action = $2`, orderID, service.AffiliateQualificationDirtyAuditAction, newDetail)
	require.NoError(t, err)

	deleted, err := repo.DeleteAffiliateQualificationDirtyEvent(ctx, oldEvent)
	require.NoError(t, err)
	require.False(t, deleted, "an old worker must not delete a newer terminal event")
	events, err = repo.ListAffiliateQualificationDirtyEvents(ctx, 500)
	require.NoError(t, err)
	newEvent := findAffiliateQualificationDirtyEvent(t, events, orderID)
	require.Equal(t, newDetail, newEvent.Detail)
	deleted, err = repo.DeleteAffiliateQualificationDirtyEvent(ctx, newEvent)
	require.NoError(t, err)
	require.True(t, deleted)
}

func findAffiliateQualificationDirtyEvent(t *testing.T, events []service.AffiliateQualificationDirtyEvent, orderID string) service.AffiliateQualificationDirtyEvent {
	t.Helper()
	for _, event := range events {
		if event.OrderID == orderID {
			return event
		}
	}
	t.Fatalf("affiliate qualification dirty event %s not found", orderID)
	return service.AffiliateQualificationDirtyEvent{}
}

func TestAffiliateRepository_QualificationServiceInstancesDatabaseLock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base1 := NewAffiliateRepository(integrationEntClient, integrationDB)
	base2 := NewAffiliateRepository(integrationEntClient, integrationDB)
	shared := &affiliateQualificationLockTestState{started: make(chan struct{}), release: make(chan struct{})}
	repo1 := &affiliateQualificationLockTestRepo{AffiliateRepository: base1, qualification: base1.(service.AffiliateQualificationRepository), shared: shared}
	repo2 := &affiliateQualificationLockTestRepo{AffiliateRepository: base2, qualification: base2.(service.AffiliateQualificationRepository), shared: shared}
	settingsRepo := newAffiliateQualificationLockSettingRepo()
	settings := service.NewSettingService(settingsRepo, nil)
	first := service.NewAffiliateService(repo1, settings, nil, nil)
	second := service.NewAffiliateService(repo2, settings, nil, nil)
	clearAffiliateReconcilePending(t, ctx, repo1)
	initialGeneration, err := repo1.MarkReconcileRequired(ctx)
	require.NoError(t, err)
	require.False(t, initialGeneration.WasPendingBefore)

	firstErr := make(chan error, 1)
	go func() { firstErr <- first.ReconcilePendingAffiliateQualifications(ctx) }()
	select {
	case <-shared.started:
	case <-ctx.Done():
		t.Fatal("timed out waiting for first qualification reconcile")
	}
	var releaseOnce sync.Once
	releaseWinner := func() { releaseOnce.Do(func() { close(shared.release) }) }
	defer releaseWinner()

	secondErr := second.ReconcilePendingAffiliateQualifications(ctx)
	snapshotWhileLocked, err := repo2.ReadReconcilePendingSnapshot(ctx)
	require.NoError(t, err)
	callsWhileLocked := shared.callCount()
	releaseWinner()

	require.ErrorIs(t, secondErr, service.ErrAffiliateQualificationReconcileBusy)
	require.True(t, snapshotWhileLocked.Required)
	require.Equal(t, initialGeneration.Generation, snapshotWhileLocked.Generation)
	require.Equal(t, 1, callsWhileLocked)
	select {
	case err := <-firstErr:
		require.NoError(t, err)
	case <-ctx.Done():
		t.Fatal("timed out waiting for winning qualification reconcile")
	}
	finalSnapshot, err := repo2.ReadReconcilePendingSnapshot(ctx)
	require.NoError(t, err)
	require.False(t, finalSnapshot.Required)
	require.Equal(t, initialGeneration.Generation, finalSnapshot.Generation)
}

func TestAffiliateRepository_QualificationFullReconcileDoesNotClearConcurrentGeneration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	base1 := NewAffiliateRepository(integrationEntClient, integrationDB)
	base2 := NewAffiliateRepository(integrationEntClient, integrationDB)
	shared := &affiliateQualificationLockTestState{started: make(chan struct{}), release: make(chan struct{})}
	repo1 := &affiliateQualificationLockTestRepo{AffiliateRepository: base1, qualification: base1.(service.AffiliateQualificationRepository), shared: shared}
	qualification2 := base2.(service.AffiliateQualificationRepository)
	svc := service.NewAffiliateService(repo1, service.NewSettingService(newAffiliateQualificationLockSettingRepo(), nil), nil, nil)
	clearAffiliateReconcilePending(t, ctx, repo1)
	initialGeneration, err := repo1.MarkReconcileRequired(ctx)
	require.NoError(t, err)
	require.False(t, initialGeneration.WasPendingBefore)

	result := make(chan error, 1)
	go func() { result <- svc.ReconcilePendingAffiliateQualifications(ctx) }()
	select {
	case <-shared.started:
	case <-ctx.Done():
		t.Fatal("timed out waiting for full qualification reconcile")
	}
	concurrentGeneration, err := qualification2.MarkReconcileRequired(ctx)
	require.NoError(t, err)
	require.True(t, concurrentGeneration.WasPendingBefore)
	require.Greater(t, concurrentGeneration.Generation, initialGeneration.Generation)
	close(shared.release)
	select {
	case err := <-result:
		require.ErrorIs(t, err, service.ErrAffiliateQualificationReconcileStale)
	case <-ctx.Done():
		t.Fatal("timed out waiting for stale full qualification reconcile")
	}
	finalSnapshot, err := qualification2.ReadReconcilePendingSnapshot(ctx)
	require.NoError(t, err)
	require.True(t, finalSnapshot.Required)
	require.Equal(t, concurrentGeneration.Generation, finalSnapshot.Generation)
}

func clearAffiliateReconcilePending(t *testing.T, ctx context.Context, repo service.AffiliateQualificationRepository) {
	t.Helper()
	snapshot, err := repo.ReadReconcilePendingSnapshot(ctx)
	require.NoError(t, err)
	if !snapshot.Required {
		return
	}
	cleared, err := repo.ClearReconcileRequiredIfGeneration(ctx, snapshot.Generation)
	require.NoError(t, err)
	require.True(t, cleared)
}

func TestAffiliateRepository_QualificationFullReconcileUsesIndependentTransactions(t *testing.T) {
	ctx := context.Background()
	repo := NewAffiliateRepository(integrationEntClient, integrationDB)
	inviter := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("affiliate-full-inviter-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive,
	})
	invitee := mustCreateUser(t, integrationEntClient, &service.User{
		Email: fmt.Sprintf("affiliate-full-invitee-%d@example.com", time.Now().UnixNano()), PasswordHash: "hash", Role: service.RoleUser, Status: service.StatusActive,
	})
	t.Cleanup(func() {
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM payment_orders WHERE user_id = $1", invitee.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM user_affiliates WHERE user_id IN ($1, $2)", inviter.ID, invitee.ID)
		_, _ = integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id IN ($1, $2)", inviter.ID, invitee.ID)
	})
	_, err := repo.EnsureUserAffiliate(ctx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(ctx, invitee.ID)
	require.NoError(t, err)
	bound, err := repo.BindInviter(ctx, invitee.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound)
	insertQualificationOrder(t, ctx, integrationEntClient, invitee.ID, 50, 0, "COMPLETED", "balance")

	require.NoError(t, repo.(service.AffiliateQualificationRepository).ReconcileAllAffiliateQualifications(ctx, 50, 1))
	require.InDelta(t, 50.0, querySingleFloat(t, ctx, integrationEntClient,
		"SELECT qualifying_payment_amount::double precision FROM user_affiliates WHERE user_id = $1", invitee.ID), 1e-9)

	const readers = 8
	var wg sync.WaitGroup
	errs := make(chan error, readers)
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			count, err := repo.(service.AffiliateQualificationRepository).CountQualifiedInvitees(ctx, inviter.ID, 50)
			if err == nil && count != 1 {
				err = fmt.Errorf("qualified count = %d, want 1", count)
			}
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func insertAffiliateRelationship(ctx context.Context, client *dbent.Client, userID, inviterID int64, code string) error {
	_, err := client.ExecContext(ctx, `
INSERT INTO user_affiliates (user_id, aff_code, inviter_id, created_at, updated_at)
VALUES ($1, $2, $3, NOW(), NOW())`, userID, code, inviterID)
	return err
}

type affiliateQualificationLockTestState struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	release chan struct{}
}

func (s *affiliateQualificationLockTestState) reconcile(ctx context.Context) error {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	if call == 1 {
		close(s.started)
		select {
		case <-s.release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *affiliateQualificationLockTestState) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

type affiliateQualificationLockTestRepo struct {
	service.AffiliateRepository
	qualification service.AffiliateQualificationRepository
	shared        *affiliateQualificationLockTestState
}

func (r *affiliateQualificationLockTestRepo) ReconcileInviteeQualification(ctx context.Context, userID int64, threshold float64) (*service.AffiliateQualification, error) {
	return r.qualification.ReconcileInviteeQualification(ctx, userID, threshold)
}

func (r *affiliateQualificationLockTestRepo) CountQualifiedInvitees(ctx context.Context, inviterID int64, threshold float64) (int, error) {
	return r.qualification.CountQualifiedInvitees(ctx, inviterID, threshold)
}

func (r *affiliateQualificationLockTestRepo) ReconcileAllAffiliateQualifications(ctx context.Context, _ float64, _ int) error {
	return r.shared.reconcile(ctx)
}

func (r *affiliateQualificationLockTestRepo) TryWithAffiliateQualificationReconcileLock(ctx context.Context, fn func(context.Context) error) (bool, error) {
	return r.qualification.TryWithAffiliateQualificationReconcileLock(ctx, fn)
}

func (r *affiliateQualificationLockTestRepo) MarkReconcileRequired(ctx context.Context) (service.AffiliateReconcileToken, error) {
	return r.qualification.MarkReconcileRequired(ctx)
}

func (r *affiliateQualificationLockTestRepo) ReadReconcilePendingSnapshot(ctx context.Context) (service.AffiliateReconcilePendingSnapshot, error) {
	return r.qualification.ReadReconcilePendingSnapshot(ctx)
}

func (r *affiliateQualificationLockTestRepo) ClearReconcileRequiredIfGeneration(ctx context.Context, expected int64) (bool, error) {
	return r.qualification.ClearReconcileRequiredIfGeneration(ctx, expected)
}

func (r *affiliateQualificationLockTestRepo) ListAffiliateQualificationDirtyEvents(ctx context.Context, limit int) ([]service.AffiliateQualificationDirtyEvent, error) {
	return r.qualification.ListAffiliateQualificationDirtyEvents(ctx, limit)
}

func (r *affiliateQualificationLockTestRepo) DeleteAffiliateQualificationDirtyEvent(ctx context.Context, event service.AffiliateQualificationDirtyEvent) (bool, error) {
	return r.qualification.DeleteAffiliateQualificationDirtyEvent(ctx, event)
}

func (r *affiliateQualificationLockTestRepo) MarkAffiliateQualificationDirtyEventFailed(ctx context.Context, event service.AffiliateQualificationDirtyEvent, cause error) error {
	return r.qualification.MarkAffiliateQualificationDirtyEventFailed(ctx, event, cause)
}

type affiliateQualificationLockSettingRepo struct {
	service.SettingRepository
	mu     sync.Mutex
	values map[string]string
}

func newAffiliateQualificationLockSettingRepo() *affiliateQualificationLockSettingRepo {
	return &affiliateQualificationLockSettingRepo{values: map[string]string{
		service.SettingKeyAffiliateRebateRate:          "8",
		service.SettingKeyAffiliateQualificationAmount: "50",
		service.SettingKeyAffiliateBronzeInvitees:      "3",
		service.SettingKeyAffiliateBronzeRate:          "10",
		service.SettingKeyAffiliateSilverInvitees:      "10",
		service.SettingKeyAffiliateSilverRate:          "12",
		service.SettingKeyAffiliateGoldInvitees:        "30",
		service.SettingKeyAffiliateGoldRate:            "15",
	}}
}

func (r *affiliateQualificationLockSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", service.ErrSettingNotFound
	}
	return value, nil
}

func (r *affiliateQualificationLockSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			out[key] = value
		}
	}
	return out, nil
}

func (r *affiliateQualificationLockSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}

func insertQualificationOrder(t *testing.T, ctx context.Context, client *dbent.Client, userID int64, amount, refundAmount float64, status, orderType string) {
	t.Helper()
	now := time.Now()
	_, err := client.PaymentOrder.Create().
		SetUserID(userID).
		SetUserEmail(fmt.Sprintf("qualification-%d@example.com", userID)).
		SetUserName("qualification").
		SetAmount(amount).
		SetPayAmount(amount).
		SetFeeRate(0).
		SetRechargeCode(fmt.Sprintf("QUAL-%d-%d", userID, now.UnixNano())).
		SetOutTradeNo(fmt.Sprintf("qual_%d_%d", userID, now.UnixNano())).
		SetPaymentType("test").
		SetPaymentTradeNo(fmt.Sprintf("trade_%d", now.UnixNano())).
		SetOrderType(orderType).
		SetStatus(status).
		SetRefundAmount(refundAmount).
		SetExpiresAt(now.Add(time.Hour)).
		SetClientIP("127.0.0.1").
		SetSrcHost("affiliate-qualification-test").
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	require.NoError(t, err)
}

// TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction guards the
// cross-layer tx propagation invariant: when AccrueQuota is called with a ctx
// that already carries a transaction (via dbent.NewTxContext), repo.withTx
// must reuse that tx rather than opening a nested one. If this invariant
// breaks, AccrueQuota would commit independently and survive a rollback of
// the outer tx, which would violate payment_fulfillment's all-or-nothing
// semantics.
func TestAffiliateRepository_AccrueQuota_ReusesOuterTransaction(t *testing.T) {
	ctx := context.Background()

	outerTx, err := integrationEntClient.Tx(ctx)
	require.NoError(t, err, "begin outer tx")
	// Defensive cleanup: if any require.* below fires before the explicit
	// Rollback, this prevents the tx from leaking until container teardown.
	// Rollback is idempotent at the driver level (extra rollback returns an
	// error we ignore).
	t.Cleanup(func() { _ = outerTx.Rollback() })
	client := outerTx.Client()
	txCtx := dbent.NewTxContext(ctx, outerTx)

	inviter := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-inviter-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})
	invitee := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-invitee-%d@example.com", time.Now().UnixNano()+1),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Concurrency:  5,
	})

	repo := NewAffiliateRepository(client, integrationDB)
	_, err = repo.EnsureUserAffiliate(txCtx, inviter.ID)
	require.NoError(t, err)
	_, err = repo.EnsureUserAffiliate(txCtx, invitee.ID)
	require.NoError(t, err)

	bound, err := repo.BindInviter(txCtx, invitee.ID, inviter.ID)
	require.NoError(t, err)
	require.True(t, bound, "invitee must bind to inviter")

	applied, err := repo.AccrueQuota(txCtx, inviter.ID, invitee.ID, 3.5, 0, nil)
	require.NoError(t, err)
	require.True(t, applied, "AccrueQuota must report applied=true")

	// Visible inside the outer tx.
	innerQuota := querySingleFloat(t, txCtx, client,
		"SELECT aff_quota::double precision FROM user_affiliates WHERE user_id = $1", inviter.ID)
	require.InDelta(t, 3.5, innerQuota, 1e-9)

	// Roll back the outer tx; if AccrueQuota had opened its own inner tx and
	// committed it, the rows would still be visible to the global client.
	require.NoError(t, outerTx.Rollback())

	rows, err := integrationEntClient.QueryContext(ctx,
		"SELECT COUNT(*) FROM user_affiliates WHERE user_id IN ($1, $2)",
		inviter.ID, invitee.ID)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	require.True(t, rows.Next())
	var postRollbackCount int
	require.NoError(t, rows.Scan(&postRollbackCount))
	require.Equal(t, 0, postRollbackCount,
		"AccrueQuota must propagate the outer tx — found persisted rows after rollback")
}

func TestAffiliateRepository_TransferQuotaToBalance_EmptyQuota(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-empty-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
		Balance:      3.21,
		Concurrency:  5,
	})

	affCode := fmt.Sprintf("AFF%09d", time.Now().UnixNano()%1_000_000_000)
	_, err := client.ExecContext(txCtx, `
INSERT INTO user_affiliates (user_id, aff_code, aff_quota, aff_history_quota, created_at, updated_at)
VALUES ($1, $2, 0, 0, NOW(), NOW())`, u.ID, affCode)
	require.NoError(t, err)

	transferred, balance, err := repo.TransferQuotaToBalance(txCtx, u.ID)
	require.ErrorIs(t, err, service.ErrAffiliateQuotaEmpty)
	require.InDelta(t, 0.0, transferred, 1e-9)
	require.InDelta(t, 0.0, balance, 1e-9)

	persistedBalance := querySingleFloat(t, txCtx, client,
		"SELECT balance::double precision FROM users WHERE id = $1", u.ID)
	require.InDelta(t, 3.21, persistedBalance, 1e-9)
}

// TestAffiliateRepository_AdminCustomCode covers the success path of admin
// invite-code rewrite + reset within a shared test transaction:
// - UpdateUserAffCode replaces aff_code, sets aff_code_custom=true, lookup works
// - the old code can no longer be found
// - ResetUserAffCode reverts aff_code_custom and assigns a new system-format code
//
// The conflict path (duplicate code → ErrAffiliateCodeTaken) lives in its own
// test because a unique-violation aborts the surrounding Postgres tx, which
// would poison subsequent assertions in the same transaction.
func TestAffiliateRepository_AdminCustomCode(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-custom-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	original, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.False(t, original.AffCodeCustom, "system-generated codes start as non-custom")
	originalCode := original.AffCode

	// Rewrite to a custom code
	customCode := fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, u.ID, customCode))

	updated, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, customCode, updated.AffCode)
	require.True(t, updated.AffCodeCustom)

	// Lookup by new custom code finds the user
	byCode, err := repo.GetAffiliateByCode(txCtx, customCode)
	require.NoError(t, err)
	require.Equal(t, u.ID, byCode.UserID)

	// Old system code should no longer match
	_, err = repo.GetAffiliateByCode(txCtx, originalCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)

	// Reset back to a fresh system code, clears custom flag
	newSysCode, err := repo.ResetUserAffCode(txCtx, u.ID)
	require.NoError(t, err)
	require.NotEqual(t, customCode, newSysCode)

	reset, err := repo.EnsureUserAffiliate(txCtx, u.ID)
	require.NoError(t, err)
	require.Equal(t, newSysCode, reset.AffCode)
	require.False(t, reset.AffCodeCustom)

	// The old custom code is now free again
	_, err = repo.GetAffiliateByCode(txCtx, customCode)
	require.ErrorIs(t, err, service.ErrAffiliateProfileNotFound)
}

// TestAffiliateRepository_AdminCustomCode_Conflict isolates the unique-violation
// path. PostgreSQL aborts the enclosing tx when a unique constraint fires, so
// this test must be the only assertion and run in its own tx — production
// callers each have their own outer tx, so this matches real behavior.
func TestAffiliateRepository_AdminCustomCode_Conflict(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	taker := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-taker-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	requester := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-conflict-req-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})

	takenCode := fmt.Sprintf("HOT%09d", time.Now().UnixNano()%1_000_000_000)
	require.NoError(t, repo.UpdateUserAffCode(txCtx, taker.ID, takenCode))

	// Now requester tries to grab the same code → conflict.
	err := repo.UpdateUserAffCode(txCtx, requester.ID, takenCode)
	require.ErrorIs(t, err, service.ErrAffiliateCodeTaken)
}

// TestAffiliateRepository_AdminRebateRate covers per-user exclusive rate
// set/clear and the Batch variant including NULL semantics.
func TestAffiliateRepository_AdminRebateRate(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	u1 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-a@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})
	u2 := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rate-%d-b@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser,
		Status:       service.StatusActive,
	})

	// Set exclusive rate for u1
	rate := 42.5
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, &rate))

	got, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.NotNil(t, got.AffRebateRatePercent)
	require.InDelta(t, 42.5, *got.AffRebateRatePercent, 1e-9)

	// Clear exclusive rate
	require.NoError(t, repo.SetUserRebateRate(txCtx, u1.ID, nil))
	cleared, err := repo.EnsureUserAffiliate(txCtx, u1.ID)
	require.NoError(t, err)
	require.Nil(t, cleared.AffRebateRatePercent)

	// Batch set both users
	batchRate := 15.0
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, &batchRate))

	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.NotNil(t, v.AffRebateRatePercent)
		require.InDelta(t, 15.0, *v.AffRebateRatePercent, 1e-9)
	}

	// Batch clear
	require.NoError(t, repo.BatchSetUserRebateRate(txCtx, []int64{u1.ID, u2.ID}, nil))
	for _, uid := range []int64{u1.ID, u2.ID} {
		v, err := repo.EnsureUserAffiliate(txCtx, uid)
		require.NoError(t, err)
		require.Nil(t, v.AffRebateRatePercent)
	}
}

// TestAffiliateRepository_ListUsersWithCustomSettings verifies the admin list
// only includes users with at least one override applied.
func TestAffiliateRepository_ListUsersWithCustomSettings(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	txCtx := dbent.NewTxContext(ctx, tx)
	client := tx.Client()

	repo := NewAffiliateRepository(client, integrationDB)

	// User without any custom config — should NOT appear in the list.
	plainEmail := fmt.Sprintf("affiliate-plain-%d@example.com", time.Now().UnixNano())
	uPlain := mustCreateUser(t, client, &service.User{
		Email: plainEmail, PasswordHash: "hash",
		Role: service.RoleUser, Status: service.StatusActive,
	})
	_, err := repo.EnsureUserAffiliate(txCtx, uPlain.ID)
	require.NoError(t, err)

	// User with a custom code — should appear.
	uCode := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-codeonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	require.NoError(t, repo.UpdateUserAffCode(txCtx, uCode.ID, fmt.Sprintf("VIP%09d", time.Now().UnixNano()%1_000_000_000)))

	// User with only an exclusive rate — should appear.
	uRate := mustCreateUser(t, client, &service.User{
		Email:        fmt.Sprintf("affiliate-rateonly-%d@example.com", time.Now().UnixNano()),
		PasswordHash: "hash",
		Role:         service.RoleUser, Status: service.StatusActive,
	})
	r := 33.3
	require.NoError(t, repo.SetUserRebateRate(txCtx, uRate.ID, &r))

	entries, total, err := repo.ListUsersWithCustomSettings(txCtx, service.AffiliateAdminFilter{
		Page: 1, PageSize: 100,
	})
	require.NoError(t, err)

	// Build a quick lookup to assert per-user attributes (other tests may have
	// inserted custom rows in the same DB; we only care about our 3).
	byUserID := make(map[int64]service.AffiliateAdminEntry, len(entries))
	for _, e := range entries {
		byUserID[e.UserID] = e
	}

	require.NotContains(t, byUserID, uPlain.ID, "users without overrides must not appear")

	codeEntry, ok := byUserID[uCode.ID]
	require.True(t, ok, "custom-code user missing from list")
	require.True(t, codeEntry.AffCodeCustom)
	require.Nil(t, codeEntry.AffRebateRatePercent)

	rateEntry, ok := byUserID[uRate.ID]
	require.True(t, ok, "custom-rate user missing from list")
	require.False(t, rateEntry.AffCodeCustom)
	require.NotNil(t, rateEntry.AffRebateRatePercent)
	require.InDelta(t, 33.3, *rateEntry.AffRebateRatePercent, 1e-9)

	require.GreaterOrEqual(t, total, int64(2), "total must include at least our 2 custom rows")
}
