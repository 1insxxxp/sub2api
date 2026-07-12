package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration175AddsAffiliatePromotionTiers(t *testing.T) {
	content, err := FS.ReadFile("175_add_affiliate_promotion_tiers.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS qualifying_payment_amount DECIMAL(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS qualified_at TIMESTAMPTZ NULL")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_user_affiliates_inviter_qualified")
	require.Contains(t, sql, "ON user_affiliates (inviter_id)")
	require.Contains(t, sql, "WHERE qualified_at IS NOT NULL")

	require.Contains(t, sql, "FROM user_affiliates ua")
	require.Contains(t, sql, "ua.inviter_id IS NOT NULL")
	require.Contains(t, sql, "FROM payment_orders po")
	require.Contains(t, sql, "po.order_type IN ('balance', 'subscription')")
	require.Contains(t, sql, "WHEN 'COMPLETED' THEN po.amount")
	require.Contains(t, sql, "WHEN 'PARTIALLY_REFUNDED' THEN GREATEST(po.amount - po.refund_amount, 0)")
	require.Contains(t, sql, "WHEN 'REFUNDED' THEN 0")
	require.Contains(t, sql, "ELSE 0")
	require.Contains(t, sql, "SUM(net_amount) OVER")
	require.Contains(t, sql, "cumulative_amount >= 50")
	require.Contains(t, sql, "SET qualifying_payment_amount = backfill.qualifying_payment_amount")
	require.Contains(t, sql, "COALESCE(ua.qualified_at, backfill.qualified_at)")
	require.NotContains(t, sql, "redeem_codes")

	require.Contains(t, sql, "INSERT INTO settings (key, value)")
	require.Contains(t, sql, "VALUES ('affiliate_rebate_rate', '8')")
	require.Contains(t, sql, "ON CONFLICT (key) DO NOTHING")
	require.Contains(t, sql, "value::numeric = 10")
	require.NotContains(t, sql, "ON CONFLICT (key) DO UPDATE")
}
