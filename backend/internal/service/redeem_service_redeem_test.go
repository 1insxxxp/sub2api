package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/stretchr/testify/require"
)

type giftRedeemUserRepo struct {
	*userRepoStub
	updateAmounts []float64
	giftAmounts   []float64
	giftErr       error
}

func (r *giftRedeemUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.updateAmounts = append(r.updateAmounts, amount)
	return nil
}

func (r *giftRedeemUserRepo) CreditGiftBalance(_ context.Context, _ int64, amount float64) error {
	r.giftAmounts = append(r.giftAmounts, amount)
	return r.giftErr
}

type ordinaryOnlyRedeemUserRepo struct {
	*userRepoStub
	updateAmounts []float64
}

func (r *ordinaryOnlyRedeemUserRepo) UpdateBalance(_ context.Context, _ int64, amount float64) error {
	r.updateAmounts = append(r.updateAmounts, amount)
	return nil
}

type redeemAuthCacheInvalidator struct {
	userIDs []int64
}

func (*redeemAuthCacheInvalidator) InvalidateAuthCacheByKey(context.Context, string) {}

func (i *redeemAuthCacheInvalidator) InvalidateAuthCacheByUserID(_ context.Context, userID int64) {
	i.userIDs = append(i.userIDs, userID)
}

func (*redeemAuthCacheInvalidator) InvalidateAuthCacheByGroupID(context.Context, int64) {}

func redeemServiceForCode(
	t *testing.T,
	code RedeemCode,
	userRepo UserRepository,
	invalidator APIKeyAuthCacheInvalidator,
) (*RedeemService, *batchRedeemRepo) {
	t.Helper()
	redeemRepo := &batchRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes:            map[string]*RedeemCode{code.Code: &code},
	}
	return NewRedeemService(
		redeemRepo,
		userRepo,
		nil,
		nil,
		nil,
		newCheckinServiceTestClient(t),
		invalidator,
		nil,
	), redeemRepo
}

func TestRedeemGiftBalanceUsesGiftCreditAndInvalidatesCache(t *testing.T) {
	const userID int64 = 7
	userRepo := &giftRedeemUserRepo{userRepoStub: &userRepoStub{user: &User{ID: userID}}}
	invalidator := &redeemAuthCacheInvalidator{}
	svc, _ := redeemServiceForCode(t, RedeemCode{
		ID:              301,
		Code:            "GIFT-10",
		Type:            RedeemTypeBalance,
		Value:           10,
		Status:          StatusUnused,
		ThresholdExempt: true,
	}, userRepo, invalidator)

	got, err := svc.Redeem(context.Background(), userID, "GIFT-10")

	require.NoError(t, err)
	require.Equal(t, "GIFT-10", got.Code)
	require.Equal(t, []float64{10}, userRepo.giftAmounts)
	require.Empty(t, userRepo.updateAmounts)
	require.Equal(t, []int64{userID}, invalidator.userIDs)
}

func TestRedeemOrdinaryBalanceKeepsCumulativeCreditPath(t *testing.T) {
	const userID int64 = 7
	userRepo := &giftRedeemUserRepo{userRepoStub: &userRepoStub{user: &User{ID: userID}}}
	svc, _ := redeemServiceForCode(t, RedeemCode{
		ID:     302,
		Code:   "ORDINARY-10",
		Type:   RedeemTypeBalance,
		Value:  10,
		Status: StatusUnused,
	}, userRepo, nil)

	_, err := svc.Redeem(context.Background(), userID, "ORDINARY-10")

	require.NoError(t, err)
	require.Equal(t, []float64{10}, userRepo.updateAmounts)
	require.Empty(t, userRepo.giftAmounts)
}

func TestRedeemGiftBalanceFailsWithoutGiftRepository(t *testing.T) {
	const userID int64 = 7
	userRepo := &ordinaryOnlyRedeemUserRepo{userRepoStub: &userRepoStub{user: &User{ID: userID}}}
	svc, redeemRepo := redeemServiceForCode(t, RedeemCode{
		ID:              303,
		Code:            "GIFT-NO-CAPABILITY",
		Type:            RedeemTypeBalance,
		Value:           10,
		Status:          StatusUnused,
		ThresholdExempt: true,
	}, userRepo, nil)

	got, err := svc.Redeem(context.Background(), userID, "GIFT-NO-CAPABILITY")

	require.Nil(t, got)
	require.ErrorContains(t, err, "does not support gift balance credit")
	require.Empty(t, userRepo.updateAmounts)
	require.Equal(t, StatusUnused, redeemRepo.codes["GIFT-NO-CAPABILITY"].Status)
}

