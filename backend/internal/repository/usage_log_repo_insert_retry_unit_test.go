//go:build unit

package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	gocache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/require"
)

// Exercise the actual database/sql context checks with virtual time: an expired
// batch context prevents even entering the driver for a subsequent single insert.
func TestUsageLogBestEffortRetry_BatchTimeoutGivesEachSingleFreshBudget(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls int
		var singleRequestIDs []string
		conn := &usageLogRetryConn{exec: func(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
			calls++
			if calls == 1 {
				require.Len(t, args, 2*len(usageLogInsertArgTypes), "duplicate requests must share one insert")
				<-ctx.Done()
				return nil, ctx.Err()
			}
			requireUsageLogRetryBudget(t, ctx)
			require.Len(t, args, len(usageLogInsertArgTypes))
			singleRequestIDs = append(singleRequestIDs, args[3].Value.(string))
			return driver.RowsAffected(1), nil
		}}
		db := sql.OpenDB(usageLogRetryConnector{conn: conn})
		defer db.Close()
		repo := usageLogRepository{bestEffortRecent: gocache.New(usageLogBestEffortRecentTTL, 0)}
		first := newUsageLogBestEffortRetryRequest("first")
		duplicate := newUsageLogBestEffortRetryRequest("first")
		second := newUsageLogBestEffortRetryRequest("second")

		started := time.Now()
		repo.flushBestEffortBatch(db, []usageLogBestEffortRequest{first, duplicate, second})

		require.NoError(t, <-first.resultCh, "a timed-out batch must not poison its single retry")
		require.NoError(t, <-duplicate.resultCh, "all waiters for the same key receive the retry acknowledgement")
		require.NoError(t, <-second.resultCh)
		require.Equal(t, []string{"first", "second"}, singleRequestIDs)
		require.Equal(t, 3, calls)
		require.Equal(t, 10*time.Second, time.Since(started))
		_, cachedFirst := repo.bestEffortRecent.Get(usageLogBatchKey("first", first.apiKeyID))
		_, cachedSecond := repo.bestEffortRecent.Get(usageLogBatchKey("second", second.apiKeyID))
		require.True(t, cachedFirst)
		require.True(t, cachedSecond)
	})
}

func TestUsageLogBestEffortRetry_SingleTimeoutDoesNotPoisonNextKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls int
		conn := &usageLogRetryConn{exec: func(ctx context.Context, _ []driver.NamedValue) (driver.Result, error) {
			calls++
			switch calls {
			case 1:
				return nil, errors.New("batch rejected")
			case 2:
				requireUsageLogRetryBudget(t, ctx)
				<-ctx.Done()
				return nil, ctx.Err()
			default:
				requireUsageLogRetryBudget(t, ctx)
				return driver.RowsAffected(1), nil
			}
		}}
		db := sql.OpenDB(usageLogRetryConnector{conn: conn})
		defer db.Close()
		repo := usageLogRepository{bestEffortRecent: gocache.New(usageLogBestEffortRecentTTL, 0)}
		first := newUsageLogBestEffortRetryRequest("times-out")
		second := newUsageLogBestEffortRetryRequest("recovers")

		repo.flushBestEffortBatch(db, []usageLogBestEffortRequest{first, second})

		require.ErrorIs(t, <-first.resultCh, context.DeadlineExceeded)
		require.NoError(t, <-second.resultCh, "one slow key must not consume the next key's retry budget")
		require.Equal(t, 3, calls)
		_, cachedFirst := repo.bestEffortRecent.Get(usageLogBatchKey("times-out", first.apiKeyID))
		_, cachedSecond := repo.bestEffortRecent.Get(usageLogBatchKey("recovers", second.apiKeyID))
		require.False(t, cachedFirst, "failed writes must remain eligible for synchronous recovery")
		require.True(t, cachedSecond)
	})
}

func TestUsageLogBestEffortRetry_PermanentErrorIsBoundedAndAcknowledged(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		writeErr := errors.New("invalid usage row")
		var calls int
		conn := &usageLogRetryConn{exec: func(ctx context.Context, _ []driver.NamedValue) (driver.Result, error) {
			calls++
			requireUsageLogRetryBudget(t, ctx)
			return nil, writeErr
		}}
		db := sql.OpenDB(usageLogRetryConnector{conn: conn})
		defer db.Close()
		repo := usageLogRepository{}
		first := newUsageLogBestEffortRetryRequest("first")
		duplicate := newUsageLogBestEffortRetryRequest("first")
		second := newUsageLogBestEffortRetryRequest("second")

		repo.flushBestEffortBatch(db, []usageLogBestEffortRequest{first, duplicate, second})

		require.ErrorIs(t, <-first.resultCh, writeErr)
		require.ErrorIs(t, <-duplicate.resultCh, writeErr)
		require.ErrorIs(t, <-second.resultCh, writeErr)
		require.Equal(t, 3, calls, "one batch plus one fallback per unique key; no unbounded retry")
	})
}

