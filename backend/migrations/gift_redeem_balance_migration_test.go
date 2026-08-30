package migrations

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration232AddsGiftRedeemEligibilityAccounting(t *testing.T) {
	content, err := FS.ReadFile("232_gift_redeem_balance_eligibility.sql")
	require.NoError(t, err)
	sum := sha256.Sum256([]byte(strings.TrimSpace(string(content))))
	require.Equal(t, "4a05939396c8a8d69ebc14eeec30260254ba93477e4bc5a6a7386e2dcb9a33c4", hex.EncodeToString(sum[:]))

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
		require.Contains(t, sql, "ADD CONSTRAINT "+constraint.name+" CHECK ("+constraint.column+" >= 0) NOT VALID")
	}
	require.NotContains(t, sql, "users_gift_balance_within_balance")
	require.NotContains(t, sql, "VALIDATE CONSTRAINT")
}

func TestMigration234RepairsAndConstrainsGiftBalance(t *testing.T) {
	content, err := FS.ReadFile("234_enforce_gift_balance_within_balance.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "UPDATE users SET gift_balance = LEAST(gift_balance, GREATEST(balance, 0))")
	require.Contains(t, sql, "ADD CONSTRAINT users_gift_balance_within_balance CHECK (gift_balance <= GREATEST(balance, 0))")
	require.Contains(t, sql, "VALIDATE CONSTRAINT users_gift_balance_within_balance")
	require.NotContains(t, sql, "NOT VALID")
}
