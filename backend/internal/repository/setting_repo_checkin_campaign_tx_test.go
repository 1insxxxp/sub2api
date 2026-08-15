//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func newCheckinCampaignTxSettingRepositoryTest(t *testing.T) (*settingRepository, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	t.Cleanup(func() { _ = client.Close() })
	return &settingRepository{client: client}, mock
}

func expectCheckinCampaignAdvisoryTxBegin(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_advisory_xact_lock").
		WithArgs(checkinCampaignConfigAdvisoryLockKey).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock"}).AddRow(nil))
}

func expectCheckinCampaignAdvisoryReadTxBegin(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT pg_advisory_xact_lock_shared").
		WithArgs(checkinCampaignConfigAdvisoryLockKey).
		WillReturnRows(sqlmock.NewRows([]string{"pg_advisory_xact_lock_shared"}).AddRow(nil))
}

func TestSettingRepositoryWithCheckinCampaignConfigTxCommitsAfterCallback(t *testing.T) {
	repo, mock := newCheckinCampaignTxSettingRepositoryTest(t)
	expectCheckinCampaignAdvisoryTxBegin(mock)
	mock.ExpectCommit()

	called := false
	err := repo.WithCheckinCampaignConfigTx(context.Background(), func(client *dbent.Client, txRepo service.SettingRepository) error {
		called = true
		require.NotSame(t, repo.client, client)
		require.NotSame(t, repo, txRepo)
		return nil
	})

	require.NoError(t, err)
	require.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingRepositoryWithCheckinCampaignConfigTxRollsBackSettingWriteFailure(t *testing.T) {
	repo, mock := newCheckinCampaignTxSettingRepositoryTest(t)
	sentinel := errors.New("setting bulk write failed")
	expectCheckinCampaignAdvisoryTxBegin(mock)
	mock.ExpectQuery(`INSERT INTO "settings"`).
		WithArgs("checkin.test", sqlmock.AnyArg(), "value").
		WillReturnError(sentinel)
	mock.ExpectRollback()

	err := repo.WithCheckinCampaignConfigTx(context.Background(), func(_ *dbent.Client, txRepo service.SettingRepository) error {
		return txRepo.SetMultiple(context.Background(), map[string]string{"checkin.test": "value"})
	})

	require.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingRepositoryWithCheckinCampaignConfigReadTxUsesSharedLockAndCommits(t *testing.T) {
	repo, mock := newCheckinCampaignTxSettingRepositoryTest(t)
	expectCheckinCampaignAdvisoryReadTxBegin(mock)
	mock.ExpectCommit()

	called := false
	err := repo.WithCheckinCampaignConfigReadTx(context.Background(), func(client *dbent.Client, txRepo service.SettingRepository) error {
		called = true
		require.NotSame(t, repo.client, client)
		require.NotSame(t, repo, txRepo)
		return nil
	})

	require.NoError(t, err)
	require.True(t, called)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestSettingRepositoryWithCheckinCampaignConfigReadTxRollsBackCallbackFailure(t *testing.T) {
	repo, mock := newCheckinCampaignTxSettingRepositoryTest(t)
	sentinel := errors.New("read callback failed")
	expectCheckinCampaignAdvisoryReadTxBegin(mock)
	mock.ExpectRollback()

	err := repo.WithCheckinCampaignConfigReadTx(context.Background(), func(*dbent.Client, service.SettingRepository) error {
		return sentinel
	})

	require.ErrorIs(t, err, sentinel)
	require.NoError(t, mock.ExpectationsWereMet())
}
