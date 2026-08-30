//go:build unit

package service

import (
	"context"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

type redeemBatchGenerateRepo struct {
	RedeemCodeRepository
	created []RedeemCode
}

func (r *redeemBatchGenerateRepo) Create(_ context.Context, code *RedeemCode) error {
	clone := *code
	clone.ID = int64(len(r.created) + 1)
	code.ID = clone.ID
	r.created = append(r.created, clone)
	return nil
}

func TestAdminServiceGenerateRedeemCodesCreatesIndependentRestrictedBatches(t *testing.T) {
	repo := &redeemBatchGenerateRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	first, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:            2,
		Type:             RedeemTypeBalance,
		Value:            10,
		SingleUsePerUser: true,
	})
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.NotNil(t, first[0].BatchID)
	require.Equal(t, first[0].BatchID, first[1].BatchID)

	second, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:            1,
		Type:             RedeemTypeBalance,
		Value:            10,
		SingleUsePerUser: true,
	})
	require.NoError(t, err)
	require.NotNil(t, second[0].BatchID)
	require.NotEqual(t, *first[0].BatchID, *second[0].BatchID)
}

func TestAdminServiceGenerateRedeemCodesLeavesUnrestrictedCodesWithoutBatch(t *testing.T) {
	repo := &redeemBatchGenerateRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1,
		Type:  RedeemTypeBalance,
		Value: 10,
	})
	require.NoError(t, err)
	require.Nil(t, codes[0].BatchID)
}

func TestAdminServiceGenerateRedeemCodesRejectsRestrictedInvitationBatch(t *testing.T) {
	repo := &redeemBatchGenerateRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:            1,
		Type:             RedeemTypeInvitation,
		SingleUsePerUser: true,
	})
	require.Error(t, err)
	require.Nil(t, codes)
	require.Empty(t, repo.created)
}

func TestGenerateRedeemCodesAppliesThresholdExemptToPositiveBalanceCodes(t *testing.T) {
	repo := &redeemBatchGenerateRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count:           2,
		Type:            RedeemTypeBalance,
		Value:           10,
		ThresholdExempt: true,
	})

	require.NoError(t, err)
	require.Len(t, codes, 2)
	require.Len(t, repo.created, 2)
	for i := range codes {
		require.True(t, codes[i].ThresholdExempt)
		require.True(t, repo.created[i].ThresholdExempt)
	}
}

func TestGenerateRedeemCodesRejectsInvalidThresholdExemptCodes(t *testing.T) {
	tests := []struct {
		name  string
		type_ string
		value float64
	}{
		{name: "concurrency", type_: RedeemTypeConcurrency, value: 1},
		{name: "subscription", type_: RedeemTypeSubscription, value: 1},
		{name: "invitation", type_: RedeemTypeInvitation, value: 1},
		{name: "zero balance", type_: RedeemTypeBalance, value: 0},
		{name: "negative balance", type_: RedeemTypeBalance, value: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &redeemBatchGenerateRepo{}
			svc := &adminServiceImpl{redeemCodeRepo: repo}

			codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
				Count:           1,
				Type:            tt.type_,
				Value:           tt.value,
				ThresholdExempt: true,
			})

			require.Error(t, err)
			require.Nil(t, codes)
			require.Equal(t, "REDEEM_THRESHOLD_EXEMPT_INVALID", infraerrors.Reason(err))
			require.Equal(t, "gift credit is only supported for positive balance redeem codes", infraerrors.Message(err))
			require.Empty(t, repo.created)
		})
	}
}

func TestGenerateRedeemCodesDefaultsThresholdExemptToFalse(t *testing.T) {
	repo := &redeemBatchGenerateRepo{}
	svc := &adminServiceImpl{redeemCodeRepo: repo}

	codes, err := svc.GenerateRedeemCodes(context.Background(), &GenerateRedeemCodesInput{
		Count: 1,
		Type:  RedeemTypeBalance,
		Value: 10,
	})

	require.NoError(t, err)
	require.Len(t, codes, 1)
	require.False(t, codes[0].ThresholdExempt)
	require.Len(t, repo.created, 1)
	require.False(t, repo.created[0].ThresholdExempt)
}
