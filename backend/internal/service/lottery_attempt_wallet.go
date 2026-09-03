package service

import (
	"context"
	"fmt"
	"strings"

	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/lotteryattemptledger"
	"github.com/Wei-Shaw/sub2api/ent/lotteryattemptwallet"
)

const (
	LotteryAttemptLedgerSourceCheckinStreak = "checkin_streak"
	LotteryAttemptLedgerSourceLotteryDraw   = "lottery_draw"
	LotteryAttemptLedgerSourceAdminGrant    = "admin_grant"
)

func lotteryAttemptBalance(ctx context.Context, client *dbent.Client, userID int64) (int, error) {
	if client == nil || userID <= 0 {
		return 0, ErrUserNotFound
	}
	wallet, err := lotteryClientFromContext(ctx, client).LotteryAttemptWallet.Query().
		Where(lotteryattemptwallet.UserIDEQ(userID)).
		Only(ctx)
	if dbent.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("load lottery attempt wallet: %w", err)
	}
	return wallet.Balance, nil
}

func creditLotteryAttempts(
	ctx context.Context,
	client *dbent.Client,
	userID int64,
	amount int,
	sourceType string,
	sourceID int64,
	description string,
) (int, bool, error) {
	if client == nil || userID <= 0 {
		return 0, false, ErrUserNotFound
	}
	if amount <= 0 || sourceID <= 0 || sourceType != LotteryAttemptLedgerSourceCheckinStreak {
		return 0, false, ErrLotteryConfigurationDenied
	}
	if dbent.TxFromContext(ctx) == nil {
		var (
			balance  int
			credited bool
		)
		err := withLotteryAttemptTx(ctx, client, func(txCtx context.Context) error {
			var err error
			balance, credited, err = creditLotteryAttemptsWithClient(txCtx, lotteryClientFromContext(txCtx, client), userID, amount, sourceType, sourceID, description)
			return err
		})
		return balance, credited, err
	}
	return creditLotteryAttemptsWithClient(ctx, lotteryClientFromContext(ctx, client), userID, amount, sourceType, sourceID, description)
}

func creditLotteryAttemptsWithClient(
	ctx context.Context,
	client *dbent.Client,
	userID int64,
	amount int,
	sourceType string,
	sourceID int64,
	description string,
) (int, bool, error) {
	existing, err := client.LotteryAttemptLedger.Query().Where(
		lotteryattemptledger.SourceTypeEQ(sourceType),
		lotteryattemptledger.SourceIDEQ(sourceID),
	).Only(ctx)
	if err == nil {
		return existing.BalanceAfter, false, nil
	}
	if !dbent.IsNotFound(err) {
		return 0, false, fmt.Errorf("load lottery attempt credit ledger: %w", err)
	}

	err = client.LotteryAttemptWallet.Create().
		SetUserID(userID).
		SetBalance(0).
		OnConflict(entsql.ConflictColumns(lotteryattemptwallet.FieldUserID)).
		DoNothing().
		Exec(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("ensure lottery attempt wallet: %w", err)
	}
	wallet, err := client.LotteryAttemptWallet.Query().
		Where(lotteryattemptwallet.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("load lottery attempt wallet for credit: %w", err)
	}
	wallet, err = client.LotteryAttemptWallet.UpdateOneID(wallet.ID).
		AddBalance(amount).
		Save(ctx)
	if err != nil {
		return 0, false, fmt.Errorf("credit lottery attempt wallet: %w", err)
	}
	if _, err = client.LotteryAttemptLedger.Create().
		SetUserID(userID).
		SetDelta(amount).
		SetBalanceAfter(wallet.Balance).
		SetSourceType(sourceType).
		SetSourceID(sourceID).
		SetDescription(strings.TrimSpace(description)).
		Save(ctx); err != nil {
		return 0, false, fmt.Errorf("create lottery attempt credit ledger: %w", err)
	}
	return wallet.Balance, true, nil
}

func debitLotteryAttempt(
	ctx context.Context,
	client *dbent.Client,
	userID int64,
	sourceID int64,
	description string,
) (int, error) {
	if client == nil || userID <= 0 {
		return 0, ErrUserNotFound
	}
	if sourceID <= 0 {
		return 0, ErrLotteryConfigurationDenied
	}
	if dbent.TxFromContext(ctx) == nil {
		var balance int
		err := withLotteryAttemptTx(ctx, client, func(txCtx context.Context) error {
			var err error
			balance, err = debitLotteryAttempt(txCtx, client, userID, sourceID, description)
			return err
		})
		return balance, err
	}

	txClient := lotteryClientFromContext(ctx, client)
	existing, err := txClient.LotteryAttemptLedger.Query().Where(
		lotteryattemptledger.SourceTypeEQ(LotteryAttemptLedgerSourceLotteryDraw),
		lotteryattemptledger.SourceIDEQ(sourceID),
	).Only(ctx)
	if err == nil {
		return existing.BalanceAfter, nil
	}
	if !dbent.IsNotFound(err) {
		return 0, fmt.Errorf("load lottery attempt debit ledger: %w", err)
	}

	updated, err := txClient.LotteryAttemptWallet.Update().Where(
		lotteryattemptwallet.UserIDEQ(userID),
		lotteryattemptwallet.BalanceGT(0),
	).AddBalance(-1).Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("debit lottery attempt wallet: %w", err)
	}
	if updated == 0 {
		return 0, ErrLotteryAttemptsExhausted
	}
	wallet, err := txClient.LotteryAttemptWallet.Query().
		Where(lotteryattemptwallet.UserIDEQ(userID)).
		Only(ctx)
	if err != nil {
		return 0, fmt.Errorf("load lottery attempt wallet after debit: %w", err)
	}
	if _, err = txClient.LotteryAttemptLedger.Create().
		SetUserID(userID).
		SetDelta(-1).
		SetBalanceAfter(wallet.Balance).
		SetSourceType(LotteryAttemptLedgerSourceLotteryDraw).
		SetSourceID(sourceID).
		SetDescription(strings.TrimSpace(description)).
		Save(ctx); err != nil {
		return 0, fmt.Errorf("create lottery attempt debit ledger: %w", err)
	}
	return wallet.Balance, nil
}

func lotteryClientFromContext(ctx context.Context, fallback *dbent.Client) *dbent.Client {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return tx.Client()
	}
	return fallback
}

func withLotteryAttemptTx(ctx context.Context, client *dbent.Client, fn func(context.Context) error) error {
	tx, err := client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin lottery attempt transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(dbent.NewTxContext(ctx, tx)); err != nil {
		return err
	}
	return tx.Commit()
}
