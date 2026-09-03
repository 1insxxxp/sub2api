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
		UserIDs: []int64{first.ID, second.ID}, Amount: 3, Description: "manual bonus", CreatedBy: creator.ID,
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
		All: true, Amount: 2, CreatedBy: creator.ID,
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
