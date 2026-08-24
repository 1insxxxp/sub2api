package service

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

func TestLotteryDailyAttemptStartUsesApplicationTimezone(t *testing.T) {
	beijing := time.Date(2026, time.August, 25, 1, 30, 0, 0, timezone.Location())

	start := lotteryAttemptStart(LotteryAttemptModeDaily, beijing)

	require.Equal(t, timezone.StartOfDay(beijing), start)
}

func TestLotteryAttemptStartForTotalModeIsZero(t *testing.T) {
	now := time.Date(2026, time.August, 25, 1, 30, 0, 0, timezone.Location())

	require.True(t, lotteryAttemptStart(LotteryAttemptModeTotal, now).IsZero())
}

func TestSelectLotteryPrizeHonorsWeightsAndSkipsUnavailableProducts(t *testing.T) {
	prizes := []LotteryPrize{
		{ID: 1, Name: "余额 1 元", Type: LotteryPrizeTypeBalance, Weight: 1, Enabled: true, BalanceAmount: lotteryFloat64Ptr(1)},
		{ID: 2, Name: "已发完兑换码", Type: LotteryPrizeTypeProduct, Weight: 100, Enabled: true, AvailableItemCount: 0},
		{ID: 3, Name: "余额 2 元", Type: LotteryPrizeTypeBalance, Weight: 2, Enabled: true, BalanceAmount: lotteryFloat64Ptr(2)},
	}

	prize, err := selectLotteryPrize(prizes, 1)

	require.NoError(t, err)
	require.Equal(t, int64(3), prize.ID)
}

func TestValidateLotteryPrizeRejectsInvalidBalancePrize(t *testing.T) {
	err := validateLotteryPrize(LotteryPrizeInput{
		Name:   "空余额",
		Type:   LotteryPrizeTypeBalance,
		Weight: 1,
	})

	require.ErrorIs(t, err, ErrLotteryPrizeInvalid)
}

func lotteryFloat64Ptr(value float64) *float64 {
	return &value
}
