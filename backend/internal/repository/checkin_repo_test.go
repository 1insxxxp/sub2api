package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/usercheckin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newCheckinRepoSQLite(t *testing.T) (*checkinRepository, *dbent.Client) {
	t.Helper()

	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })

	return NewCheckinRepository(client).(*checkinRepository), client
}

func mustCreateCheckinRepoUser(t *testing.T, ctx context.Context, client *dbent.Client, email string, balance, totalRecharged float64) *dbent.User {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("test-password-hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		SetBalance(balance).
		SetTotalRecharged(totalRecharged).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func TestCheckinRepositoryCheckinCreditsBalanceWithoutTotalRecharged(t *testing.T) {
	repo, client := newCheckinRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateCheckinRepoUser(t, ctx, client, "checkin-credit@test.com", 10, 7.5)

	result, err := repo.Checkin(ctx, service.CheckinCreateInput{
		UserID:           user.ID,
		CheckinDate:      "2026-06-14",
		BaseRewardAmount: 2,
		StreakEnabled:    true,
		StreakRules: []service.CheckinStreakRule{
			{Day: 1, BonusAmount: 10},
		},
	})

	require.NoError(t, err)
	require.True(t, result.CheckedInToday)
	require.Equal(t, 1, result.CurrentStreak)
	require.Equal(t, 1, result.LifetimeCheckinDays)
	require.Equal(t, 2.0, result.BaseRewardAmount)
	require.Equal(t, 10.0, result.BonusRewardAmount)
	require.Equal(t, 12.0, result.TotalRewardAmount)
	require.Equal(t, 10.0, result.BalanceBefore)
	require.Equal(t, 22.0, result.BalanceAfter)

	gotUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 22.0, gotUser.Balance)
	require.Equal(t, 7.5, gotUser.TotalRecharged)
}

func TestCheckinRepositoryDuplicateSameDayDoesNotCreditAgain(t *testing.T) {
	repo, client := newCheckinRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateCheckinRepoUser(t, ctx, client, "checkin-dupe@test.com", 10, 0)
	input := service.CheckinCreateInput{
		UserID:           user.ID,
		CheckinDate:      "2026-06-14",
		BaseRewardAmount: 2,
	}

	first, err := repo.Checkin(ctx, input)
	require.NoError(t, err)
	second, err := repo.Checkin(ctx, input)
	require.NoError(t, err)

	require.Equal(t, first.BalanceAfter, second.BalanceAfter)
	gotUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 12.0, gotUser.Balance)
	count, err := client.UserCheckin.Query().
		Where(usercheckin.UserIDEQ(user.ID), usercheckin.CheckinDateEQ("2026-06-14")).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, count)
}

func TestCheckinRepositoryActiveBlacklistRejectsCheckin(t *testing.T) {
	repo, client := newCheckinRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateCheckinRepoUser(t, ctx, client, "checkin-blacklist@test.com", 10, 0)
	_, err := client.UserCheckinBlacklist.Create().
		SetUserID(user.ID).
		SetReason("abuse").
		Save(ctx)
	require.NoError(t, err)

	_, err = repo.Checkin(ctx, service.CheckinCreateInput{
		UserID:           user.ID,
		CheckinDate:      "2026-06-14",
		BaseRewardAmount: 2,
	})

	require.ErrorIs(t, err, service.ErrCheckinBlacklisted)
	gotUser, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, 10.0, gotUser.Balance)
}
