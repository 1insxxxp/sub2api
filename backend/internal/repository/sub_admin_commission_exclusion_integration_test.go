//go:build integration

package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestSubAdminCommissionExclusionRollupPreservesGrossCost(t *testing.T) {
	ctx := context.Background()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()
	require.NoError(t, setGroupUsageRollupTriggerTimeZone(ctx, tx, "UTC"))
	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (42), (43);
		INSERT INTO groups (id) VALUES (1);
		INSERT INTO sub_admin_commission_exclusions (user_id, started_at, ended_at) VALUES
			(42, TIMESTAMPTZ '2026-08-13 10:00:00+00', TIMESTAMPTZ '2026-08-13 11:00:00+00'),
			(42, TIMESTAMPTZ '2026-08-13 12:00:00+00', NULL);
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at) VALUES
			(1, 42, 1, 1, TIMESTAMPTZ '2026-08-13 09:59:59+00'),
			(2, 42, 1, 2, TIMESTAMPTZ '2026-08-13 10:00:00+00'),
			(3, 42, 1, 3, TIMESTAMPTZ '2026-08-13 11:00:00+00'),
			(4, 42, 1, 4, TIMESTAMPTZ '2026-08-13 12:00:00+00'),
			(5, 43, 1, 5, TIMESTAMPTZ '2026-08-13 12:00:00+00');
	`)
	require.NoError(t, err)
	repo := newDashboardAggregationRepositoryWithSQL(tx)
	require.NoError(t, repo.SyncGroupUsageRollups(ctx, time.Date(2026, 8, 14, 0, 0, 0, 0, time.UTC)))
	var grossCost, excludedCost float64
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT actual_cost, commission_excluded_cost FROM usage_group_daily_rollups WHERE group_id = 1`).Scan(&grossCost, &excludedCost))
	require.Equal(t, float64(15), grossCost)
	require.Equal(t, float64(6), excludedCost)
}

func newCommissionExclusionCustomer(t *testing.T) (*subAdminCommissionRepository, int64) {
	t.Helper()
	var userID int64
	email := fmt.Sprintf("commission-exclusion-%d@example.invalid", time.Now().UnixNano())
	require.NoError(t, integrationDB.QueryRow(`INSERT INTO users (email, password_hash, username) VALUES ($1, 'test-hash', 'customer') RETURNING id`, email).Scan(&userID))
	t.Cleanup(func() {
		_, _ = integrationDB.Exec(`DELETE FROM sub_admin_commission_exclusions WHERE user_id = $1`, userID)
		_, _ = integrationDB.Exec(`DELETE FROM users WHERE id = $1`, userID)
	})
	return &subAdminCommissionRepository{db: integrationDB, sql: integrationDB}, userID
}

