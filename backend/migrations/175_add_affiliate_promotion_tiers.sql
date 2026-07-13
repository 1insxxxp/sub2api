-- Keep this transactional migration short: ALTER TABLE requires an ACCESS
-- EXCLUSIVE lock, so historical data work and index creation live in 176/177.
ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS qualifying_payment_amount DECIMAL(20,8) NOT NULL DEFAULT 0;

ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS qualified_at TIMESTAMPTZ NULL;

COMMENT ON COLUMN user_affiliates.qualifying_payment_amount IS
    'Authoritative cumulative balance/subscription payment amount net of refunds';
COMMENT ON COLUMN user_affiliates.qualified_at IS
    'First time currently valid historical payments reached the qualification threshold';

-- Seed the new base rate only when absent. Every existing value is an
-- administrator configuration and must be preserved.
INSERT INTO settings (key, value)
VALUES ('affiliate_rebate_rate', '8')
ON CONFLICT (key) DO NOTHING;
