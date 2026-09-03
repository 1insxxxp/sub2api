package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/enttest"
	"github.com/Wei-Shaw/sub2api/ent/lotteryattemptledger"
	"github.com/Wei-Shaw/sub2api/ent/lotteryattemptwallet"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func newLotteryAttemptGrantTestClient(t *testing.T) *dbent.Client {
	t.Helper()
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=memory&cache=shared&_fk=1", t.Name()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	db.SetMaxOpenConns(10)
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)
	drv := entsql.OpenDB(dialect.SQLite, db)
	client := enttest.NewClient(t, enttest.WithOptions(dbent.Driver(drv)))
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func createLotteryGrantTestUser(t *testing.T, ctx context.Context, client *dbent.Client, email string) *dbent.User {
	t.Helper()
	user, err := client.User.Create().
		SetEmail(email).
		SetPasswordHash("test-password-hash").
		SetRole(service.RoleUser).
		SetStatus(service.StatusActive).
		Save(ctx)
	require.NoError(t, err)
	return user
}

func TestLotteryRepositoryGrantLotteryAttemptsSelectedUsers(t *testing.T) {
	ctx := context.Background()
	client := newLotteryAttemptGrantTestClient(t)
	creator := createLotteryGrantTestUser(t, ctx, client, "grant-admin@example.com")
	first := createLotteryGrantTestUser(t, ctx, client, "grant-first@example.com")
	second := createLotteryGrantTestUser(t, ctx, client, "grant-second@example.com")
	repo := &lotteryRepository{client: client}

	result, err := repo.GrantLotteryAttempts(ctx, service.LotteryAttemptGrantInput{
		UserIDs: []int64{first.ID, second.ID}, Amount: 3, Description: "manual bonus", RequestKey: "selected-request-1", CreatedBy: creator.ID,
	})

	require.NoError(t, err)
	require.Equal(t, service.LotteryAttemptGrantResult{Affected: 2, TotalGranted: 6}, result)
	for _, userID := range []int64{first.ID, second.ID} {
		wallet, err := client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(userID)).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, wallet.Balance)
		ledger, err := client.LotteryAttemptLedger.Query().Where(
			lotteryattemptledger.UserIDEQ(userID),
			lotteryattemptledger.SourceTypeEQ(service.LotteryAttemptLedgerSourceAdminGrant),
		).Only(ctx)
		require.NoError(t, err)
		require.Equal(t, 3, ledger.Delta)
		require.Equal(t, "manual bonus", ledger.Description)
	}
	require.Equal(t, 2, client.LotteryAttemptGrant.Query().CountX(ctx))
}

func TestLotteryRepositoryGrantLotteryAttemptsAllExcludesDeletedUsers(t *testing.T) {
	ctx := context.Background()
	client := newLotteryAttemptGrantTestClient(t)
	creator := createLotteryGrantTestUser(t, ctx, client, "grant-all-admin@example.com")
	active := createLotteryGrantTestUser(t, ctx, client, "grant-all-active@example.com")
	deleted := createLotteryGrantTestUser(t, ctx, client, "grant-all-deleted@example.com")
	_, err := client.User.UpdateOneID(deleted.ID).SetDeletedAt(time.Now()).Save(ctx)
	require.NoError(t, err)
	repo := &lotteryRepository{client: client}

	result, err := repo.GrantLotteryAttempts(ctx, service.LotteryAttemptGrantInput{
		All: true, Amount: 2, RequestKey: "all-request-1", CreatedBy: creator.ID,
	})

	require.NoError(t, err)
	require.Equal(t, service.LotteryAttemptGrantResult{Affected: 2, TotalGranted: 4}, result)
	_, err = client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(deleted.ID)).Only(ctx)
	require.Error(t, err)
	activeWallet, err := client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(active.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, activeWallet.Balance)
	require.Equal(t, 2, client.LotteryAttemptGrant.Query().CountX(ctx))
}

func TestLotteryRepositoryGrantLotteryAttemptsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client := newLotteryAttemptGrantTestClient(t)
	creator := createLotteryGrantTestUser(t, ctx, client, "grant-idempotent-admin@example.com")
	target := createLotteryGrantTestUser(t, ctx, client, "grant-idempotent-target@example.com")
	repo := &lotteryRepository{client: client}
	input := service.LotteryAttemptGrantInput{UserIDs: []int64{target.ID}, Amount: 4, RequestKey: "retry-request-1", CreatedBy: creator.ID}

	first, err := repo.GrantLotteryAttempts(ctx, input)
	require.NoError(t, err)
	second, err := repo.GrantLotteryAttempts(ctx, input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, client.LotteryAttemptGrant.Query().CountX(ctx))
	require.Equal(t, 1, client.LotteryAttemptLedger.Query().CountX(ctx))
	wallet, err := client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(target.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 4, wallet.Balance)
}

