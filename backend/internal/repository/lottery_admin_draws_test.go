package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestLotteryRepositoryListAdminDrawsAppliesFiltersBeforePagination(t *testing.T) {
	ctx := context.Background()
	client := newLotteryAttemptGrantTestClient(t)
	firstUser := createLotteryGrantTestUser(t, ctx, client, "draw-first@example.com")
	secondUser := createLotteryGrantTestUser(t, ctx, client, "draw-second@example.com")
	repo := &lotteryRepository{client: client}
	now := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)

	for _, draw := range []struct {
		userID int64
		name   string
		typeID string
		source string
		key    string
	}{
		{firstUser.ID, "余额奖励", service.LotteryPrizeTypeBalance, service.LotteryAttemptSourceWallet, "draw-filter-1"},
		{firstUser.ID, "兑换码", service.LotteryPrizeTypeProduct, service.LotteryAttemptSourceWallet, "draw-filter-2"},
		{firstUser.ID, "活动兑换码", service.LotteryPrizeTypeProduct, service.LotteryAttemptSourceActivity, "draw-filter-3"},
		{secondUser.ID, "其他兑换码", service.LotteryPrizeTypeProduct, service.LotteryAttemptSourceWallet, "draw-filter-4"},
	} {
		_, err := client.LotteryDraw.Create().
			SetUserID(draw.userID).
			SetPrizeName(draw.name).
			SetPrizeType(draw.typeID).
			SetAttemptSource(draw.source).
			SetAttemptKey(draw.key).
			SetCreatedAt(now).
			Save(ctx)
		require.NoError(t, err)
	}

	draws, total, err := repo.ListAdminDraws(ctx, service.LotteryAdminDrawQuery{
		Offset: 0, Limit: 1, UserID: firstUser.ID,
		PrizeType: service.LotteryPrizeTypeProduct, AttemptSource: service.LotteryAttemptSourceWallet,
	})

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, draws, 1)
	require.Equal(t, "兑换码", draws[0].PrizeName)
	require.Equal(t, "draw-first@example.com", draws[0].UserEmail)
}

func TestLotteryRepositoryListAdminDrawsFiltersNonWinningRecords(t *testing.T) {
	ctx := context.Background()
	client := newLotteryAttemptGrantTestClient(t)
	target := createLotteryGrantTestUser(t, ctx, client, "draw-winner@example.com")
	repo := &lotteryRepository{client: client}

	for index, typeID := range []string{service.LotteryPrizeTypeBalance, service.LotteryPrizeTypeNone} {
		_, err := client.LotteryDraw.Create().
			SetUserID(target.ID).
			SetPrizeName(typeID).
			SetPrizeType(typeID).
			SetAttemptSource(service.LotteryAttemptSourceWallet).
			SetAttemptKey("draw-winner-" + string(rune('1'+index))).
			Save(ctx)
		require.NoError(t, err)
	}

	draws, total, err := repo.ListAdminDraws(ctx, service.LotteryAdminDrawQuery{Limit: 20, WinnersOnly: true})

	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, draws, 1)
	require.Equal(t, service.LotteryPrizeTypeBalance, draws[0].PrizeType)
}
