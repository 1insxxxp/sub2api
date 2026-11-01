//go:build integration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/lib/pq"
	"github.com/stretchr/testify/require"
)

func TestRequestLogRetention_CandidateSelectionDoesNotBlockCurrentInserts(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, true)
	db := openRetentionTestDB(t, ctx, schema)
	_, err := db.ExecContext(ctx, `
		CREATE TABLE usage_logs_history PARTITION OF usage_logs
		FOR VALUES FROM ('2020-01-01') TO ('2021-01-01');
		INSERT INTO groups (id) VALUES (10);
		INSERT INTO users (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		VALUES (1, 1, 10, 1, '2020-01-02');
		UPDATE usage_group_rollup_state
		SET closed_before = (CURRENT_TIMESTAMP AT TIME ZONE 'UTC')::date;
	`)
	require.NoError(t, err)

	blocker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	var blockerPID int
	require.NoError(t, blocker.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&blockerPID))
	_, err = blocker.ExecContext(ctx, "LOCK TABLE usage_logs_history IN ACCESS EXCLUSIVE MODE")
	require.NoError(t, err)

	cleanupResult := make(chan error, 1)
	go func() {
		_, cleanupErr := cleanupUsageLogsBatchWithRollupInvalidation(ctx, db, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		cleanupResult <- cleanupErr
	}()
	var cleanupPID int
	require.Eventually(t, func() bool {
		return db.QueryRowContext(ctx, `
			SELECT pid FROM pg_stat_activity
			WHERE $1 = ANY(pg_blocking_pids(pid)) AND query LIKE '%usage_logs%'
		`, blockerPID).Scan(&cleanupPID) == nil
	}, 2*time.Second, 10*time.Millisecond, "cleanup must be waiting on the historical partition")

	insertCtx, insertCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer insertCancel()
	_, insertErr := db.ExecContext(insertCtx, `
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		VALUES (2, 1, 10, 1, CURRENT_TIMESTAMP)
	`)
	// Release the historical lock before assertions so failure cannot strand a worker.
	require.NoError(t, blocker.Rollback())
	require.NoError(t, <-cleanupResult)
	require.NoError(t, insertErr, "slow expired-row selection must not hold the watermark lock needed by current usage INSERTs")

	var closedBefore string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT closed_before::text FROM usage_group_rollup_state WHERE id = 1").Scan(&closedBefore))
	require.Equal(t, "2020-01-02", closedBefore, "actual deletes must still invalidate the published rollup")
}

func TestRequestLogRetention_EmptyBatchDoesNotWaitForWatermark(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	db := openRetentionTestDB(t, ctx, schema)
	blocker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	require.NoError(t, lockGroupUsageRollupState(ctx, blocker))
	cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cleanupCancel()
	affected, cleanupErr := cleanupUsageLogsBatchWithRollupInvalidation(cleanupCtx, db, time.Now())
	require.NoError(t, blocker.Rollback())
	require.NoError(t, cleanupErr, "no expired rows must return without acquiring the shared watermark")
	require.Zero(t, affected)
}