func TestRedeemRejectsCorruptGiftBalanceCodesBeforeUse(t *testing.T) {
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), 0.000000001, 1_000_000_000_000} {
		t.Run("invalid_value", func(t *testing.T) {
			const userID int64 = 7
			userRepo := &giftRedeemUserRepo{userRepoStub: &userRepoStub{user: &User{ID: userID}}}
			svc, redeemRepo := redeemServiceForCode(t, RedeemCode{
				ID:              304,
				Code:            "CORRUPT-GIFT",
				Type:            RedeemTypeBalance,
				Value:           value,
				Status:          StatusUnused,
				ThresholdExempt: true,
			}, userRepo, nil)

			got, err := svc.Redeem(context.Background(), userID, "CORRUPT-GIFT")

			require.Nil(t, got)
			require.Error(t, err)
			require.True(t, errors.Is(err, ErrRedeemCodeInvalidGiftBalance))
			require.Empty(t, userRepo.updateAmounts)
			require.Empty(t, userRepo.giftAmounts)
			require.Equal(t, StatusUnused, redeemRepo.codes["CORRUPT-GIFT"].Status)
		})
	}
}

func TestRedeemRejectsThresholdExemptNonBalanceCodesBeforeUse(t *testing.T) {
	groupID := int64(11)
	for _, code := range []RedeemCode{
		{
			ID: 305, Code: "CORRUPT-GIFT-CONCURRENCY", Type: RedeemTypeConcurrency,
			Value: 1, Status: StatusUnused, ThresholdExempt: true,
		},
		{
			ID: 306, Code: "CORRUPT-GIFT-SUBSCRIPTION", Type: RedeemTypeSubscription,
			Value: 1, Status: StatusUnused, ThresholdExempt: true, GroupID: &groupID,
		},
	} {
		t.Run(code.Type, func(t *testing.T) {
			const userID int64 = 7
			userRepo := &giftRedeemUserRepo{userRepoStub: &userRepoStub{user: &User{ID: userID}}}
			svc, redeemRepo := redeemServiceForCode(t, code, userRepo, nil)

			got, err := svc.Redeem(context.Background(), userID, code.Code)

			require.Nil(t, got)
			require.ErrorIs(t, err, ErrRedeemCodeInvalidGiftBalance)
			require.Empty(t, userRepo.updateAmounts)
			require.Empty(t, userRepo.giftAmounts)
			require.Equal(t, StatusUnused, redeemRepo.codes[code.Code].Status)
		})
	}
}

type redeemRejectRepo struct {
	code      RedeemCode
	useCalled bool
}

func (r *redeemRejectRepo) Create(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Create call")
}

func (r *redeemRejectRepo) CreateBatch(ctx context.Context, codes []RedeemCode) error {
	panic("unexpected CreateBatch call")
}

func (r *redeemRejectRepo) GetByID(ctx context.Context, id int64) (*RedeemCode, error) {
	if r.code.ID != id {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}

func (r *redeemRejectRepo) GetByCode(ctx context.Context, code string) (*RedeemCode, error) {
	if r.code.Code != code {
		return nil, ErrRedeemCodeNotFound
	}
	clone := r.code
	return &clone, nil
}

func (r *redeemRejectRepo) Update(ctx context.Context, code *RedeemCode) error {
	panic("unexpected Update call")
}

func (r *redeemRejectRepo) BatchUpdate(ctx context.Context, ids []int64, fields RedeemCodeBatchUpdateFields) (int64, error) {
	panic("unexpected BatchUpdate call")
}

func (r *redeemRejectRepo) Delete(ctx context.Context, id int64) error {
	panic("unexpected Delete call")
}

func (r *redeemRejectRepo) Use(ctx context.Context, id, userID int64) error {
	r.useCalled = true
	r.code.Status = StatusUsed
	r.code.UsedBy = &userID
	return nil
}

func (r *redeemRejectRepo) List(ctx context.Context, params pagination.PaginationParams) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected List call")
}

func (r *redeemRejectRepo) ListWithFilters(ctx context.Context, params pagination.PaginationParams, codeType, status, search string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListWithFilters call")
}

func (r *redeemRejectRepo) ListByUser(ctx context.Context, userID int64, limit int) ([]RedeemCode, error) {
	panic("unexpected ListByUser call")
}

func (r *redeemRejectRepo) ListByUserPaginated(ctx context.Context, userID int64, params pagination.PaginationParams, codeType string) ([]RedeemCode, *pagination.PaginationResult, error) {
	panic("unexpected ListByUserPaginated call")
}

func (r *redeemRejectRepo) SumPositiveBalanceByUser(ctx context.Context, userID int64) (float64, error) {
	panic("unexpected SumPositiveBalanceByUser call")
}

