package dto

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRedeemCodeFromService_DoesNotExposeRelatedUsers(t *testing.T) {
	t.Parallel()

	usedBy := int64(200)
	createdBy := int64(100)

	out := RedeemCodeFromService(&service.RedeemCode{
		ID:        1,
		Code:      "TRANSFER-CODE",
		Type:      service.RedeemTypeBalance,
		Value:     10,
		Status:    service.StatusUsed,
		UsedBy:    &usedBy,
		CreatedBy: &createdBy,
		User: &service.User{
			ID:      usedBy,
			Email:   "recipient@example.com",
			Balance: 123,
		},
		Creator: &service.User{
			ID:      createdBy,
			Email:   "creator@example.com",
			Balance: 456,
		},
	})

	require.NotNil(t, out)
	require.Equal(t, &usedBy, out.UsedBy)
	require.Equal(t, &createdBy, out.CreatedBy)
	require.Nil(t, out.User)
	require.Nil(t, out.Creator)
}

func TestRedeemCodeFromServiceAdmin_ExposesRelatedUsers(t *testing.T) {
	t.Parallel()

	usedBy := int64(200)
	createdBy := int64(100)

	out := RedeemCodeFromServiceAdmin(&service.RedeemCode{
		ID:        1,
		Code:      "TRANSFER-CODE",
		Type:      service.RedeemTypeBalance,
		Value:     10,
		Status:    service.StatusUsed,
		UsedBy:    &usedBy,
		CreatedBy: &createdBy,
		User: &service.User{
			ID:    usedBy,
			Email: "recipient@example.com",
		},
		Creator: &service.User{
			ID:    createdBy,
			Email: "creator@example.com",
		},
	})

	require.NotNil(t, out)
	require.NotNil(t, out.User)
	require.NotNil(t, out.Creator)
	require.Equal(t, "recipient@example.com", out.User.Email)
	require.Equal(t, "creator@example.com", out.Creator.Email)
}
