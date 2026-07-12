package repository

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAffiliateUserOverviewSQLIncludesMaturedFrozenQuota(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateUserOverviewSQL), " ")

	require.Contains(t, query, "ua.aff_quota + COALESCE(matured.matured_frozen_quota, 0)")
	require.Contains(t, query, "frozen_until <= NOW()")
}

func TestAffiliateRecordQueriesUseLedgerAuditFields(t *testing.T) {
	source, err := os.ReadFile("affiliate_repo.go")
	require.NoError(t, err)
	content := string(source)

	require.Contains(t, content, "JOIN payment_orders po ON po.id = ual.source_order_id")
	require.Contains(t, content, "ual.amount::double precision")
	require.Contains(t, content, "ual.balance_after::double precision")
	require.NotContains(t, content, "parseAffiliateRebateAmount")
	require.NotContains(t, content, `"current_balance": "u.balance"`)
}

func TestAffiliateQualificationReconcileSQLMatchesAuthoritativePaymentRules(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateQualificationReconcileSQL), " ")

	require.Contains(t, query, "FOR UPDATE")
	require.Contains(t, query, "FROM payment_orders po CROSS JOIN locked")
	require.Contains(t, query, "po.order_type IN ('balance', 'subscription')")
	require.Contains(t, query, "'COMPLETED', 'REFUND_REQUESTED', 'REFUNDING', 'REFUND_PENDING', 'REFUND_FAILED'")
	require.Contains(t, query, "WHEN po.status = 'PARTIALLY_REFUNDED' THEN GREATEST(po.amount - po.refund_amount, 0)")
	require.Contains(t, query, "WHEN po.status = 'REFUNDED' THEN 0")
	require.Contains(t, query, "GREATEST(COALESCE((SELECT SUM(net_amount) FROM authoritative_orders), 0), 0)")
	require.Contains(t, query, "WHEN totals.amount >= $2 AND locked.qualifying_payment_amount < $2")
	require.Contains(t, query, "WHEN totals.amount < $2 THEN NULL")
}

func TestAffiliateQualificationCountSQLUsesCurrentThreshold(t *testing.T) {
	query := strings.Join(strings.Fields(affiliateQualificationCountSQL), " ")

	require.Contains(t, query, "inviter_id = $1")
	require.Contains(t, query, "qualifying_payment_amount >= $2")
	require.NotContains(t, query, "qualified_at IS NOT NULL")
	require.NotContains(t, query, "aff_count")
}