func TestRedeemRejectsInvitationCodeBeforeTransaction(t *testing.T) {
	ctx := context.Background()
	redeemRepo := &redeemRejectRepo{
		code: RedeemCode{
			ID:     1,
			Code:   "INVITE-001",
			Type:   RedeemTypeInvitation,
			Status: StatusUnused,
		},
	}
	redeemService := NewRedeemService(redeemRepo, nil, nil, nil, nil, nil, nil, nil)

	got, err := redeemService.Redeem(ctx, 2, redeemRepo.code.Code)

	require.Nil(t, got)
	require.Error(t, err)
	require.True(t, infraerrors.IsBadRequest(err))
	require.Equal(t, "REDEEM_CODE_UNSUPPORTED_TYPE", infraerrors.Reason(err))
	require.Equal(t, "invitation codes can only be used during registration", infraerrors.Message(err))
	require.False(t, redeemRepo.useCalled)
	require.Equal(t, StatusUnused, redeemRepo.code.Status)
	require.Nil(t, redeemRepo.code.UsedBy)
}

func TestRedeemBalanceCodeReconcilesAffiliateQualification(t *testing.T) {
	ctx := context.Background()
	client := newCheckinServiceTestClient(t)
	inviterID := int64(1)
	inviteeID := int64(2)
	code := &RedeemCode{
		ID:     201,
		Code:   "BALANCE-500",
		Type:   RedeemTypeBalance,
		Value:  500,
		Status: StatusUnused,
	}
	redeemRepo := &batchRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[string]*RedeemCode{
			code.Code: code,
		},
	}
	userRepo := &batchRedeemUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: inviteeID}},
	}
	affiliateRepo := &affiliateTierServiceRepoStub{
		qualifiedCount: 10,
		inviteeSummary: &AffiliateSummary{
			UserID:    inviteeID,
			InviterID: &inviterID,
			CreatedAt: time.Now().Add(-time.Hour),
		},
		inviterSummary: &AffiliateSummary{
			UserID:    inviterID,
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
	}
	settingRepo := newAffiliateTierServiceSettingRepo()
	settingRepo.values[SettingKeyAffiliateEnabled] = "true"
	affiliateService := NewAffiliateService(affiliateRepo, NewSettingService(settingRepo, nil), nil, nil)
	redeemService := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, affiliateService)

	result, err := redeemService.Redeem(ctx, inviteeID, code.Code)

	require.NoError(t, err)
	require.Equal(t, code.Code, result.Code)
	require.Equal(t, 500.0, userRepo.balance)
	require.Equal(t, 60.0, affiliateRepo.accruedAmount)
	require.Equal(t, 1, affiliateRepo.reconcileInviteeCalls)
}

func TestRedeemSkipsAffiliateForBalanceTransferCode(t *testing.T) {
	ctx := context.Background()
	client := newCheckinServiceTestClient(t)
	inviterID := int64(1)
	inviteeID := int64(2)
	code := &RedeemCode{
		ID:     202,
		Code:   "TRANSFER-500",
		Type:   RedeemTypeBalance,
		Value:  500,
		Status: StatusUnused,
		Source: RedeemCodeSourceUserBalanceTransfer,
	}
	redeemRepo := &batchRedeemRepo{
		redeemRejectRepo: &redeemRejectRepo{},
		codes: map[string]*RedeemCode{
			code.Code: code,
		},
	}
	userRepo := &batchRedeemUserRepo{
		userRepoStub: &userRepoStub{user: &User{ID: inviteeID}},
	}
	affiliateRepo := &affiliateTierServiceRepoStub{
		qualifiedCount: 10,
		inviteeSummary: &AffiliateSummary{
			UserID:    inviteeID,
			InviterID: &inviterID,
			CreatedAt: time.Now().Add(-time.Hour),
		},
		inviterSummary: &AffiliateSummary{
			UserID:    inviterID,
			CreatedAt: time.Now().Add(-24 * time.Hour),
		},
	}
	settingRepo := newAffiliateTierServiceSettingRepo()
	settingRepo.values[SettingKeyAffiliateEnabled] = "true"
	affiliateService := NewAffiliateService(affiliateRepo, NewSettingService(settingRepo, nil), nil, nil)
	redeemService := NewRedeemService(redeemRepo, userRepo, nil, nil, nil, client, nil, affiliateService)

	result, err := redeemService.Redeem(ctx, inviteeID, code.Code)

	require.NoError(t, err)
	require.Equal(t, code.Code, result.Code)
	require.Equal(t, 500.0, userRepo.balance)
	require.Zero(t, affiliateRepo.accruedAmount)
	require.Zero(t, affiliateRepo.reconcileInviteeCalls)
}