func TestLotteryRepositoryListsAdminAttemptBalances(t *testing.T) {
	ctx := context.Background()
	client := newLotteryAttemptGrantTestClient(t)
	alice := createLotteryGrantTestUser(t, ctx, client, "alice@example.com")
	_, err := client.User.UpdateOneID(alice.ID).SetUsername("Alice").Save(ctx)
	require.NoError(t, err)
	bob := createLotteryGrantTestUser(t, ctx, client, "bob@example.com")
	_, err = client.User.UpdateOneID(bob.ID).SetStatus(service.StatusDisabled).Save(ctx)
	require.NoError(t, err)
	deleted := createLotteryGrantTestUser(t, ctx, client, "deleted@example.com")
	_, err = client.User.UpdateOneID(deleted.ID).SetDeletedAt(time.Now()).Save(ctx)
	require.NoError(t, err)

	activity, err := client.LotteryActivity.Create().
		SetName("September draw").
		SetStatus(service.LotteryActivityStatusActive).
		SetAttemptMode(service.LotteryAttemptModeDaily).
		SetAttemptLimit(5).
		Save(ctx)
	require.NoError(t, err)
	since := time.Date(2026, 9, 3, 0, 0, 0, 0, time.UTC)
	_, err = client.LotteryDraw.Create().
		SetActivityID(activity.ID).
		SetUserID(alice.ID).
		SetPrizeName("today-1").
		SetPrizeType(service.LotteryPrizeTypeBalance).
		SetAttemptKey("alice-today-1").
		SetCreatedAt(since.Add(time.Hour)).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.LotteryDraw.Create().
		SetActivityID(activity.ID).
		SetUserID(alice.ID).
		SetPrizeName("today-2").
		SetPrizeType(service.LotteryPrizeTypeBalance).
		SetAttemptKey("alice-today-2").
		SetCreatedAt(since.Add(2 * time.Hour)).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.LotteryDraw.Create().
		SetActivityID(activity.ID).
		SetUserID(alice.ID).
		SetPrizeName("yesterday").
		SetPrizeType(service.LotteryPrizeTypeBalance).
		SetAttemptKey("alice-yesterday").
		SetCreatedAt(since.Add(-time.Hour)).
		Save(ctx)
	require.NoError(t, err)
	_, err = client.LotteryAttemptWallet.Create().SetUserID(alice.ID).SetBalance(4).Save(ctx)
	require.NoError(t, err)
	_, err = client.LotteryAttemptWallet.Create().SetUserID(bob.ID).SetBalance(2).Save(ctx)
	require.NoError(t, err)

	repo := &lotteryRepository{client: client}
	rows, total, err := repo.ListAdminAttemptBalances(ctx, service.LotteryAdminAttemptBalanceQuery{
		Offset: 0, Limit: 10, ActivityID: activity.ID, ActivityLimit: 5, ActivitySince: &since,
	})
	require.NoError(t, err)
	require.Equal(t, 2, total)
	require.Len(t, rows, 2)
	require.Equal(t, "alice@example.com", rows[0].UserEmail)
	require.Equal(t, "active", rows[0].UserStatus)
	require.Zero(t, rows[0].ActivityRemaining)
	require.Equal(t, 4, rows[0].RewardRemaining)
	require.Equal(t, 4, rows[0].TotalRemaining)
	require.Equal(t, "bob@example.com", rows[1].UserEmail)
	require.Equal(t, service.StatusDisabled, rows[1].UserStatus)
	require.Zero(t, rows[1].ActivityRemaining)
	require.Equal(t, 2, rows[1].RewardRemaining)

	filtered, filteredTotal, err := repo.ListAdminAttemptBalances(ctx, service.LotteryAdminAttemptBalanceQuery{
		Offset: 0, Limit: 10, Search: "Alice", ActivityID: activity.ID, ActivityLimit: 5, ActivitySince: &since,
	})
	require.NoError(t, err)
	require.Equal(t, 1, filteredTotal)
	require.Len(t, filtered, 1)
	require.Equal(t, alice.ID, filtered[0].UserID)

}