func TestRequestLogRetention_ReleasesSelectionLocksBeforeWaitingForWatermark(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, true)
	db := openRetentionTestDB(t, ctx, schema)
	seedRetentionCandidate(t, ctx, db)
	blocker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	require.NoError(t, lockGroupUsageRollupState(ctx, blocker))
	var blockerPID int
	require.NoError(t, blocker.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&blockerPID))
	cleanupResult := make(chan error, 1)
	go func() {
		_, cleanupErr := cleanupUsageLogsBatchWithRollupInvalidation(ctx, db, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
		cleanupResult <- cleanupErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		return db.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE $1 = ANY(pg_blocking_pids(pid)) AND query LIKE '%FOR UPDATE%')
		`, blockerPID).Scan(&waiting) == nil && waiting
	}, 2*time.Second, 10*time.Millisecond)
	// Existing partition removal holds state before taking ACCESS EXCLUSIVE.
	// Candidate reads must have released their relation locks by this point.
	_, lockErr := blocker.ExecContext(ctx, "LOCK TABLE usage_logs IN ACCESS EXCLUSIVE MODE NOWAIT")
	require.NoError(t, blocker.Rollback())
	require.NoError(t, <-cleanupResult)
	require.NoError(t, lockErr, "candidate selection must not invert the state -> partition lock order")
}

func TestRequestLogRetention_WatermarkUsesOnlyActualDeletedRows(t *testing.T) {
	ctx := context.Background()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	db := openRetentionTestDB(t, ctx, schema)
	seedRetentionCandidate(t, ctx, db)
	_, err := db.ExecContext(ctx, `
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		VALUES (2, 1, 10, 1, '2020-02-02');
	`)
	require.NoError(t, err)
	cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	candidates, err := selectUsageLogsCleanupCandidates(ctx, db, cutoff)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		UPDATE usage_logs SET created_at = CURRENT_TIMESTAMP WHERE id = 1;
		UPDATE usage_group_rollup_state SET closed_before = DATE '2021-01-01';
	`)
	require.NoError(t, err)
	affected, err := cleanupUsageLogsCandidatesWithRollupInvalidation(ctx, db, cutoff, candidates)
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	var closedBefore string
	require.NoError(t, db.QueryRowContext(ctx, "SELECT closed_before::text FROM usage_group_rollup_state WHERE id = 1").Scan(&closedBefore))
	require.Equal(t, "2020-02-02", closedBefore)
}

