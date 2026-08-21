//go:build unit

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

type convertUserRepoStub struct {
	*userRepoStub
	deducted  []float64
	deductErr error
}

func (s *convertUserRepoStub) DeductBalance(ctx context.Context, id int64, amount float64) error {
	if s.deductErr != nil {
		return s.deductErr
	}
	s.deducted = append(s.deducted, amount)
	if s.userRepoStub != nil && s.userRepoStub.user != nil {
		s.userRepoStub.user.Balance -= amount
	}
	return nil
}

type convertRedeemRepoStub struct {
	*redeemRepoStub
	created   []RedeemCode
	createErr error
}

func (s *convertRedeemRepoStub) CreateBatch(ctx context.Context, codes []RedeemCode) error {
	if s.createErr != nil {
		return s.createErr
	}
	for _, code := range codes {
		s.created = append(s.created, code)
	}
	return nil
}

func TestRedeemService_ConvertBalanceToRedeemCodes_DeductsBalanceAndCreatesCodes(t *testing.T) {
	userRepo := &convertUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Balance: 10, Role: RoleSubAdmin, Status: StatusActive}},
	}
	redeemRepo := &convertRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil)

	result, err := svc.ConvertBalanceToRedeemCodes(context.Background(), 7, ConvertBalanceToRedeemCodesInput{
		Value: 2.5,
		Count: 3,
	})

	require.NoError(t, err)
	require.Len(t, result.Codes, 3)
	require.InDelta(t, 2.5, result.Codes[0].Value, 0.000001)
	require.Equal(t, RedeemTypeBalance, result.Codes[0].Type)
	require.Equal(t, StatusUnused, result.Codes[0].Status)
	require.Len(t, redeemRepo.created, 3)
	require.Equal(t, []float64{7.5}, userRepo.deducted)
	require.InDelta(t, 2.5, result.NewBalance, 0.000001)
}

func TestRedeemService_ConvertBalanceToRedeemCodes_RejectsInsufficientBalance(t *testing.T) {
	userRepo := &convertUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Balance: 4, Role: RoleSubAdmin, Status: StatusActive}},
	}
	redeemRepo := &convertRedeemRepoStub{redeemRepoStub: &redeemRepoStub{}}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil)

	_, err := svc.ConvertBalanceToRedeemCodes(context.Background(), 7, ConvertBalanceToRedeemCodesInput{
		Value: 2.5,
		Count: 2,
	})

	require.ErrorIs(t, err, ErrInsufficientBalance)
	require.Empty(t, redeemRepo.created)
	require.Empty(t, userRepo.deducted)
}

func TestRedeemService_ConvertBalanceToRedeemCodes_RollsBackDeductionOnCreateFailure(t *testing.T) {
	userRepo := &convertUserRepoStub{
		userRepoStub: &userRepoStub{user: &User{ID: 7, Balance: 10, Role: RoleSubAdmin, Status: StatusActive}},
	}
	redeemRepo := &convertRedeemRepoStub{
		redeemRepoStub: &redeemRepoStub{},
		createErr:      errors.New("create failed"),
	}
	svc := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, nil, nil)

	_, err := svc.ConvertBalanceToRedeemCodes(context.Background(), 7, ConvertBalanceToRedeemCodesInput{
		Value: 2.5,
		Count: 2,
	})

	require.Error(t, err)
	require.Empty(t, userRepo.deducted)
	require.InDelta(t, 10, userRepo.user.Balance, 0.000001)
}
