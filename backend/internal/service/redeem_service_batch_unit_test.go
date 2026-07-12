//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type batchRedeemRepo struct {
	*redeemRejectRepo
	codes map[string]*RedeemCode
}

func (r *batchRedeemRepo) GetByCode(_ context.Context, code string) (*RedeemCode, error) {
	stored, ok := r.codes[code]
	if !ok {
		return nil, ErrRedeemCodeNotFound
	}
	clone := *stored
	return &clone, nil
}

func (r *batchRedeemRepo) GetByID(_ context.Context, id int64) (*RedeemCode, error) {
	for _, stored := range r.codes {
		if stored.ID == id {
			clone := *stored
			return &clone, nil
		}
	}
	return nil, ErrRedeemCodeNotFound
}

func (r *batchRedeemRepo) Use(_ context.Context, id, userID int64) error {
	for _, stored := range r.codes {
		if stored.ID != id {
			continue
		}
		if stored.Status != StatusUnused {
			return ErrRedeemCodeUsed
		}
		stored.Status = StatusUsed
		stored.UsedBy = &userID
		return nil
	}
	return ErrRedeemCodeNotFound
}

type batchRedeemUserRepo struct {
	*userRepoStub
	balance float64
}

func (r *batchRedeemUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.balance += amount
	return nil
}

func TestRedeemService_BatchSingleUseRejectsSecondCode(t *testing.T) {
	ctx := context.Background()
	client := newCheckinServiceTestClient(t)
	batchID := "activity-batch-1"
	first := &RedeemCode{
		ID:      101,
		Code:    "BATCH-FIRST",
		Type:    RedeemTypeBalance,
		Value:   10,
		Status:  StatusUnused,
		BatchID: &batchID,
	}
	second := &RedeemCode{
		ID:      102,
		Code:    "BATCH-SECOND",
		Type:    RedeemTypeBalance,
		Value:   10,
		Status:  StatusUnused,
		BatchID: &batchID,
	}
	redeemRepo := &batchRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[string]*RedeemCode{
			first.Code:  first,
			second.Code: second,
		},
	}
	userRepo := &batchRedeemUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: 2}},
	}
	redeemService := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, nil)

	result, err := redeemService.Redeem(ctx, 2, first.Code)
	require.NoError(t, err)
	require.Equal(t, first.Code, result.Code)
	require.Equal(t, 10.0, userRepo.balance)

	result, err = redeemService.Redeem(ctx, 2, second.Code)
	require.Nil(t, result)
	require.ErrorIs(t, err, ErrRedeemBatchUserLimit)
	require.Equal(t, 10.0, userRepo.balance)
	require.Equal(t, StatusUnused, second.Status)
	require.Nil(t, second.UsedBy)
}
