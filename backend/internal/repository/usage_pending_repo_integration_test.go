//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestPendingUsageRepositoryHasIndependentStore(t *testing.T) {
	tx := testEntTx(t)
	repo := newUsageLogRepositoryWithSQL(tx.Client(), tx)
	_, ok := any(repo).(interface {
		UpsertPendingUsage(context.Context, *service.UsageLog) error
	})
	require.True(t, ok, "unmeasured requests need an independent durable store, never a zero-cost billing placeholder")
}

func TestPendingUsageConcurrentSettlementCannotResurrectAfterPurge(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := integrationEntClient
	prefix := fmt.Sprintf("pending-race-%d", time.Now().UnixNano())
	user := mustCreateUser(t, client, &service.User{Email: prefix + "@example.invalid"})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-" + prefix, Name: prefix})
	account := mustCreateAccount(t, client, &service.Account{Name: prefix})
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM usage_pending_records WHERE api_key_id = $1", key.ID)
		_, _ = integrationDB.ExecContext(cleanupCtx, "DELETE FROM usage_logs WHERE api_key_id = $1", key.ID)
		_ = client.User.DeleteOneID(user.ID).Exec(cleanupCtx)
		_ = client.Account.DeleteOneID(account.ID).Exec(cleanupCtx)
	})
	for _, pendingFirst := range []bool{true, false} {
		name := "settlement_first"
		if pendingFirst {
			name = "pending_first"
		}
		t.Run(name, func(t *testing.T) {
			pending := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: prefix + "-" + name, Model: "gemini-test", CreatedAt: time.Now()}
			settled := *pending
			settled.InputTokens, settled.OutputTokens = 23, 7
			hold, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = hold.Rollback() }()
			waiter, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = waiter.Rollback() }()
			var waiterPID int
			require.NoError(t, waiter.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&waiterPID))
			holdRepo := newUsageLogRepositoryWithSQL(client, hold)
			waitRepo := newUsageLogRepositoryWithSQL(client, waiter)
			if pendingFirst {
				require.NoError(t, holdRepo.UpsertPendingUsage(ctx, pending))
			} else {
				_, err = holdRepo.createSingle(ctx, hold, &settled)
				require.NoError(t, err)
			}
			done := make(chan error, 1)
			go func() {
				var err error
				if pendingFirst {
					_, err = waitRepo.createSingle(ctx, waiter, &settled)
				} else {
					err = waitRepo.UpsertPendingUsage(ctx, pending)
				}
				if err == nil {
					err = waiter.Commit()
				}
				done <- err
			}()
			// Verify the intended interleaving using the real database lock state,
			// rather than relying on a timing-only sleep.
			require.Eventually(t, func() bool {
				var blocked bool
				err := integrationDB.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM pg_locks WHERE pid = $1 AND locktype = 'advisory' AND NOT granted)", waiterPID).Scan(&blocked)
				return err == nil && blocked
			}, 5*time.Second, 10*time.Millisecond)
			require.NoError(t, hold.Commit())
			select {
			case err := <-done:
				require.NoError(t, err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			var unresolved, dedup int64
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_pending_records WHERE request_id = $1 AND resolved_at IS NULL", pending.RequestID).Scan(&unresolved))
			require.Zero(t, unresolved, "both transaction orderings must resolve or suppress pending evidence")
			require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_billing_dedup WHERE request_id = $1", pending.RequestID).Scan(&dedup))
			require.Zero(t, dedup, "exercise simple-mode race without relying on a billing tombstone")
			repo := newUsageLogRepositoryWithSQL(client, integrationDB)
			require.NoError(t, repo.Delete(ctx, settled.ID))
			logs, page, err := repo.ListWithFilters(ctx, pagination.PaginationParams{Page: 1, PageSize: 10}, usagestats.UsageLogFilters{UserID: user.ID, RequestID: pending.RequestID, IncludePending: true})
			require.NoError(t, err)
			require.Empty(t, logs)
			require.Zero(t, page.Total)
		})
	}
	t.Run("reject stale outer snapshot without taking ownership", func(t *testing.T) {
		tx, err := integrationDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
		require.NoError(t, err)
		defer func() { _ = tx.Rollback() }()
		repo := newUsageLogRepositoryWithSQL(client, tx)
		pending := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, RequestID: prefix + "-stale", Model: "gemini-test"}
		require.ErrorContains(t, repo.UpsertPendingUsage(ctx, pending), "READ COMMITTED")
		var stillUsable int
		require.NoError(t, tx.QueryRowContext(ctx, "SELECT 1").Scan(&stillUsable))
		require.Equal(t, 1, stillUsable)
	})
}

