//go:build unit

package service

import (
	"context"
	"database/sql"
	"testing"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "modernc.org/sqlite"
)

func newCheckinServiceTestClient(t *testing.T) *dbent.Client {
	t.Helper()

	db, err := sql.Open("sqlite", "file:checkin_service?mode=memory&cache=shared&_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestCheckinRewardForRollUsesWeightedTiers(t *testing.T) {
	require.Equal(t, 2.0, checkinRewardForRoll(0))
	require.Equal(t, 2.0, checkinRewardForRoll(0.4999))
	require.Equal(t, 3.0, checkinRewardForRoll(0.5))
	require.Equal(t, 3.0, checkinRewardForRoll(0.7999))
	require.Equal(t, 4.0, checkinRewardForRoll(0.8))
	require.Equal(t, 4.0, checkinRewardForRoll(0.9499))
	require.Equal(t, 5.0, checkinRewardForRoll(0.95))
	require.Equal(t, 5.0, checkinRewardForRoll(0.9999))
}

func TestCheckinServiceCheckinAwardsBalanceForBeijingDate(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()

	createdUser, err := client.User.Create().
		SetEmail("checkin-user@example.com").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetStatus(StatusActive).
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	svc := NewCheckinService(client, nil, nil)
	svc.now = func() time.Time {
		return time.Date(2026, 6, 4, 16, 30, 0, 0, time.UTC)
	}
	svc.rewardRoll = func() float64 { return 0.5 }

	result, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.False(t, result.AlreadyCheckedIn)
	require.True(t, result.CheckedIn)
	require.Equal(t, "2026-06-05", result.CheckinDate)
	require.Equal(t, 3.0, result.RewardAmount)
	require.Equal(t, 10.0, result.BalanceBefore)
	require.Equal(t, 13.0, result.BalanceAfter)

	updatedUser, err := client.User.Get(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 13.0, updatedUser.Balance)

	records, err := client.UserCheckin.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, records, 1)
	require.Equal(t, "2026-06-05", records[0].CheckinDate)
	require.Equal(t, 3.0, records[0].RewardAmount)

	historyCount, err := client.RedeemCode.Query().
		Where(
			redeemcode.UsedByEQ(createdUser.ID),
			redeemcode.TypeEQ(AdjustmentTypeCheckinReward),
		).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, historyCount)
}

func TestCheckinServiceSecondCheckinSameBeijingDateDoesNotAwardAgain(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()

	createdUser, err := client.User.Create().
		SetEmail("checkin-once@example.com").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetStatus(StatusActive).
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	svc := NewCheckinService(client, nil, nil)
	svc.now = func() time.Time {
		return time.Date(2026, 6, 5, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	}
	svc.rewardRoll = func() float64 { return 0 }

	first, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 2.0, first.RewardAmount)

	svc.rewardRoll = func() float64 { return 0.99 }
	second, err := svc.Checkin(ctx, createdUser.ID)
	require.NoError(t, err)
	require.True(t, second.AlreadyCheckedIn)
	require.Equal(t, 2.0, second.RewardAmount)
	require.Equal(t, 12.0, second.BalanceAfter)

	updatedUser, err := client.User.Get(ctx, createdUser.ID)
	require.NoError(t, err)
	require.Equal(t, 12.0, updatedUser.Balance)

	checkinCount, err := client.UserCheckin.Query().Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, checkinCount)

	historyCount, err := client.RedeemCode.Query().
		Where(redeemcode.TypeEQ(AdjustmentTypeCheckinReward)).
		Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, historyCount)
}

func TestCheckinServiceBlacklistBlocksStatusAndCheckin(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()

	createdUser, err := client.User.Create().
		SetEmail("blocked@example.com").
		SetPasswordHash("hash").
		SetRole(RoleUser).
		SetStatus(StatusActive).
		SetBalance(10).
		Save(ctx)
	require.NoError(t, err)

	_, err = client.UserCheckinBlacklist.Create().
		SetUserID(createdUser.ID).
		SetReason("manual block").
		SetCreatedBy(1).
		Save(ctx)
	require.NoError(t, err)

	svc := NewCheckinService(client, nil, nil)
	status, err := svc.GetStatus(ctx, createdUser.ID)
	require.NoError(t, err)
	require.True(t, status.Blacklisted)
	require.False(t, status.Enabled)

	_, err = svc.Checkin(ctx, createdUser.ID)
	require.ErrorIs(t, err, ErrCheckinBlacklisted)
}
