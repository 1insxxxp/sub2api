//go:build integration

package repository

import (
	"context"
	"errors"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/redeemcode"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

var errForcedSecondBalanceTransferCreate = errors.New("forced second balance transfer create failure")
var errForcedGiftCreditRollback = errors.New("forced failure after gift credit")

type failAfterGiftCreditUserRepository struct {
	service.UserRepository
	service.GiftBalanceRepository
}

func (r *failAfterGiftCreditUserRepository) CreditGiftBalance(ctx context.Context, userID int64, amount float64) error {
	if dbent.TxFromContext(ctx) == nil {
		return errors.New("gift credit did not receive ent transaction context")
	}
	if err := r.GiftBalanceRepository.CreditGiftBalance(ctx, userID, amount); err != nil {
		return err
	}
	return errForcedGiftCreditRollback
}

type failSecondBalanceTransferCreateRepository struct {
	service.RedeemCodeRepository
	createCalls        int
	firstCreateID      int64
	firstCreateUsedTxn bool
}

func (r *failSecondBalanceTransferCreateRepository) Create(ctx context.Context, code *service.RedeemCode) error {
	r.createCalls++
	if r.createCalls == 2 {
		if dbent.TxFromContext(ctx) == nil {
			return errors.New("second create did not receive ent transaction context")
		}
		return errForcedSecondBalanceTransferCreate
	}

	r.firstCreateUsedTxn = dbent.TxFromContext(ctx) != nil
	if err := r.RedeemCodeRepository.Create(ctx, code); err != nil {
		return err
	}
	r.firstCreateID = code.ID
	return nil
}

func (s *UserRepoSuite) TestBalanceTransferBatchRollsBackOrdinaryDeductionAndFirstCode() {
	user := s.mustCreateUser(&service.User{
		Email:   "balance-transfer-rollback@example.com",
		Role:    service.RoleSubAdmin,
		Balance: 20,
	})
	s.Require().NoError(s.client.User.UpdateOneID(user.ID).SetGiftBalance(8).Exec(s.ctx))

	redeemRepo := &failSecondBalanceTransferCreateRepository{
		RedeemCodeRepository: NewRedeemCodeRepository(s.client),
	}
	redeemService := service.NewRedeemService(
		redeemRepo,
		s.repo,
		nil,
		nil,
		nil,
		s.client,
		nil,
		nil,
	)

	codes, err := redeemService.GenerateBalanceTransferCodes(s.ctx, user.ID, service.GenerateBalanceTransferCodeInput{
		Amount:          5,
		Count:           2,
		ThresholdExempt: true,
	})

	s.Require().Nil(codes)
	s.Require().ErrorIs(err, errForcedSecondBalanceTransferCreate)
	s.Require().Equal(2, redeemRepo.createCalls)
	s.Require().True(redeemRepo.firstCreateUsedTxn)
	s.Require().NotZero(redeemRepo.firstCreateID, "the first code must be inserted before the forced failure")

	got, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(20.0, got.Balance)
	s.Require().Equal(8.0, got.GiftBalance)

	persisted, err := s.client.RedeemCode.Query().
		Where(
			redeemcode.CreatedByEQ(user.ID),
			redeemcode.SourceEQ(service.RedeemCodeSourceUserBalanceTransfer),
		).
		Count(s.ctx)
	s.Require().NoError(err)
	s.Require().Zero(persisted)
}

func (s *UserRepoSuite) TestRedeemCreditsOrdinaryAndGiftBalancesWithDifferentRechargeAccounting() {
	user := s.mustCreateUser(&service.User{
		Email:          "gift-redeem-success@example.com",
		Balance:        0,
		GiftBalance:    0,
		TotalRecharged: 0,
	})
	redeemRepo := NewRedeemCodeRepository(s.client)
	ordinary := &service.RedeemCode{
		Code: "REDEEM-ORDINARY-10", Type: service.RedeemTypeBalance, Value: 10,
		Status: service.StatusUnused,
	}
	gift := &service.RedeemCode{
		Code: "REDEEM-GIFT-10", Type: service.RedeemTypeBalance, Value: 10,
		Status: service.StatusUnused, ThresholdExempt: true,
	}
	s.Require().NoError(redeemRepo.Create(s.ctx, ordinary))
	s.Require().NoError(redeemRepo.Create(s.ctx, gift))
	redeemService := service.NewRedeemService(
		redeemRepo,
		s.repo,
		nil,
		nil,
		nil,
		s.client,
		nil,
		nil,
	)

	_, err := redeemService.Redeem(s.ctx, user.ID, ordinary.Code)
	s.Require().NoError(err)
	afterOrdinary, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(10.0, afterOrdinary.Balance)
	s.Require().Zero(afterOrdinary.GiftBalance)
	s.Require().Equal(10.0, afterOrdinary.TotalRecharged)

	_, err = redeemService.Redeem(s.ctx, user.ID, gift.Code)
	s.Require().NoError(err)
	afterGift, err := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(err)
	s.Require().Equal(20.0, afterGift.Balance)
	s.Require().Equal(10.0, afterGift.GiftBalance)
	s.Require().Equal(10.0, afterGift.TotalRecharged)
}

func (s *UserRepoSuite) TestRedeemGiftCreditFailureRollsBackCodeAndWallet() {
	user := s.mustCreateUser(&service.User{
		Email:          "gift-redeem-rollback@example.com",
		Balance:        4,
		GiftBalance:    1,
		TotalRecharged: 2,
	})
	s.Require().NoError(s.client.User.UpdateOneID(user.ID).
		SetGiftBalance(1).
		SetTotalRecharged(2).
		Exec(s.ctx))
	redeemRepo := NewRedeemCodeRepository(s.client)
	code := &service.RedeemCode{
		Code: "REDEEM-GIFT-ROLLBACK", Type: service.RedeemTypeBalance, Value: 10,
		Status: service.StatusUnused, ThresholdExempt: true,
	}
	s.Require().NoError(redeemRepo.Create(s.ctx, code))
	failingUserRepo := &failAfterGiftCreditUserRepository{
		UserRepository:        s.repo,
		GiftBalanceRepository: s.repo,
	}
	redeemService := service.NewRedeemService(
		redeemRepo,
		failingUserRepo,
		nil,
		nil,
		nil,
		s.client,
		nil,
		nil,
	)

	got, err := redeemService.Redeem(s.ctx, user.ID, code.Code)

	s.Require().Nil(got)
	s.Require().ErrorIs(err, errForcedGiftCreditRollback)
	storedUser, getUserErr := s.repo.GetByID(s.ctx, user.ID)
	s.Require().NoError(getUserErr)
	s.Require().Equal(4.0, storedUser.Balance)
	s.Require().Equal(1.0, storedUser.GiftBalance)
	s.Require().Equal(2.0, storedUser.TotalRecharged)
	storedCode, getCodeErr := redeemRepo.GetByID(s.ctx, code.ID)
	s.Require().NoError(getCodeErr)
	s.Require().Equal(service.StatusUnused, storedCode.Status)
	s.Require().Nil(storedCode.UsedBy)
}
