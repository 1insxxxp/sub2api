ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS min_total_recharge_usd NUMERIC(20, 8) NOT NULL DEFAULT 0;

ALTER TABLE groups
    DROP CONSTRAINT IF EXISTS groups_min_total_recharge_usd_nonnegative;

ALTER TABLE groups
    ADD CONSTRAINT groups_min_total_recharge_usd_nonnegative
    CHECK (min_total_recharge_usd >= 0);
