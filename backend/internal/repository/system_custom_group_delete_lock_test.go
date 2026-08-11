//go:build unit

package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestSystemCustomGroupDeleteAcquiresReferenceAdvisoryLockBeforeGroupRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &systemCustomGroupRepository{client: client}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
	mock.ExpectQuery("SELECT system_custom_routing_enabled").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"system_custom_routing_enabled"}).AddRow(false))
	mock.ExpectRollback()

	_, err = repo.DeleteWithImpact(context.Background(), 42)

	require.ErrorIs(t, err, service.ErrSystemCustomGroupNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSystemCustomGroupUpdateAcquiresReferenceAdvisoryLockBeforeLiveGroupRow(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	repo := &systemCustomGroupRepository{client: client}

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
	mock.ExpectQuery("SELECT system_custom_routing_enabled").
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"system_custom_routing_enabled"}))
	mock.ExpectRollback()

	err = repo.Update(context.Background(), &service.Group{
		ID: 42, Name: "custom group", Platform: service.PlatformComposite,
		SubscriptionType: service.SubscriptionTypeSubscription, Status: service.StatusActive,
	}, nil)

	require.ErrorIs(t, err, service.ErrSystemCustomGroupNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
