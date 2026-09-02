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

func TestValidateLotteryActivityAllowsZeroFreeAttempts(t *testing.T) {
	err := validateLotteryActivity(LotteryActivityInput{
		Name:         "签到次数活动",
		Status:       LotteryActivityStatusActive,
		AttemptMode:  LotteryAttemptModeDaily,
		AttemptLimit: 0,
	})

	require.NoError(t, err)
}

func TestLotteryAttemptSummaryAddsPersistentRewardBalance(t *testing.T) {
	summary := summarizeLotteryAttempts(2, 1, 3)

	require.Equal(t, 1, summary.ActivityRemaining)
	require.Equal(t, 3, summary.RewardRemaining)
	require.Equal(t, 4, summary.TotalRemaining)
	require.Equal(t, LotteryAttemptSourceActivity, summary.NextSource)
}

func TestLotteryAttemptSummaryFallsBackToRewardBalance(t *testing.T) {
	summary := summarizeLotteryAttempts(0, 0, 2)

	require.Equal(t, 0, summary.ActivityRemaining)
	require.Equal(t, 2, summary.RewardRemaining)
	require.Equal(t, 2, summary.TotalRemaining)
	require.Equal(t, LotteryAttemptSourceWallet, summary.NextSource)
}

func TestLotteryAttemptSummaryIsExhaustedWhenBothSourcesAreEmpty(t *testing.T) {
	summary := summarizeLotteryAttempts(0, 0, 0)

	require.Zero(t, summary.TotalRemaining)
	require.Empty(t, summary.NextSource)
}

func lotteryFloat64Ptr(value float64) *float64 {
	return &value
}