func TestSubAdminCommissionExclusionLifecyclePreservesClosedHistory(t *testing.T) {
	repo, userID := newCommissionExclusionCustomer(t)
	ctx := context.Background()
	before := time.Now().Add(-time.Second)
	first, err := repo.AddExclusion(ctx, userID, 0)
	require.NoError(t, err)
	require.True(t, first.StartedAt.After(before))
	require.Equal(t, userID, first.UserID)
	require.Equal(t, "customer", first.Username)
	repeated, err := repo.AddExclusion(ctx, userID, 0)
	require.NoError(t, err)
	require.Equal(t, first.ID, repeated.ID)
	require.Equal(t, first.StartedAt, repeated.StartedAt)
	items, page, err := repo.ListExclusions(ctx, pagination.PaginationParams{Page: 1, PageSize: 1000})
	require.NoError(t, err)
	require.GreaterOrEqual(t, page.Total, int64(1))
	found := false
	for _, item := range items {
		found = found || item.ID == first.ID
	}
	require.True(t, found)
	require.NoError(t, repo.RemoveExclusion(ctx, userID))
	require.NoError(t, repo.RemoveExclusion(ctx, userID))
	second, err := repo.AddExclusion(ctx, userID, 0)
	require.NoError(t, err)
	require.NotEqual(t, first.ID, second.ID)
	var endedAt time.Time
	require.NoError(t, integrationDB.QueryRow(`SELECT ended_at FROM sub_admin_commission_exclusions WHERE id = $1`, first.ID).Scan(&endedAt))
	require.False(t, endedAt.Before(first.StartedAt))
	require.False(t, second.StartedAt.Before(endedAt))
	var intervalCount int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM sub_admin_commission_exclusions WHERE user_id = $1`, userID).Scan(&intervalCount))
	require.Equal(t, 2, intervalCount)
}

func TestSubAdminCommissionExclusionConcurrentAddsKeepOneOriginalInterval(t *testing.T) {
	repo, userID := newCommissionExclusionCustomer(t)
	const workers = 8
	results := make(chan *service.SubAdminCommissionExclusion, workers)
	errors := make(chan error, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			item, err := repo.AddExclusion(context.Background(), userID, 0)
			results <- item
			errors <- err
		}()
	}
	wg.Wait()
	close(results)
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	var original *service.SubAdminCommissionExclusion
	for item := range results {
		if original == nil {
			original = item
		}
		require.Equal(t, original.ID, item.ID)
		require.Equal(t, original.StartedAt, item.StartedAt)
	}
	var count int
	require.NoError(t, integrationDB.QueryRow(`SELECT COUNT(*) FROM sub_admin_commission_exclusions WHERE user_id = $1`, userID).Scan(&count))
	require.Equal(t, 1, count)
}

func TestSubAdminCommissionExclusionConcurrentAddRemoveNeverOverlap(t *testing.T) {
	repo, userID := newCommissionExclusionCustomer(t)
	ctx := context.Background()
	first, err := repo.AddExclusion(ctx, userID, 0)
	require.NoError(t, err)
	const workers = 16
	errors := make(chan error, workers)
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(remove bool) {
			defer wg.Done()
			if remove {
				errors <- repo.RemoveExclusion(ctx, userID)
			} else {
				_, err := repo.AddExclusion(ctx, userID, 0)
				errors <- err
			}
		}(i%2 == 0)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.NoError(t, repo.RemoveExclusion(ctx, userID))
	var overlaps int
	require.NoError(t, integrationDB.QueryRow(`
		SELECT COUNT(*) FROM sub_admin_commission_exclusions a
		JOIN sub_admin_commission_exclusions b ON a.user_id = b.user_id AND a.id < b.id
		WHERE a.user_id = $1
			AND a.started_at < COALESCE(b.ended_at, 'infinity'::timestamptz)
			AND b.started_at < COALESCE(a.ended_at, 'infinity'::timestamptz)
	`, userID).Scan(&overlaps))
	require.Zero(t, overlaps)
	var originalExists bool
	require.NoError(t, integrationDB.QueryRow(`SELECT EXISTS (SELECT 1 FROM sub_admin_commission_exclusions WHERE id = $1 AND ended_at IS NOT NULL)`, first.ID).Scan(&originalExists))
	require.True(t, originalExists, "closing an interval must never erase historical exclusion")
}

func TestSubAdminCommissionExclusionAdministratorsCanExcludeEachOtherConcurrently(t *testing.T) {
	repo, firstAdmin := newCommissionExclusionCustomer(t)
	_, secondAdmin := newCommissionExclusionCustomer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := integrationDB.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE id IN ($1, $2)`, firstAdmin, secondAdmin)
	require.NoError(t, err)
	publisher, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = publisher.Rollback() }()
	var watermarkID int
	require.NoError(t, publisher.QueryRowContext(ctx, `SELECT id FROM usage_group_rollup_state WHERE id = 1 FOR UPDATE`).Scan(&watermarkID))
	done := make(chan error, 2)
	for _, pair := range [][2]int64{{firstAdmin, secondAdmin}, {secondAdmin, firstAdmin}} {
		go func(target, operator int64) {
			_, err := repo.AddExclusion(ctx, target, operator)
			done <- err
		}(pair[0], pair[1])
	}
	// The publisher barrier lets both operations lock their respective target
	// before either reaches the created_by FK check. This exposes a reciprocal
	// target FOR UPDATE / operator KEY SHARE deadlock deterministically.
	require.Eventually(t, func() bool {
		var waiting int
		err := integrationDB.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND state = 'active' AND wait_event_type = 'Lock'
				AND query LIKE '%usage_group_rollup_state%FOR KEY SHARE%'
		`).Scan(&waiting)
		return err == nil && waiting == 2
	}, 5*time.Second, 10*time.Millisecond)
	require.NoError(t, publisher.Commit())
	for range 2 {
		select {
		case err := <-done:
			require.NoError(t, err, "reciprocal administrator exclusions must not deadlock on created_by")
		case <-ctx.Done():
			t.Fatal("reciprocal administrator exclusions did not complete")
		}
	}
	var count int
	require.NoError(t, integrationDB.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM sub_admin_commission_exclusions
		WHERE (user_id = $1 AND created_by = $2) OR (user_id = $2 AND created_by = $1)
	`, firstAdmin, secondAdmin).Scan(&count))
	require.Equal(t, 2, count, "operator attribution and both exclusions must be retained")
}

