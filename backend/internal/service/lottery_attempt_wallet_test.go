//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/ent/lotteryattemptledger"
	"github.com/stretchr/testify/require"
)

func TestLotteryAttemptWalletCreditIsIdempotentBySource(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "lottery-credit@example.com", 0)

	balance, credited, err := creditLotteryAttempts(
		ctx, client, user.ID, 2, LotteryAttemptLedgerSourceCheckinStreak, 101, "streak reward",
	)
	require.NoError(t, err)
	require.True(t, credited)
	require.Equal(t, 2, balance)

	balance, credited, err = creditLotteryAttempts(
		ctx, client, user.ID, 2, LotteryAttemptLedgerSourceCheckinStreak, 101, "streak reward",
	)
	require.NoError(t, err)
	require.False(t, credited)
	require.Equal(t, 2, balance)

	ledgerCount, err := client.LotteryAttemptLedger.Query().Where(
		lotteryattemptledger.SourceTypeEQ(LotteryAttemptLedgerSourceCheckinStreak),
		lotteryattemptledger.SourceIDEQ(101),
	).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, 1, ledgerCount)
}

func TestLotteryAttemptWalletDebitRecordsBalanceAndRejectsExhaustion(t *testing.T) {
	client := newCheckinServiceTestClient(t)
	ctx := context.Background()
	user := createCheckinTestUser(t, ctx, client, "lottery-debit@example.com", 0)

	_, _, err := creditLotteryAttempts(
		ctx, client, user.ID, 1, LotteryAttemptLedgerSourceCheckinStreak, 201, "streak reward",
	)
	require.NoError(t, err)

	balance, err := debitLotteryAttempt(ctx, client, user.ID, 301, "lottery draw")
	require.NoError(t, err)
	require.Equal(t, 0, balance)

	ledger, err := client.LotteryAttemptLedger.Query().Where(
		lotteryattemptledger.SourceTypeEQ(LotteryAttemptLedgerSourceLotteryDraw),
		lotteryattemptledger.SourceIDEQ(301),
	).Only(ctx)
	require.NoError(t, err)
	require.Equal(t, -1, ledger.Delta)
	require.Equal(t, 0, ledger.BalanceAfter)

	_, err = debitLotteryAttempt(ctx, client, user.ID, 302, "lottery draw")
	require.ErrorIs(t, err, ErrLotteryAttemptsExhausted)
}
