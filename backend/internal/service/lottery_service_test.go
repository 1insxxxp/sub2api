package service

import (
	"encoding/json"
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

func TestLotteryAdminDrawIncludesUserAndRewardDetailsWithoutAttemptKey(t *testing.T) {
	now := time.Date(2026, time.August, 25, 12, 30, 0, 0, timezone.Location())
	draw := LotteryDraw{
		ID:             42,
		ActivityID:     lotteryInt64Ptr(7),
		PrizeID:        lotteryInt64Ptr(9),
		UserID:         11,
		PrizeName:      "高级兑换码",
		PrizeType:      LotteryPrizeTypeProduct,
		ProductContent: lotteryStringPtr("code-001"),
		AttemptKey:     "11:attempt-1",
		AttemptSource:  LotteryAttemptSourceWallet,
		CreatedAt:      now,
	}
	user := &User{ID: 11, Email: "winner@example.com", Username: "Winner"}

	adminDraw := NewLotteryAdminDraw(draw, user)

	if adminDraw.UserEmail != user.Email || adminDraw.UserName != user.Username {
		t.Fatalf("admin draw user mismatch: %#v", adminDraw)
	}
	if adminDraw.PrizeName != draw.PrizeName || adminDraw.ProductContent == nil || *adminDraw.ProductContent != "code-001" {
		t.Fatalf("admin draw reward mismatch: %#v", adminDraw)
	}
	if adminDraw.UserID != draw.UserID || adminDraw.AttemptSource != draw.AttemptSource || !adminDraw.CreatedAt.Equal(now) {
		t.Fatalf("admin draw audit fields mismatch: %#v", adminDraw)
	}

	payload, err := json.Marshal(adminDraw)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) == "" || containsJSONKey(payload, "attempt_key") {
		t.Fatalf("admin draw must not expose attempt key: %s", payload)
	}
}

func TestLotteryAdminDrawMarksDeletedUser(t *testing.T) {
	deletedAt := time.Date(2026, time.August, 26, 9, 0, 0, 0, timezone.Location())
	adminDraw := NewLotteryAdminDraw(LotteryDraw{UserID: 12}, &User{ID: 12, DeletedAt: &deletedAt})

	if !adminDraw.UserDeleted {
		t.Fatalf("expected deleted user marker: %#v", adminDraw)
	}
}

func lotteryFloat64Ptr(value float64) *float64 {
	return &value
}

func lotteryInt64Ptr(value int64) *int64 {
	return &value
}

func lotteryStringPtr(value string) *string {
	return &value
}

func containsJSONKey(payload []byte, key string) bool {
	var fields map[string]any
	if err := json.Unmarshal(payload, &fields); err != nil {
		return false
	}
	_, ok := fields[key]
	return ok
}
