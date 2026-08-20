package service

import (
	"context"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type balanceTransferUserRepo struct {
	*userRepoStub
	balance     float64
	adjustCalls []float64
}

func (r *balanceTransferUserRepo) AdjustBalance(_ context.Context, id int64, delta float64) (BalanceChange, error) {
	r.adjustCalls = append(r.adjustCalls, delta)
	if r.user == nil || r.user.ID != id {
		return BalanceChange{}, ErrUserNotFound
	}
	old := r.balance
	next := old + delta
	if next < 0 {
		return BalanceChange{Old: old, New: next}, ErrBalanceNegative
	}
	r.balance = next
	r.user.Balance = next
	return BalanceChange{Old: old, New: next}, nil
}

type balanceTransferRedeemRepo struct {
	*redeemRejectRepo
	nextID    int64
	created   []*RedeemCode
	generated []RedeemCode
}

func (r *balanceTransferRedeemRepo) Create(_ context.Context, code *RedeemCode) error {
	if r.nextID == 0 {
		r.nextID = 100
	}
	code.ID = r.nextID
	r.nextID++
	if code.CreatedAt.IsZero() {
		code.CreatedAt = time.Now().UTC()
	}
	clone := *code
	r.created = append(r.created, &clone)
	return nil
}

func (r *balanceTransferRedeemRepo) ListByCreator(_ context.Context, userID int64, limit int) ([]RedeemCode, error) {
	if limit <= 0 {
		limit = 25
	}
	out := make([]RedeemCode, 0)
	for _, code := range r.generated {
		if code.CreatedBy == nil || *code.CreatedBy != userID {
			continue
		}
		out = append(out, code)
		if len(out) >= limit {
			break
		}
	}
	return out, nil
}

func TestRedeemServiceBalanceTransferRejectsUnauthorizedUser(t *testing.T) {
	ctx := context.Background()
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Status: StatusActive, Balance: 50}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCode(ctx, 7, GenerateBalanceTransferCodeInput{Amount: 10})

	require.Nil(t, got)
	require.Error(t, err)
	require.True(t, infraerrors.IsForbidden(err))
	require.Equal(t, "BALANCE_TRANSFER_REDEEM_NOT_ALLOWED", infraerrors.Reason(err))
	require.Empty(t, redeemRepo.created)
	require.Empty(t, userRepo.adjustCalls)
	require.Equal(t, 50.0, userRepo.balance)
}

func TestRedeemServiceBalanceTransferRejectsInsufficientBalance(t *testing.T) {
	ctx := context.Background()
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Status: StatusActive, Balance: 3, BalanceRedeemCodeEnabled: true}},
		balance:      3,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCode(ctx, 7, GenerateBalanceTransferCodeInput{Amount: 10})

	require.Nil(t, got)
	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Empty(t, redeemRepo.created)
	require.Equal(t, []float64{-10}, userRepo.adjustCalls)
	require.Equal(t, 3.0, userRepo.balance)
}

func TestRedeemServiceBalanceTransferCreatesCodeAndDeductsBalance(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	userRepo := &balanceTransferUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: userID, Status: StatusActive, Balance: 50, BalanceRedeemCodeEnabled: true}},
		balance:      50,
	}
	redeemRepo := &balanceTransferRedeemRepo{redeemRejectRepo: &redeemRejectRepo{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil, nil)

	got, err := svc.GenerateBalanceTransferCode(ctx, userID, GenerateBalanceTransferCodeInput{
		Amount:        12.5,
		ExpiresInDays: 14,
		Notes:         "for teammate",
	})

	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Code, 32)
	require.Equal(t, RedeemTypeBalance, got.Type)
	require.Equal(t, 12.5, got.Value)
	require.Equal(t, StatusUnused, got.Status)
	require.NotNil(t, got.CreatedBy)
	require.Equal(t, userID, *got.CreatedBy)
	require.Equal(t, RedeemCodeSourceUserBalanceTransfer, got.Source)
	require.Equal(t, "for teammate", got.Notes)
	require.Equal(t, 14, got.ValidityDays)
	require.NotNil(t, got.ExpiresAt)
	require.WithinDuration(t, time.Now().UTC().AddDate(0, 0, 14), *got.ExpiresAt, 3*time.Second)
	require.Equal(t, []float64{-12.5}, userRepo.adjustCalls)
	require.Equal(t, 37.5, userRepo.balance)
	require.Len(t, redeemRepo.created, 1)
}

func TestRedeemServiceBalanceTransferListsOnlyCreatorCodes(t *testing.T) {
	ctx := context.Background()
	userID := int64(7)
	otherID := int64(8)
	redeemRepo := &balanceTransferRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		generated: []RedeemCode{
			{ID: 1, Code: "OWN", CreatedBy: &userID, Source: RedeemCodeSourceUserBalanceTransfer},
			{ID: 2, Code: "OTHER", CreatedBy: &otherID, Source: RedeemCodeSourceUserBalanceTransfer},
		},
	}
	svc := NewRedeemService(redeemRepo, &balanceTransferUserRepo{userRepoStub: &userRepoStub{}}, nil, nil, nil, nil, nil, nil)

	codes, err := svc.ListGeneratedBalanceTransferCodes(ctx, userID, 25)

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.Equal(t, "OWN", codes[0].Code)
}
