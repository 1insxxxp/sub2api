package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type supportLookupReader interface {
	Lookup(context.Context, int64, string) (*service.UserSupportGrantResult, error)
}

func TestUserSupportLookupDoesNotWriteOrConsumeEligibility(t *testing.T) {
	for _, state := range []string{"eligible", "already_received", "legacy"} {
		t.Run(state, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			reader, ok := any(&userSupportRepository{db: db}).(supportLookupReader)
			require.True(t, ok, "support lookup must have an independent read-only repository method")
			stamp := time.Unix(1700000000, 0).UTC()
			mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).
				WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
			mock.ExpectQuery(`^SELECT id,email,balance FROM users WHERE id=\$1 AND deleted_at IS NULL AND status='active'$`).
				WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"id", "email", "balance"}).AddRow(2, "customer@example.com", 7))
			claim := sqlmock.NewRows([]string{"granted_at", "legacy"})
			if state == "already_received" {
				claim.AddRow(stamp, false)
			}
			mock.ExpectQuery(`^SELECT granted_at,legacy FROM support_trial_grants WHERE user_id=\$1$`).WithArgs(int64(2)).WillReturnRows(claim)
			if state != "already_received" {
				legacy := sqlmock.NewRows([]string{"id", "granted_at"})
				if state == "legacy" {
					legacy.AddRow(9, stamp)
				}
				mock.ExpectQuery(`SELECT id,COALESCE\(used_at,created_at\) FROM redeem_codes`).WithArgs(int64(2)).WillReturnRows(legacy)
			}
			mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).
				WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
			got, err := reader.Lookup(context.Background(), 1, "2")
			require.NoError(t, err)
			require.Equal(t, float64(7), got.User.Balance)
			require.Equal(t, 15, got.Trial.Amount)
			if state == "eligible" {
				require.Equal(t, "eligible", got.Trial.Status)
				require.True(t, got.Trial.GrantedAt.IsZero())
			} else {
				require.Equal(t, "already_received", got.Trial.Status)
				require.Equal(t, stamp, got.Trial.GrantedAt)
			}
			require.Equal(t, state == "legacy", got.Trial.Legacy)
			require.NoError(t, mock.ExpectationsWereMet(), "only the expected SELECT statements may execute")
		})
	}
}

func TestUserSupportLookupRejectsFreshPermissionAndMissingTarget(t *testing.T) {
	for _, stage := range []string{"initial", "after_read", "missing"} {
		t.Run(stage, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			reader, ok := any(&userSupportRepository{db: db}).(supportLookupReader)
			require.True(t, ok, "support lookup must have an independent read-only repository method")
			mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).
				WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(stage != "initial"))
			if stage != "initial" {
				rows := sqlmock.NewRows([]string{"id", "email", "balance"})
				if stage != "missing" {
					rows.AddRow(2, "customer@example.com", 7)
				}
				mock.ExpectQuery(`^SELECT id,email,balance FROM users WHERE id=\$1 AND deleted_at IS NULL AND status='active'$`).WithArgs(int64(2)).WillReturnRows(rows)
				if stage == "after_read" {
					mock.ExpectQuery(`SELECT granted_at,legacy FROM support_trial_grants`).WithArgs(int64(2)).WillReturnRows(sqlmock.NewRows([]string{"granted_at", "legacy"}))
					mock.ExpectQuery(`SELECT id,COALESCE\(used_at,created_at\) FROM redeem_codes`).WithArgs(int64(2)).WillReturnError(sql.ErrNoRows)
					mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(false))
				}
			}
			got, err := reader.Lookup(context.Background(), 1, "2")
			require.Nil(t, got)
			if stage == "missing" {
				require.ErrorIs(t, err, service.ErrUserSupportUserNotFound)
			} else {
				require.ErrorIs(t, err, service.ErrUserSupportForbidden)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserSupportSearchOnlyReadsEscapedBoundedSuggestions(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
	mock.ExpectQuery(`SELECT u.id,u.email FROM users u WHERE u.deleted_at IS NULL AND u.status='active'.*LIMIT 8`).
		WithArgs("%a\\%\\_\\\\b%", "a\\%\\_\\\\b%").
		WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(2, `a%_\b@example.com`))
	mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).
		WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
	got, err := (&userSupportRepository{db: db}).Search(context.Background(), 1, ` A%_\B `)
	require.NoError(t, err)
	require.Equal(t, []service.UserSupportSuggestion{{ID: 2, Email: `a%_\b@example.com`}}, got)
	require.NoError(t, mock.ExpectationsWereMet(), "suggestions must execute only SELECT statements")
}

func TestUserSupportSearchRejectsPermissionRevokedDuringRead(t *testing.T) {
	for _, initiallyAllowed := range []bool{false, true} {
		t.Run(map[bool]string{false: "initially_denied", true: "revoked_during_read"}[initiallyAllowed], func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).
				WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(initiallyAllowed))
			if initiallyAllowed {
				mock.ExpectQuery(`SELECT u.id,u.email FROM users u`).WithArgs("%customer%", "customer%").
					WillReturnRows(sqlmock.NewRows([]string{"id", "email"}).AddRow(2, "customer@example.com"))
				mock.ExpectQuery(regexp.QuoteMeta(supportAccessSQL)).WithArgs(int64(1)).
					WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(false))
			}
			got, err := (&userSupportRepository{db: db}).Search(context.Background(), 1, "customer")
			require.Nil(t, got)
			require.ErrorIs(t, err, service.ErrUserSupportForbidden)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
