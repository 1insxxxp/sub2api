//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type redeemBatchGenerateRepo struct {
	*redeemRepoStub
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
	repo := &redeemBatchGenerateRepo{redeemRepoStub: &redeemRepoStub{}}
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
	repo := &redeemBatchGenerateRepo{redeemRepoStub: &redeemRepoStub{}}
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
	repo := &redeemBatchGenerateRepo{redeemRepoStub: &redeemRepoStub{}}
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
