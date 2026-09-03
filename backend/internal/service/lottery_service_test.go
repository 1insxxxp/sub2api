package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/stretchr/testify/require"
)

type lotteryActivityRepositoryStub struct {
	LotteryRepository
	input    LotteryActivityInput
	activity *LotteryActivity
}

func (s *lotteryActivityRepositoryStub) SaveActivity(_ context.Context, _ int64, input LotteryActivityInput, _ *int64) (*LotteryActivity, error) {
	s.input = input
	return s.activity, nil
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

func TestLotteryActivityPolicyIsNormalizedToWalletOnly(t *testing.T) {
	input := normalizeLotteryActivityInput(LotteryActivityInput{
		Name:         "签到次数活动",
		Status:       LotteryActivityStatusActive,
		AttemptMode:  LotteryAttemptModeDaily,
		AttemptLimit: 10,
	})

	require.Equal(t, LotteryAttemptModeTotal, input.AttemptMode)
	require.Zero(t, input.AttemptLimit)
}

func TestSaveLotteryActivityPersistsWalletOnlyPolicy(t *testing.T) {
	repo := &lotteryActivityRepositoryStub{activity: &LotteryActivity{ID: 7, AttemptMode: LotteryAttemptModeDaily, AttemptLimit: 5}}
	svc := NewLotteryService(nil, repo, nil, nil)

	activity, err := svc.SaveActivity(context.Background(), 7, LotteryActivityInput{
		Name: "抽奖活动", Status: LotteryActivityStatusActive, AttemptMode: LotteryAttemptModeDaily, AttemptLimit: 5,
	}, nil)

	require.NoError(t, err)
	require.Equal(t, LotteryAttemptModeTotal, repo.input.AttemptMode)
	require.Zero(t, repo.input.AttemptLimit)
	require.Equal(t, LotteryAttemptModeTotal, activity.AttemptMode)
	require.Zero(t, activity.AttemptLimit)
}

func TestLotteryAttemptSummaryUsesOnlyPersistentRewardBalance(t *testing.T) {
	summary := summarizeLotteryAttempts(3)

	require.Zero(t, summary.ActivityRemaining)
	require.Equal(t, 3, summary.RewardRemaining)
	require.Equal(t, 3, summary.TotalRemaining)
	require.Equal(t, LotteryAttemptSourceWallet, summary.NextSource)
}

func TestLotteryAttemptSummaryIsExhaustedWhenBothSourcesAreEmpty(t *testing.T) {
	summary := summarizeLotteryAttempts(0)

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
