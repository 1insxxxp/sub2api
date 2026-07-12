package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration175AddsAffiliatePromotionTierStorage(t *testing.T) {
	content, err := FS.ReadFile("175_add_affiliate_promotion_tiers.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS qualifying_payment_amount DECIMAL(20,8) NOT NULL DEFAULT 0")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS qualified_at TIMESTAMPTZ NULL")
	require.Contains(t, sql, "VALUES ('affiliate_rebate_rate', '8')")
	require.Contains(t, sql, "value::numeric = 10")
	require.NotContains(t, sql, "payment_orders")
	require.NotContains(t, sql, "CREATE INDEX")
}

func TestMigration176BackfillsAffiliatePromotionTiers(t *testing.T) {
	content, err := FS.ReadFile("176_backfill_affiliate_promotion_tiers.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "UPDATE user_affiliates")
	require.Contains(t, sql, "affiliate_tier_reconcile_required")
	require.Contains(t, sql, "'true'")
	require.NotContains(t, sql, "ALTER TABLE")
	require.NotContains(t, sql, "CREATE INDEX")
}

func TestMigration177AddsAffiliateQualifiedLookupIndexConcurrently(t *testing.T) {
	content, err := FS.ReadFile("177_add_affiliate_qualified_lookup_index_notx.sql")
	require.NoError(t, err)

	sql := string(content)
	dropStatement := "DROP INDEX CONCURRENTLY IF EXISTS idx_user_affiliates_inviter_qualified"
	createStatement := "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliates_inviter_qualified"
	require.Contains(t, sql, dropStatement)
	require.Contains(t, sql, createStatement)
	require.Less(t, strings.Index(sql, dropStatement), strings.Index(sql, createStatement))
	require.Contains(t, sql, "ON user_affiliates (inviter_id)")
	require.Contains(t, sql, "WHERE qualified_at IS NOT NULL")
	require.NotContains(t, sql, "UPDATE ")
	require.NotContains(t, sql, "ALTER TABLE")
}