func TestUsageLogCreateRetry_BatchTimeoutLeavesSynchronousFallbackUsable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var calls int
		createdAt := time.Now().UTC()
		conn := &usageLogRetryConn{query: func(ctx context.Context, _ []driver.NamedValue) (driver.Rows, error) {
			calls++
			if calls == 1 {
				<-ctx.Done()
				return nil, ctx.Err()
			}
			requireUsageLogRetryBudget(t, ctx)
			return &usageLogRetryRows{values: []driver.Value{int64(calls), createdAt}}, nil
		}}
		db := sql.OpenDB(usageLogRetryConnector{conn: conn})
		defer db.Close()
		repo := usageLogRepository{}
		first := newUsageLogCreateRetryRequest("first")
		second := newUsageLogCreateRetryRequest("second")

		repo.flushCreateBatch(db, []usageLogCreateRequest{first, second})

		for _, req := range []usageLogCreateRequest{first, second} {
			res := <-req.resultCh
			require.NoError(t, res.err)
			require.True(t, res.inserted)
			require.NotZero(t, req.log.ID)
			require.Equal(t, usageLogCreateStateCompleted, req.shared.state.Load())
		}
		require.Equal(t, 3, calls)
	})
}

func requireUsageLogRetryBudget(t *testing.T, ctx context.Context) {
	t.Helper()
	require.NoError(t, ctx.Err())
	deadline, ok := ctx.Deadline()
	require.True(t, ok, "each database attempt must remain bounded")
	require.Equal(t, 10*time.Second, time.Until(deadline), "each attempt needs its own unchanged 10-second budget")
}

func newUsageLogBestEffortRetryRequest(requestID string) usageLogBestEffortRequest {
	log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, RequestID: requestID, Model: "gemini-test"}
	return usageLogBestEffortRequest{prepared: prepareUsageLogInsert(log), apiKeyID: log.APIKeyID, resultCh: make(chan error, 1)}
}

func newUsageLogCreateRetryRequest(requestID string) usageLogCreateRequest {
	log := &service.UsageLog{UserID: 1, APIKeyID: 2, AccountID: 3, RequestID: requestID, Model: "gemini-test"}
	return usageLogCreateRequest{log: log, prepared: prepareUsageLogInsert(log), shared: &usageLogCreateShared{}, resultCh: make(chan usageLogCreateResult, 1)}
}

type usageLogRetryConnector struct{ conn *usageLogRetryConn }

func (c usageLogRetryConnector) Connect(context.Context) (driver.Conn, error) { return c.conn, nil }
func (usageLogRetryConnector) Driver() driver.Driver                          { return usageLogRetryDriver{} }

type usageLogRetryDriver struct{}

func (usageLogRetryDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use connector")
}

type usageLogRetryConn struct {
	exec  func(context.Context, []driver.NamedValue) (driver.Result, error)
	query func(context.Context, []driver.NamedValue) (driver.Rows, error)
}

func (*usageLogRetryConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepared statement")
}
func (*usageLogRetryConn) Close() error { return nil }
func (*usageLogRetryConn) Begin() (driver.Tx, error) {
	return nil, errors.New("unexpected transaction")
}
func (*usageLogRetryConn) CheckNamedValue(*driver.NamedValue) error { return nil }
func (c *usageLogRetryConn) ExecContext(ctx context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
	return c.exec(ctx, args)
}
func (c *usageLogRetryConn) QueryContext(ctx context.Context, _ string, args []driver.NamedValue) (driver.Rows, error) {
	return c.query(ctx, args)
}

type usageLogRetryRows struct{ values []driver.Value }

func (*usageLogRetryRows) Columns() []string { return []string{"id", "created_at"} }
func (*usageLogRetryRows) Close() error      { return nil }
func (r *usageLogRetryRows) Next(dest []driver.Value) error {
	if r.values == nil {
		return io.EOF
	}
	copy(dest, r.values)
	r.values = nil
	return nil
}