func TestPendingUsageHistoryPaginationFiltersAndLateSettlement(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newUsageLogRepositoryWithSQL(client, tx)
	user := mustCreateUser(t, client, &service.User{Email: "pending-history@example.invalid", Balance: 100})
	other := mustCreateUser(t, client, &service.User{Email: "pending-other@example.invalid", Balance: 55})
	key := mustCreateApiKey(t, client, &service.APIKey{UserID: user.ID, Key: "sk-pending-history", Name: "history"})
	otherKey := mustCreateApiKey(t, client, &service.APIKey{UserID: other.ID, Key: "sk-pending-other", Name: "other"})
	account := mustCreateAccount(t, client, &service.Account{Name: "pending-history"})
	group := mustCreateGroup(t, client, &service.Group{Name: "pending-history", Platform: service.PlatformGemini})
	now := time.Now().UTC().Truncate(time.Microsecond)
	pending := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID, RequestID: "client:pending-main", Model: "gemini-pending", RequestedModel: "gemini-display", Stream: true, InputTokens: 200, OutputTokens: 40, InputTokensSource: service.UsageTokenSourceUnreported, OutputTokensSource: service.UsageTokenSourceUpstream, CreatedAt: now}
	require.NoError(t, repo.UpsertPendingUsage(ctx, pending))
	firstID := pending.ID
	require.Negative(t, firstID)
	// Matching output types keep the large settled side eligible for index
	// scans/MergeAppend instead of forcing a whole-history coercion and sort.
	var actualTypeExprs, pendingTypeExprs []string
	for _, column := range strings.Split(usageLogSelectColumns, ", ") {
		actualTypeExprs = append(actualTypeExprs, "pg_typeof((NULL::usage_logs)."+column+")::text")
		pendingTypeExprs = append(pendingTypeExprs, "pg_typeof(projected."+column+")::text")
	}
	var actualTypes, pendingTypes pq.StringArray
	typeQuery := "SELECT ARRAY[" + strings.Join(actualTypeExprs, ",") + "], ARRAY[" + strings.Join(pendingTypeExprs, ",") + "] FROM (SELECT " + pendingUsageSelectColumns() + " FROM usage_pending_records p WHERE p.id = $1) projected"
	require.NoError(t, scanSingleRow(ctx, tx, typeQuery, []any{-firstID}, &actualTypes, &pendingTypes))
	pendingExpressions := strings.Split(pendingUsageSelectColumns(), ", ")
	for i, column := range strings.Split(usageLogSelectColumns, ", ") {
		if !strings.HasPrefix(pendingExpressions[i], "NULL AS ") { // NULL targets are resolved by UNION using the actual branch type.
			require.Equal(t, actualTypes[i], pendingTypes[i], column)
		}
	}
	// Repeat persistence is idempotent; a stale estimate cannot overwrite known measurements.
	pending.OutputTokensSource, pending.OutputTokens = service.UsageTokenSourceEstimated, 9999
	pending.CreatedAt = now.Add(time.Hour)
	require.NoError(t, repo.UpsertPendingUsage(ctx, pending))
	require.Equal(t, firstID, pending.ID)
	second := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID, RequestID: "client:pending-second", Model: "gemini-second", CreatedAt: now.Add(-time.Minute)}
	require.NoError(t, repo.UpsertPendingUsage(ctx, second))
	foreign := &service.UsageLog{UserID: other.ID, APIKeyID: otherKey.ID, AccountID: account.ID, GroupID: &group.ID, RequestID: "client:pending-foreign", Model: "gemini-pending", CreatedAt: now}
	require.NoError(t, repo.UpsertPendingUsage(ctx, foreign))
	// A pending write is not a charge, usage row, outbox entry, or billing reservation.
	for _, table := range []string{"usage_logs", "usage_billing_dedup", "usage_persistence_outbox"} {
		var count int64
		require.NoError(t, scanSingleRow(ctx, tx, "SELECT COUNT(*) FROM "+table+" WHERE api_key_id = $1", []any{key.ID}, &count))
		require.Zero(t, count, table)
	}
	var balance, quota float64
	require.NoError(t, scanSingleRow(ctx, tx, "SELECT balance FROM users WHERE id = $1", []any{user.ID}, &balance))
	require.NoError(t, scanSingleRow(ctx, tx, "SELECT quota_used FROM api_keys WHERE id = $1", []any{key.ID}, &quota))
	require.Equal(t, 100.0, balance)
	require.Zero(t, quota)
	settled := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID, RequestID: "client:already-settled", Model: "gemini-settled", InputTokens: 10, OutputTokens: 2, ActualCost: 0.12, CreatedAt: now.Add(-2 * time.Minute)}
	_, err := repo.Create(ctx, settled)
	require.NoError(t, err)
	filter := usagestats.UsageLogFilters{UserID: user.ID, IncludePending: true, ExactTotal: true}
	params := pagination.PaginationParams{Page: 1, PageSize: 2, SortBy: "created_at", SortOrder: "desc"}
	logs, page, err := repo.ListWithFilters(ctx, params, filter)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	require.Len(t, logs, 2)
	require.Equal(t, firstID, logs[0].ID)
	require.Equal(t, second.ID, logs[1].ID)
	require.Equal(t, service.UsageBillingStatusPendingReview, logs[0].BillingStatus)
	require.Equal(t, service.UsageTokenSourceUnreported, logs[0].InputTokensSource)
	require.Zero(t, logs[0].InputTokens)
	require.Equal(t, 40, logs[0].OutputTokens)
	require.Equal(t, service.UsageTokenSourceUpstream, logs[0].OutputTokensSource)
	require.Equal(t, now, logs[0].CreatedAt.UTC())
	require.Zero(t, logs[0].ActualCost)
	require.NotNil(t, logs[0].User)
	params.Page = 2
	logs, page, err = repo.ListWithFilters(ctx, params, filter)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	require.Len(t, logs, 1)
	require.Equal(t, settled.ID, logs[0].ID)
	params.Page, params.PageSize = 1, 100
	start, end := now.Add(-time.Second), now.Add(time.Second)
	for name, f := range map[string]usagestats.UsageLogFilters{
		"user ownership":    {UserID: user.ID, Model: "gemini-pending"},
		"key":               {APIKeyID: key.ID, Model: "gemini-pending"},
		"account and group": {AccountID: account.ID, GroupID: group.ID, UserID: user.ID, Model: "gemini-pending"},
		"request":           {RequestID: "client:pending-main"},
		"date":              {UserID: user.ID, StartTime: &start, EndTime: &end},
	} {
		t.Run(name, func(t *testing.T) {
			f.IncludePending, f.ExactTotal = true, true
			got, pg, err := repo.ListWithFilters(ctx, params, f)
			require.NoError(t, err)
			require.EqualValues(t, 1, pg.Total)
			require.Len(t, got, 1)
			require.Equal(t, firstID, got[0].ID)
		})
	}
	detail, err := repo.GetByID(ctx, firstID)
	require.NoError(t, err)
	require.Equal(t, user.ID, detail.UserID, "existing handler ownership guard uses this identity")
	require.Equal(t, service.UsageBillingStatusPendingReview, detail.BillingStatus)
	_, err = repo.GetByID(ctx, math.MinInt64)
	require.ErrorIs(t, err, service.ErrUsageLogNotFound)
	stats, err := repo.GetStatsWithFilters(ctx, filter)
	require.NoError(t, err)
	require.EqualValues(t, 1, stats.TotalRequests, "stats only count actual usage logs")
	require.EqualValues(t, 10, stats.TotalInputTokens)
	// A later authoritative settlement is free to claim the real request/key identity.
	late := &service.UsageLog{UserID: user.ID, APIKeyID: key.ID, AccountID: account.ID, GroupID: &group.ID, RequestID: "client:pending-main", Model: "gemini-pending", InputTokens: 1234, OutputTokens: 40, ActualCost: 0.25, CreatedAt: now}
	inserted, err := repo.Create(ctx, late)
	require.NoError(t, err)
	require.True(t, inserted, "pending observation must never reserve the real settlement key")
	logs, page, err = repo.ListWithFilters(ctx, params, filter)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total, "settlement replaces, rather than duplicates, a pending row")
	require.Equal(t, late.ID, logs[0].ID)
	require.Equal(t, 1234, logs[0].InputTokens)
	_, err = repo.GetByID(ctx, firstID)
	require.ErrorIs(t, err, service.ErrUsageLogNotFound, "old pending detail cannot contradict settled history")
	// Even a delayed pending retry after real settlement stays hidden.
	require.NoError(t, repo.UpsertPendingUsage(ctx, pending))
	logs, page, err = repo.ListWithFilters(ctx, params, filter)
	require.NoError(t, err)
	require.EqualValues(t, 3, page.Total)
	for _, log := range logs {
		require.NotEqual(t, firstID, log.ID)
	}
	var resolved bool
	require.NoError(t, scanSingleRow(ctx, tx, "SELECT resolved_at IS NOT NULL FROM usage_pending_records WHERE id = $1", []any{-firstID}, &resolved))
	require.True(t, resolved, "actual usage insert atomically resolves the separate observation")
	require.NoError(t, repo.Delete(ctx, late.ID))
	// A late retry after actual-row purging preserves the durable resolution marker.
	require.NoError(t, repo.UpsertPendingUsage(ctx, pending))
	logs, page, err = repo.ListWithFilters(ctx, params, filter)
	require.NoError(t, err)
	require.EqualValues(t, 2, page.Total)
	for _, log := range logs {
		require.NotEqual(t, firstID, log.ID, "settled pending observations never resurrect after retention")
	}
	for _, table := range []string{"usage_billing_dedup", "usage_billing_dedup_archive"} {
		requestID := "client:already-billed-" + table
		_, err := tx.ExecContext(ctx, "INSERT INTO "+table+" (request_id, api_key_id, request_fingerprint, created_at) VALUES ($1, $2, 'test-fingerprint', NOW())", requestID, key.ID)
		require.NoError(t, err)
		alreadyBilled := *second
		alreadyBilled.ID, alreadyBilled.RequestID = 0, requestID
		require.NoError(t, repo.UpsertPendingUsage(ctx, &alreadyBilled))
		var count int64
		require.NoError(t, scanSingleRow(ctx, tx, "SELECT COUNT(*) FROM usage_pending_records WHERE request_id = $1", []any{requestID}, &count))
		require.Zero(t, count, "committed settlement prevents late pending creation")
	}
	// Exercise the actual batch INSERT SQL too: its trigger must resolve pending
	// observations just like the single insert used by durable-outbox replay.
	batchPending := *second
	batchPending.ID, batchPending.RequestID = 0, "client:batch-settled"
	require.NoError(t, repo.UpsertPendingUsage(ctx, &batchPending))
	batchLog := batchPending
	batchLog.ID = 0
	batchLog.InputTokens, batchLog.OutputTokens, batchLog.ActualCost = 120, 7, 0.05
	prepared := prepareUsageLogInsert(&batchLog)
	batchKey := usageLogBatchKey(batchLog.RequestID, batchLog.APIKeyID)
	query, args := buildUsageLogBatchInsertQuery([]string{batchKey}, map[string]usageLogInsertPrepared{batchKey: prepared})
	var payload []byte
	require.NoError(t, scanSingleRow(ctx, tx, query, args, &payload))
	require.NoError(t, scanSingleRow(ctx, tx, "SELECT resolved_at IS NOT NULL FROM usage_pending_records WHERE id = $1", []any{-batchPending.ID}, &resolved))
	require.True(t, resolved, "batch INSERT must execute the settlement resolution trigger")
	_, err = repo.GetByID(ctx, batchPending.ID)
	require.ErrorIs(t, err, service.ErrUsageLogNotFound)
}
