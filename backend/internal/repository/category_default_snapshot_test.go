package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

// Observe the actual transaction contract rather than matching implementation
// strings for BEGIN. The same wrapper also gates a real concurrent writer in
// the integration test after PostgreSQL has established its read snapshot.
type categoryDefaultSnapshotDriver struct {
	dialect.Driver
	options         *sql.TxOptions
	afterConfigRead func()
}

func (d *categoryDefaultSnapshotDriver) BeginTx(ctx context.Context, opts *sql.TxOptions) (dialect.Tx, error) {
	copy := *opts
	d.options = &copy
	tx, err := d.Driver.(interface {
		BeginTx(context.Context, *sql.TxOptions) (dialect.Tx, error)
	}).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	return &categoryDefaultSnapshotTx{Tx: tx, afterConfigRead: d.afterConfigRead}, nil
}

type categoryDefaultSnapshotTx struct {
	dialect.Tx
	afterConfigRead func()
}

func (tx *categoryDefaultSnapshotTx) Query(ctx context.Context, query string, args, value any) error {
	if err := tx.Tx.Query(ctx, query, args, value); err != nil {
		return err
	}
	tx.observeConfigRead(query)
	return nil
}

// Ent's raw SQL extension calls QueryContext instead of dialect.Tx.Query.
// Preserve that optional interface so both raw and generated reads execute on
// the wrapped transaction and the concurrent-writer gate sees the real query.
func (tx *categoryDefaultSnapshotTx) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	rows, err := tx.Tx.(interface {
		QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	}).QueryContext(ctx, query, args...)
	if err == nil {
		tx.observeConfigRead(query)
	}
	return rows, err
}

func (tx *categoryDefaultSnapshotTx) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return tx.Tx.(interface {
		ExecContext(context.Context, string, ...any) (sql.Result, error)
	}).ExecContext(ctx, query, args...)
}

func (tx *categoryDefaultSnapshotTx) observeConfigRead(query string) {
	if tx.afterConfigRead != nil && strings.HasPrefix(query, "SELECT c.group_id") {
		callback := tx.afterConfigRead
		tx.afterConfigRead = nil
		callback()
	}
}

func TestCategoryDefaultSnapshotUsesReadOnlyRepeatableReadWithoutHistory(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "current-config", true: "read-failure"}[fail], func(t *testing.T) {
			db, mock := newSQLMock(t)
			driver := &categoryDefaultSnapshotDriver{Driver: entsql.OpenDB(dialect.Postgres, db)}
			client := dbent.NewClient(dbent.Driver(driver))
			repo := NewCategoryDefaultGroupRepository(client, db)
			mock.ExpectBegin()
			mock.ExpectQuery("SELECT c.group_id").WithArgs("", int64(100)).WillReturnRows(sqlmock.NewRows([]string{"group_id", "category", "name", "status", "revision"}).AddRow(100, "gemini", "default", "active", 3))
			query := mock.ExpectQuery("SELECT source_group_id FROM category_default_group_sources").WithArgs(int64(100))
			if fail {
				query.WillReturnError(errors.New("source read unavailable"))
				mock.ExpectRollback()
			} else {
				query.WillReturnRows(sqlmock.NewRows([]string{"source_group_id"}))
				mock.ExpectQuery("SELECT public_model,source_group_id,source_model,fingerprint").WithArgs(int64(100)).WillReturnRows(sqlmock.NewRows([]string{"public_model", "source_group_id", "source_model", "fingerprint"}))
				// No history query or write is allowed on a normal snapshot read.
				mock.ExpectCommit()
			}
			snapshot, err := repo.Snapshot(context.Background(), service.CategoryDefaultSnapshotRequest{GroupID: 100})
			if fail {
				require.ErrorContains(t, err, "source read unavailable")
				require.Nil(t, snapshot)
			} else {
				require.NoError(t, err)
				require.Equal(t, int64(3), snapshot.Config.Revision)
			}
			require.Equal(t, &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true}, driver.options)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
