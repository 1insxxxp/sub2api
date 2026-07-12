-- Persist each invitee relationship's authoritative real-money payment total.
-- Runtime tier resolution compares this durable amount with the configured
-- qualification threshold; qualified_at records when the rollout threshold
-- of 50 was first reached by the currently valid historical payments.
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS qualifying_payment_amount DECIMAL(20,8) NOT NULL DEFAULT 0;

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS qualified_at TIMESTAMPTZ NULL;

COMMENT ON COLUMN user_affiliates.qualifying_payment_amount IS
    'Authoritative cumulative balance/subscription payment amount net of refunds';
COMMENT ON COLUMN user_affiliates.qualified_at IS
    'First time currently valid historical payments reached the rollout threshold';

-- Tier counts query qualified relationships by inviter. The predicate keeps
-- unqualified affiliate rows out of this lookup index.
CREATE INDEX IF NOT EXISTS idx_user_affiliates_inviter_qualified
    ON user_affiliates (inviter_id)
    WHERE qualified_at IS NOT NULL;

-- Recompute rather than increment so replaying the migration cannot duplicate
-- historical payment amounts. Only relationships with an inviter participate.
WITH inviter_bound AS (
    SELECT ua.user_id
    FROM user_affiliates ua
    WHERE ua.inviter_id IS NOT NULL
),
authoritative_orders AS (
    SELECT
        po.user_id,
        po.id AS order_id,
        COALESCE(po.completed_at, po.paid_at, po.created_at) AS payment_at,
        CASE po.status
            WHEN 'COMPLETED' THEN po.amount
            WHEN 'PARTIALLY_REFUNDED' THEN GREATEST(po.amount - po.refund_amount, 0)
            WHEN 'REFUNDED' THEN 0
            ELSE 0
        END AS net_amount
    FROM payment_orders po
    JOIN inviter_bound ib ON ib.user_id = po.user_id
    WHERE po.order_type IN ('balance', 'subscription')
),
running_payments AS (
    SELECT
        user_id,
        payment_at,
        SUM(net_amount) OVER (
            PARTITION BY user_id
            ORDER BY payment_at, order_id
            ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW
        ) AS cumulative_amount
    FROM authoritative_orders
),
backfill AS (
    SELECT
        ib.user_id,
        COALESCE(MAX(rp.cumulative_amount), 0)::DECIMAL(20,8) AS qualifying_payment_amount,
        MIN(rp.payment_at) FILTER (WHERE rp.cumulative_amount >= 50) AS qualified_at
    FROM inviter_bound ib
    LEFT JOIN running_payments rp ON rp.user_id = ib.user_id
    GROUP BY ib.user_id
)
UPDATE user_affiliates ua
SET qualifying_payment_amount = backfill.qualifying_payment_amount,
    qualified_at = CASE
        WHEN backfill.qualifying_payment_amount >= 50
            THEN COALESCE(ua.qualified_at, backfill.qualified_at)
        ELSE NULL
    END
FROM backfill
WHERE ua.user_id = backfill.user_id;

-- Seed the new base rate when absent. Existing administrator values are left
-- intact except for the legacy rollout default, including numeric forms such
-- as 10.0, which moves from 10 percent to 8 percent.
INSERT INTO settings (key, value)
VALUES ('affiliate_rebate_rate', '8')
ON CONFLICT (key) DO NOTHING;

UPDATE settings
SET value = '8',
    updated_at = NOW()
WHERE key = 'affiliate_rebate_rate'
  AND value ~ '^[[:space:]]*[+-]?([0-9]+([.][0-9]*)?|[.][0-9]+)[[:space:]]*$'
  AND value::numeric = 10;
