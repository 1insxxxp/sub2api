//go:build unit

package groupref

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestLockGroupReferenceWritesUsesDedicatedNamespaceAndSortedUniqueIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	tx, err := client.Tx(context.Background())
	require.NoError(t, err)
	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(groupReferenceLockNamespace, groupReferenceLockKey(2)).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(groupReferenceLockNamespace, groupReferenceLockKey(9)).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
	mock.ExpectRollback()

	require.NoError(t, LockGroupReferenceWrites(context.Background(), tx, 9, 2, 9, 0, -1))
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestLockGroupReferenceWritesRequiresExplicitTransaction(t *testing.T) {
	err := LockGroupReferenceWrites(context.Background(), nil, 1)
	require.Error(t, err)
	require.Contains(t, err.Error(), "transaction")
}

func TestLockGroupReferenceWritesIsNoopForNonPostgresTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.SQLite, db)))
	t.Cleanup(func() { _ = client.Close() })

	mock.ExpectBegin()
	tx, err := client.Tx(context.Background())
	require.NoError(t, err)
	mock.ExpectRollback()

	require.NoError(t, LockGroupReferenceWrites(context.Background(), tx, 1))
	require.NoError(t, tx.Rollback())
	require.NoError(t, mock.ExpectationsWereMet())
}
