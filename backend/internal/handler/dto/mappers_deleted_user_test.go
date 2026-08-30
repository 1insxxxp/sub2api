package dto

import (
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserFromServiceShallow_MapsDeletedAt(t *testing.T) {
	ts := time.Date(2026, 5, 28, 10, 0, 0, 0, time.UTC)

	deleted := UserFromServiceShallow(&service.User{ID: 1, Email: "d@test.com", DeletedAt: &ts})
	require.NotNil(t, deleted.DeletedAt)
	require.Equal(t, ts, *deleted.DeletedAt)

	active := UserFromServiceShallow(&service.User{ID: 2, Email: "a@test.com"})
	require.Nil(t, active.DeletedAt, "active user must have nil DeletedAt")
}

func TestRedeemCodeFromService_HidesUserBalanceTransferAuditFields(t *testing.T) {
	creatorID := int64(9)

	userDTO := RedeemCodeFromService(&service.RedeemCode{
		ID:        1,
		Code:      "AUDIT-HIDDEN",
		Type:      service.RedeemTypeBalance,
		Status:    service.StatusUnused,
		CreatedBy: &creatorID,
		Source:    service.RedeemCodeSourceUserBalanceTransfer,
	})

	require.Nil(t, userDTO.CreatedBy)
	require.Empty(t, userDTO.Source)

	adminDTO := RedeemCodeFromServiceAdmin(&service.RedeemCode{
		ID:        1,
		Code:      "AUDIT-VISIBLE",
		Type:      service.RedeemTypeBalance,
		Status:    service.StatusUnused,
		CreatedBy: &creatorID,
		Source:    service.RedeemCodeSourceUserBalanceTransfer,
	})

	require.NotNil(t, adminDTO.CreatedBy)
	require.Equal(t, creatorID, *adminDTO.CreatedBy)
	require.Equal(t, service.RedeemCodeSourceUserBalanceTransfer, adminDTO.Source)
}

func TestRedeemCodeFromServiceWorkbenchGeneratedIncludesOwnerNotes(t *testing.T) {
	creatorID := int64(9)

	got := RedeemCodeFromServiceWorkbenchGenerated(&service.RedeemCode{
		ID:        1,
		Code:      "OWNER-NOTE",
		Type:      service.RedeemTypeBalance,
		Status:    service.StatusUnused,
		CreatedBy: &creatorID,
		Source:    service.RedeemCodeSourceUserBalanceTransfer,
		Notes:     "send to partner A",
	})

	require.NotNil(t, got.Notes)
	require.Equal(t, "send to partner A", *got.Notes)
	require.Nil(t, got.CreatedBy)
	require.Empty(t, got.Source)
}
