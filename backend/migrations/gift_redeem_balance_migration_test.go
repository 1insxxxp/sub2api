package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration232AddsGiftRedeemEligibilityAccounting(t *testing.T) {
	content, err := FS.ReadFile("232_gift_redeem_balance_eligibility.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS gift_balance NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS frozen_gift_balance NUMERIC(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS threshold_exempt BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS threshold_exempt_cost NUMERIC(20,10) NOT NULL DEFAULT 0")

	constraints := []struct {
		table  string
		name   string
		column string
	}{
		{"users", "users_gift_balance_nonnegative", "gift_balance"},
		{"users", "users_frozen_gift_balance_nonnegative", "frozen_gift_balance"},
		{"usage_logs", "usage_logs_threshold_exempt_cost_nonnegative", "threshold_exempt_cost"},
	}
	for _, constraint := range constraints {
		require.Contains(t, sql, "conname = '"+constraint.name+"' AND conrelid = '"+constraint.table+"'::regclass")
		require.Contains(t, sql, "ADD CONSTRAINT "+constraint.name+" CHECK ("+constraint.column+" >= 0)")
	}
}
