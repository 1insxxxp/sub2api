package repository

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/Wei-Shaw/sub2api/migrations"
	"github.com/stretchr/testify/require"
)

func TestGroupAccountUserExclusionMigration(t *testing.T) {
	data, err := migrations.FS.ReadFile("250_group_account_user_exclusions.sql")
	require.NoError(t, err)
	for _, clause := range []string{"UNIQUE (group_id, account_id, user_id)", "REFERENCES groups(id)", "REFERENCES accounts(id)", "REFERENCES users(id)", "(user_id, group_id)"} {
		require.Contains(t, string(data), clause)
	}
}

func TestGroupAccountUserExclusionLoadAllGroupsAndFailClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery("SELECT group_id, account_id FROM group_account_user_exclusions").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"group_id", "account_id"}).AddRow(10, 21).AddRow(10, 22).AddRow(11, 23))
	got, err := loadGroupAccountExclusions(context.Background(), db, 7)
	require.NoError(t, err)
	require.Equal(t, service.GroupAccountExclusions{10: {21, 22}, 11: {23}}, got)
	mock.ExpectQuery("SELECT group_id, account_id FROM group_account_user_exclusions").WithArgs(int64(7)).WillReturnError(errors.New("db unavailable"))
	got, err = loadGroupAccountExclusions(context.Background(), db, 7)
	require.Error(t, err)
	require.Nil(t, got)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGroupAccountUserExclusionAddIsIdempotentAndInvalidatesInTransaction(t *testing.T) {
	for _, outboxErr := range []error{nil, errors.New("outbox unavailable")} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		repo := NewGroupAccountUserExclusionRepository(db)
		now := time.Now()
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT id FROM groups").WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
		mock.ExpectQuery("(?s)SELECT a.name.*FROM accounts").WithArgs(int64(21), int64(10)).WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("private-upstream"))
		mock.ExpectQuery("(?s)SELECT email.*FROM users").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"email", "username", "notes"}).AddRow("u@example.com", "User", "admin note"))
		mock.ExpectQuery("(?s)INSERT INTO group_account_user_exclusions.*ON CONFLICT.*RETURNING").WithArgs(int64(10), int64(21), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(1, now))
		ex := mock.ExpectExec("(?s)INSERT INTO auth_cache_invalidation_outbox.*api_keys.*user_id = \\$1").WithArgs(int64(7))
		if outboxErr != nil {
			ex.WillReturnError(outboxErr)
			mock.ExpectRollback()
		} else {
			ex.WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectCommit()
		}
		got, err := repo.Add(context.Background(), 10, 21, 7)
		if outboxErr != nil {
			require.ErrorIs(t, err, outboxErr)
			require.Nil(t, got)
		} else {
			require.NoError(t, err)
			require.Equal(t, int64(1), got.ID)
			require.Equal(t, "admin note", got.Notes)
		}
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	}
}

func TestGroupAccountUserExclusionAddRejectsMissingMembershipOrInactiveUser(t *testing.T) {
	for _, missing := range []string{"account", "user"} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectBegin()
		mock.ExpectQuery("SELECT id FROM groups").WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(10))
		account := mock.ExpectQuery("(?s)SELECT a.name.*FROM accounts").WithArgs(int64(21), int64(10))
		if missing == "account" {
			account.WillReturnError(sql.ErrNoRows)
		} else {
			account.WillReturnRows(sqlmock.NewRows([]string{"name"}).AddRow("upstream"))
			mock.ExpectQuery("(?s)SELECT email.*FROM users").WithArgs(int64(7)).WillReturnError(sql.ErrNoRows)
		}
		mock.ExpectRollback()
		got, err := NewGroupAccountUserExclusionRepository(db).Add(context.Background(), 10, 21, 7)
		require.Error(t, err)
		require.Nil(t, got)
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	}
}

func TestGroupAccountUserExclusionAuthenticationRejectsUnavailableRules(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "unavailable-exclusions@example.com")
	key := &service.APIKey{UserID: user.ID, Key: "sk-unavailable-exclusions", Name: "auth", Status: service.StatusActive}
	require.NoError(t, repo.Create(ctx, key))
	_, err := repo.sql.ExecContext(ctx, `DROP TABLE group_account_user_exclusions`)
	require.NoError(t, err)
	for _, lookup := range []func() (*service.APIKey, error){func() (*service.APIKey, error) { return repo.GetByID(ctx, key.ID) }, func() (*service.APIKey, error) { return repo.GetByKey(ctx, key.Key) }, func() (*service.APIKey, error) { return repo.GetByKeyForAuth(ctx, key.Key) }} {
		result, err := lookup()
		require.Error(t, err)
		require.Nil(t, result, "cannot authenticate without access policy")
	}
}

func TestGroupAccountUserExclusionRemovalInvalidatesAtomically(t *testing.T) {
	for _, outboxErr := range []error{nil, errors.New("outbox failed")} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		mock.ExpectBegin()
		mock.ExpectQuery(`(?s)DELETE FROM group_account_user_exclusions.*id = \$1 AND group_id = \$2.*RETURNING user_id`).WithArgs(int64(5), int64(10)).WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(7))
		ex := mock.ExpectExec(`(?s)INSERT INTO auth_cache_invalidation_outbox.*api_keys.*user_id = \$1`).WithArgs(int64(7))
		if outboxErr != nil {
			ex.WillReturnError(outboxErr)
			mock.ExpectRollback()
		} else {
			ex.WillReturnResult(sqlmock.NewResult(1, 1))
			mock.ExpectCommit()
		}
		userID, err := NewGroupAccountUserExclusionRepository(db).Remove(context.Background(), 10, 5)
		if outboxErr != nil {
			require.ErrorIs(t, err, outboxErr)
			require.Zero(t, userID)
		} else {
			require.NoError(t, err)
			require.EqualValues(t, 7, userID)
		}
		require.NoError(t, mock.ExpectationsWereMet())
		_ = db.Close()
	}
}
