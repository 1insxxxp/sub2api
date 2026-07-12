-- A failed concurrent build can leave an INVALID index that IF NOT EXISTS
-- would skip on retry. This migration is retried only while it is unrecorded,
-- so dropping any same-name remnant first makes failure recovery idempotent.
DROP INDEX CONCURRENTLY IF EXISTS idx_user_affiliates_inviter_qualifying_amount;

-- Build without blocking writes to invitee relationships used by tier counts.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_user_affiliates_inviter_qualifying_amount
    ON user_affiliates (inviter_id, qualifying_payment_amount);