func TestSubAdminCommissionExclusionRejectsMissingAndSoftDeletedCustomers(t *testing.T) {
	repo, userID := newCommissionExclusionCustomer(t)
	ctx := context.Background()
	_, err := repo.AddExclusion(ctx, -1, 0)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.ErrorIs(t, repo.RemoveExclusion(ctx, -1), service.ErrUserNotFound)
	_, err = integrationDB.Exec(`UPDATE users SET deleted_at = NOW() WHERE id = $1`, userID)
	require.NoError(t, err)
	_, err = repo.AddExclusion(ctx, userID, 0)
	require.ErrorIs(t, err, service.ErrUserNotFound)
	require.ErrorIs(t, repo.RemoveExclusion(ctx, userID), service.ErrUserNotFound)
}

func TestSubAdminCommissionExclusionMutationsWaitForRollupPublisher(t *testing.T) {
	for _, operation := range []string{"add", "remove"} {
		t.Run(operation, func(t *testing.T) {
			repo, userID := newCommissionExclusionCustomer(t)
			ctx := context.Background()
			if operation == "remove" {
				_, err := repo.AddExclusion(ctx, userID, 0)
				require.NoError(t, err)
			}
			publisher, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = publisher.Rollback() }()
			var id int
			require.NoError(t, publisher.QueryRowContext(ctx, `SELECT id FROM usage_group_rollup_state WHERE id = 1 FOR UPDATE`).Scan(&id))
			done := make(chan error, 1)
			go func() {
				if operation == "add" {
					_, err := repo.AddExclusion(ctx, userID, 0)
					done <- err
				} else {
					done <- repo.RemoveExclusion(ctx, userID)
				}
			}()
			select {
			case err := <-done:
				t.Fatalf("mutation completed while publisher still owns watermark lock: %v", err)
			case <-time.After(150 * time.Millisecond):
			}
			var releasedAt time.Time
			require.NoError(t, publisher.QueryRowContext(ctx, `SELECT clock_timestamp()`).Scan(&releasedAt))
			require.NoError(t, publisher.Commit())
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-time.After(5 * time.Second):
				t.Fatal("exclusion mutation did not resume after publisher commit")
			}
			column := "started_at"
			if operation == "remove" {
				column = "ended_at"
			}
			var boundary time.Time
			require.NoError(t, integrationDB.QueryRow(`SELECT `+column+` FROM sub_admin_commission_exclusions WHERE user_id = $1`, userID).Scan(&boundary))
			require.False(t, boundary.Before(releasedAt), "boundary must be generated after waiting for publisher")
		})
	}
}

func TestSubAdminCommissionExclusionRollupInvalidatesWhenUsageCustomerChanges(t *testing.T) {
	ctx := context.Background()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	tx := beginGroupUsageRollupTriggerTestTx(t, ctx, schema)
	defer func() { _ = tx.Rollback() }()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO users (id) VALUES (42), (43);
		INSERT INTO groups (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		VALUES (1, 42, 1, 1, TIMESTAMPTZ '2026-08-13 10:00:00+00');
		UPDATE usage_group_rollup_state SET closed_before = DATE '2026-08-14';
		UPDATE usage_logs SET user_id = 43 WHERE id = 1;
	`)
	require.NoError(t, err)
	var closedBefore string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT closed_before::text FROM usage_group_rollup_state`).Scan(&closedBefore))
	require.Equal(t, "2026-08-13", closedBefore)
}
