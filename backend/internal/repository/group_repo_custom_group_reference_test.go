//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestGroupRepositoryCountCustomGroupModelReferencesIgnoresDeletedCustomGroups(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*JOIN user_custom_groups AS custom_group.*model\.source_group_id = \$1.*custom_group\.deleted_at IS NULL`).
		WithArgs(int64(42)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))

	repo := &groupRepository{sql: db}
	count, err := repo.CountCustomGroupModelReferences(context.Background(), 42)

	require.NoError(t, err)
	require.Equal(t, 7, count)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGroupRepositoryCountCustomGroupModelReferencesPropagatesQueryError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	queryErr := errors.New("database unavailable")
	mock.ExpectQuery(`(?s)SELECT COUNT\(\*\).*user_custom_group_models`).
		WithArgs(int64(42)).
		WillReturnError(queryErr)

	repo := &groupRepository{sql: db}
	count, err := repo.CountCustomGroupModelReferences(context.Background(), 42)

	require.Zero(t, count)
	require.ErrorIs(t, err, queryErr)
	require.NoError(t, mock.ExpectationsWereMet())
}
