-- Recompute rather than increment so replaying this migration cannot duplicate
-- historical payment amounts. Only relationships with an inviter participate.
-- This intentionally takes a normal PostgreSQL statement snapshot and does not
-- lock payment_orders, so online payment processing remains available.
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
        CASE
            WHEN po.status IN (
                'COMPLETED',
                'REFUND_REQUESTED',
                'REFUNDING',
                'REFUND_PENDING',
                'REFUND_FAILED'
            ) THEN po.amount
            WHEN po.status = 'PARTIALLY_REFUNDED'
                THEN GREATEST(po.amount - po.refund_amount, 0)
            WHEN po.status = 'REFUNDED' THEN 0
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

-- Concurrent payment changes can commit outside the snapshot above. Runtime
-- startup/on-demand reconciliation (Task 3) consumes and clears this marker
-- after rebuilding qualification from authoritative payment_orders.
INSERT INTO settings (key, value)
VALUES ('affiliate_tier_reconcile_generation', '1')
ON CONFLICT (key) DO NOTHING;

INSERT INTO settings (key, value)
VALUES ('affiliate_tier_reconcile_required', 'true')
ON CONFLICT (key) DO UPDATE
SET value = 'true',
    updated_at = NOW();
