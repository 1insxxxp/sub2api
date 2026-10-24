//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

func TestRedeemServiceBalanceTransferRevocationPreservesOwnedCodes(t *testing.T) {
	ctx := context.Background()
	userID, otherID := int64(7), int64(8)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{
			ID: userID, Status: StatusActive, Role: RoleUser,
			Balance: 50, BalanceRedeemCodeEnabled: true,
		}},
		balance: 50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	codes, err := svc.GenerateBalanceTransferCodes(ctx, userID, GenerateBalanceTransferCodeInput{Amount: 10, Count: 2})
	require.NoError(t, err)
	require.Len(t, codes, 2)
	require.Equal(t, 30.0, userRepo.balance)
	require.Equal(t, RoleUser, userRepo.user.Role)

	otherCode := RedeemCode{
		ID: 999, Code: "ANOTHER-USERS-CODE", Type: RedeemTypeBalance,
		Value: 10, Status: StatusUnused, CreatedBy: &otherID,
		Source: RedeemCodeSourceUserBalanceTransfer,
	}
	redeemRepo.codes[otherCode.ID] = &otherCode
	redeemRepo.generated = append(append([]RedeemCode{}, codes...), otherCode)
	userRepo.user.BalanceRedeemCodeEnabled = false

	newCodes, err := svc.GenerateBalanceTransferCodes(ctx, userID, GenerateBalanceTransferCodeInput{Amount: 10, Count: 1})
	require.Nil(t, newCodes)
	require.Equal(t, "BALANCE_TRANSFER_REDEEM_NOT_ALLOWED", infraerrors.Reason(err))
	require.Equal(t, []float64{20}, userRepo.ordinaryDeductCalls)
	require.Len(t, redeemRepo.created, 2)
	require.Equal(t, 30.0, userRepo.balance)

	history, page, err := svc.ListGeneratedBalanceTransferCodesPaginated(ctx, userID, pagination.PaginationParams{Page: 1, PageSize: 20})
	require.NoError(t, err)
	require.Len(t, history, 2)
	require.Equal(t, int64(2), page.Total)
	for _, code := range history {
		require.Equal(t, userID, *code.CreatedBy)
	}

	deleted, err := svc.DeleteGeneratedBalanceTransferCode(ctx, userID, otherCode.ID)
	require.Nil(t, deleted)
	require.ErrorIs(t, err, ErrBalanceTransferRedeemCodeNotFound)
	batchDeleted, err := svc.DeleteGeneratedBalanceTransferCodes(ctx, userID, []int64{codes[0].ID, otherCode.ID})
	require.Nil(t, batchDeleted)
	require.ErrorIs(t, err, ErrBalanceTransferRedeemCodeNotFound)
	require.Empty(t, redeemRepo.deleted)
	require.Empty(t, userRepo.adjustCalls)

	deleted, err = svc.DeleteGeneratedBalanceTransferCode(ctx, userID, codes[0].ID)
	require.NoError(t, err)
	require.Equal(t, codes[0].ID, deleted.ID)
	batchDeleted, err = svc.DeleteGeneratedBalanceTransferCodes(ctx, userID, []int64{codes[1].ID})
	require.NoError(t, err)
	require.Len(t, batchDeleted, 1)
	require.Equal(t, codes[1].ID, batchDeleted[0].ID)
	require.Equal(t, []float64{10, 10}, userRepo.adjustCalls)
	require.Equal(t, 50.0, userRepo.balance)
	require.Contains(t, redeemRepo.codes, otherCode.ID)
}