func TestRequestLogRetention_ContinuesAcrossBoundedBatches(t *testing.T) {
	ctx := context.Background()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	db := openRetentionTestDB(t, ctx, schema)
	_, err := db.ExecContext(ctx, `
		INSERT INTO groups (id) VALUES (10);
		INSERT INTO users (id) VALUES (1);
	`)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		SELECT id, 1, 10, 1, TIMESTAMPTZ '2020-01-02' FROM generate_series(1, $1::int) id;
	`, usageLogsCleanupBatchSize+3)
	require.NoError(t, err)
	repo := newDashboardAggregationRepositoryWithSQL(db)
	require.NoError(t, repo.cleanupUsageLogsBatches(ctx, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)))
	var remaining int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs").Scan(&remaining))
	require.Zero(t, remaining)
}

func TestRequestLogRetention_FencedDeleteUsesBoundedTupleLookups(t *testing.T) {
	ctx := context.Background()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	db := openRetentionTestDB(t, ctx, schema)
	_, err := db.ExecContext(ctx, `
		INSERT INTO groups (id) VALUES (10);
		INSERT INTO users (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		SELECT id, 1, 10, 1, TIMESTAMPTZ '2020-01-02' FROM generate_series(1, 100000) id;
		ANALYZE usage_logs;
	`)
	require.NoError(t, err)
	cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	candidates, err := selectUsageLogsCleanupCandidates(ctx, db, cutoff)
	require.NoError(t, err)
	tableOIDs, ids := make([]int64, len(candidates)), make([]int64, len(candidates))
	ctids, xmins := make([]string, len(candidates)), make([]string, len(candidates))
	createdAts := make([]time.Time, len(candidates))
	for i, candidate := range candidates {
		tableOIDs[i], ctids[i], xmins[i], ids[i], createdAts[i] = candidate.tableOID, candidate.ctid, candidate.xmin, candidate.id, candidate.createdAt
	}
	var plan string
	require.NoError(t, db.QueryRowContext(ctx, "EXPLAIN (FORMAT JSON) "+usageLogsCleanupDeleteQuery,
		cutoff, pq.Array(tableOIDs), pq.Array(ctids), pq.Array(xmins), pq.Array(ids), pq.Array(createdAts)).Scan(&plan))
	require.NotContains(t, plan, `"Node Type": "Seq Scan"`, "fenced deletion must not scan all usage rows")
	require.Contains(t, plan, `"Node Type": "Tid Scan"`)
}

func TestRequestLogRetention_RevalidatesSelectedTupleVersion(t *testing.T) {
	for _, partitioned := range []bool{false, true} {
		for _, change := range []string{"updated_recent", "updated_expired", "deleted", "reused_tid"} {
			t.Run(fmt.Sprintf("partitioned=%t/%s", partitioned, change), func(t *testing.T) {
				ctx := context.Background()
				schema := createGroupUsageRollupTriggerTestSchema(t, ctx, partitioned)
				db := openRetentionTestDB(t, ctx, schema)
				seedRetentionCandidate(t, ctx, db)
				cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
				candidates, err := selectUsageLogsCleanupCandidates(ctx, db, cutoff)
				require.NoError(t, err)
				require.Len(t, candidates, 1)
				switch change {
				case "updated_recent":
					_, err = db.ExecContext(ctx, "UPDATE usage_logs SET created_at = CURRENT_TIMESTAMP WHERE id = 1")
				case "updated_expired":
					_, err = db.ExecContext(ctx, "UPDATE usage_logs SET actual_cost = 2 WHERE id = 1")
				case "deleted":
					_, err = db.ExecContext(ctx, "DELETE FROM usage_logs WHERE id = 1")
				case "reused_tid":
					_, err = db.ExecContext(ctx, `
						TRUNCATE usage_logs;
						INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
						VALUES (1, 1, 10, 1, '2020-01-02');
					`)
					require.NoError(t, err)
					var ctid, xmin string
					require.NoError(t, db.QueryRowContext(ctx, "SELECT ctid::text, xmin::text FROM usage_logs WHERE id = 1").Scan(&ctid, &xmin))
					require.Equal(t, candidates[0].ctid, ctid, "fixture must reuse the physical tuple location")
					require.NotEqual(t, candidates[0].xmin, xmin, "replacement must have a distinct tuple version")
				}
				require.NoError(t, err)
				_, err = db.ExecContext(ctx, "UPDATE usage_group_rollup_state SET closed_before = DATE '2021-01-01' WHERE id = 1")
				require.NoError(t, err)
				affected, err := cleanupUsageLogsCandidatesWithRollupInvalidation(ctx, db, cutoff, candidates)
				require.NoError(t, err)
				require.Zero(t, affected, "a changed candidate must wait for a new selection")
				var remaining int
				require.NoError(t, db.QueryRowContext(ctx, "SELECT COUNT(*) FROM usage_logs").Scan(&remaining))
				if change == "deleted" {
					require.Zero(t, remaining)
				} else {
					require.Equal(t, 1, remaining)
				}
				var closedBefore string
				require.NoError(t, db.QueryRowContext(ctx, "SELECT closed_before::text FROM usage_group_rollup_state WHERE id = 1").Scan(&closedBefore))
				require.Equal(t, "2021-01-01", closedBefore, "stale candidates must not invalidate the watermark")
			})
		}
	}
}

func TestRequestLogRetention_DeletesOnlySelectedPartitionWithCollidingIdentity(t *testing.T) {
	ctx := context.Background()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, true)
	db := openRetentionTestDB(t, ctx, schema)
	_, err := db.ExecContext(ctx, `
		CREATE TABLE usage_logs_history PARTITION OF usage_logs
		FOR VALUES FROM ('2020-01-01') TO ('2021-01-01');
	`)
	require.NoError(t, err)
	seedRetentionCandidate(t, ctx, db)
	_, err = db.ExecContext(ctx, `
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		VALUES (1, 1, 10, 2, CURRENT_TIMESTAMP);
	`)
	require.NoError(t, err)
	var collisions bool
	require.NoError(t, db.QueryRowContext(ctx, `
		SELECT a.ctid = b.ctid AND a.tableoid <> b.tableoid
		FROM usage_logs_history a, usage_logs_default b
	`).Scan(&collisions))
	require.True(t, collisions, "fixture must have duplicate logical IDs and physical TIDs across partitions")
	affected, err := cleanupUsageLogsBatchWithRollupInvalidation(ctx, db, time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.EqualValues(t, 1, affected)
	var cost float64
	require.NoError(t, db.QueryRowContext(ctx, "SELECT actual_cost FROM usage_logs").Scan(&cost))
	require.Equal(t, 2.0, cost)
}

func TestRequestLogRetention_SkipsCandidateHeldByWriterWaitingForWatermark(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	schema := createGroupUsageRollupTriggerTestSchema(t, ctx, false)
	db := openRetentionTestDB(t, ctx, schema)
	seedRetentionCandidate(t, ctx, db)
	cutoff := time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)
	candidates, err := selectUsageLogsCleanupCandidates(ctx, db, cutoff)
	require.NoError(t, err)
	blocker, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = blocker.Rollback() }()
	require.NoError(t, lockGroupUsageRollupState(ctx, blocker))
	var blockerPID int
	require.NoError(t, blocker.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&blockerPID))
	cleanupResult := make(chan error, 1)
	var affected int64
	go func() {
		var cleanupErr error
		affected, cleanupErr = cleanupUsageLogsCandidatesWithRollupInvalidation(ctx, db, cutoff, candidates)
		cleanupResult <- cleanupErr
	}()
	require.Eventually(t, func() bool {
		var waiting bool
		return db.QueryRowContext(ctx, `
			SELECT EXISTS (SELECT 1 FROM pg_stat_activity
			WHERE $1 = ANY(pg_blocking_pids(pid)) AND query LIKE '%FOR UPDATE%')
		`, blockerPID).Scan(&waiting) == nil && waiting
	}, 2*time.Second, 10*time.Millisecond)

	// Queue a writer after cleanup. It holds the usage tuple while its AFTER
	// trigger waits for state; cleanup must skip the tuple when state is freed.
	writer, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = writer.Rollback() }()
	var writerPID int
	require.NoError(t, writer.QueryRowContext(ctx, "SELECT pg_backend_pid()").Scan(&writerPID))
	writerResult := make(chan error, 1)
	go func() {
		_, writerErr := writer.ExecContext(ctx, "UPDATE usage_logs SET actual_cost = 2 WHERE id = 1")
		writerResult <- writerErr
	}()
	blocked, err := waitForGroupUsageRollupStateLock(ctx, writerPID, writerResult)
	require.NoError(t, err)
	require.True(t, blocked)
	require.NoError(t, blocker.Rollback())
	require.NoError(t, <-cleanupResult, "cleanup must not hold the watermark while waiting for a writer's tuple")
	require.Zero(t, affected)
	require.NoError(t, <-writerResult)
	require.NoError(t, writer.Commit())
}

func seedRetentionCandidate(t *testing.T, ctx context.Context, db *sql.DB) {
	t.Helper()
	_, err := db.ExecContext(ctx, `
		INSERT INTO groups (id) VALUES (10);
		INSERT INTO users (id) VALUES (1);
		INSERT INTO usage_logs (id, user_id, group_id, actual_cost, created_at)
		VALUES (1, 1, 10, 1, '2020-01-02');
		UPDATE usage_group_rollup_state SET closed_before = DATE '2021-01-01' WHERE id = 1;
	`)
	require.NoError(t, err)
}

func openRetentionTestDB(t *testing.T, ctx context.Context, schema string) *sql.DB {
	t.Helper()
	dsn, err := url.Parse(integrationPostgresDSN)
	require.NoError(t, err)
	query := dsn.Query()
	query.Set("search_path", pq.QuoteIdentifier(schema))
	query.Set("application_name", fmt.Sprintf("retention-test-%s", schema))
	dsn.RawQuery = query.Encode()
	db, err := sql.Open("postgres", dsn.String())
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.PingContext(ctx))
	return db
}
