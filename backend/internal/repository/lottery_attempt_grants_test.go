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

func createLotteryGrantUsageLog(t *testing.T, ctx context.Context, client *dbent.Client, target *dbent.User, requestID string, createdAt time.Time) {
	t.Helper()
	account, err := client.Account.Create().
		SetName("lottery-account-" + requestID).
		SetPlatform("openai").
		SetType("api_key").
		Save(ctx)
	require.NoError(t, err)
	apiKey, err := client.APIKey.Create().
		SetUserID(target.ID).
		SetKey("lottery-key-" + requestID).
		SetName("lottery key").
		Save(ctx)
	require.NoError(t, err)
	_, err = client.UsageLog.Create().
		SetUserID(target.ID).
		SetAPIKeyID(apiKey.ID).
		SetAccountID(account.ID).
		SetRequestID(requestID).
		SetModel("test-model").
		SetCreatedAt(createdAt).
		Save(ctx)
	require.NoError(t, err)
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

	// A later grant must also work when the wallet rows already exist. This is
	// the normal path after the first all-user grant has initialized them.
	secondResult, err := repo.GrantLotteryAttempts(ctx, service.LotteryAttemptGrantInput{
		All: true, Amount: 3, RequestKey: "all-request-2", CreatedBy: creator.ID,
	})
	require.NoError(t, err)
	require.Equal(t, service.LotteryAttemptGrantResult{Affected: 2, TotalGranted: 6}, secondResult)
	activeWallet, err = client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(active.ID)).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, 5, activeWallet.Balance)
	require.Equal(t, 4, client.LotteryAttemptGrant.Query().CountX(ctx))
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

func TestLotteryRepositoryPreviewAndGrantActiveUsers(t *testing.T) {
	ctx := context.Background()
	client := newLotteryAttemptGrantTestClient(t)
	creator := createLotteryGrantTestUser(t, ctx, client, "active-grant-admin@example.com")
	recent := createLotteryGrantTestUser(t, ctx, client, "active-grant-recent@example.com")
	boundary := createLotteryGrantTestUser(t, ctx, client, "active-grant-boundary@example.com")
	old := createLotteryGrantTestUser(t, ctx, client, "active-grant-old@example.com")
	deleted := createLotteryGrantTestUser(t, ctx, client, "active-grant-deleted@example.com")
	_, err := client.User.UpdateOneID(deleted.ID).SetDeletedAt(time.Date(2026, 9, 3, 10, 0, 0, 0, time.UTC)).Save(ctx)
	require.NoError(t, err)

	now := time.Date(2026, 9, 3, 12, 0, 0, 0, time.UTC)
	since := now.AddDate(0, 0, -7)
	createLotteryGrantUsageLog(t, ctx, client, recent, "active-recent", now.Add(-time.Hour))
	createLotteryGrantUsageLog(t, ctx, client, boundary, "active-boundary", since)
	createLotteryGrantUsageLog(t, ctx, client, old, "active-old", since.Add(-time.Nanosecond))
	createLotteryGrantUsageLog(t, ctx, client, deleted, "active-deleted", now.Add(-time.Hour))
	repo := &lotteryRepository{client: client}
	input := service.LotteryAttemptGrantInput{Target: service.LotteryAttemptGrantTargetActive, ActiveDays: service.LotteryAttemptActiveDays7, ActiveSince: &since}

	preview, err := repo.PreviewLotteryAttemptGrant(ctx, input)
	require.NoError(t, err)
	require.Equal(t, service.LotteryAttemptGrantPreviewResult{Count: 2}, preview)

	result, err := repo.GrantLotteryAttempts(ctx, service.LotteryAttemptGrantInput{
		Target: service.LotteryAttemptGrantTargetActive, ActiveDays: service.LotteryAttemptActiveDays7, ActiveSince: &since,
		Amount: 3, RequestKey: "active-request-1", CreatedBy: creator.ID,
	})
	require.NoError(t, err)
	require.Equal(t, service.LotteryAttemptGrantResult{Affected: 2, TotalGranted: 6}, result)
	for _, userID := range []int64{recent.ID, boundary.ID} {
		wallet, walletErr := client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(userID)).Only(ctx)
		require.NoError(t, walletErr)
		require.Equal(t, 3, wallet.Balance)
	}
	for _, userID := range []int64{old.ID, deleted.ID} {
		_, walletErr := client.LotteryAttemptWallet.Query().Where(lotteryattemptwallet.UserIDEQ(userID)).Only(ctx)
		require.Error(t, walletErr)
	}
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
