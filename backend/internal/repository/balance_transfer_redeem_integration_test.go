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
