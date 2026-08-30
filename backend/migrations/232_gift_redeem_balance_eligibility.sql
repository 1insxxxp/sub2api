ALTER TABLE users
    ADD COLUMN IF NOT EXISTS gift_balance NUMERIC(20,8) NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS frozen_gift_balance NUMERIC(20,8) NOT NULL DEFAULT 0;

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS threshold_exempt BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS threshold_exempt_cost NUMERIC(20,10) NOT NULL DEFAULT 0;

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_gift_balance_nonnegative' AND conrelid = 'users'::regclass) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_gift_balance_nonnegative CHECK (gift_balance >= 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_frozen_gift_balance_nonnegative' AND conrelid = 'users'::regclass) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_frozen_gift_balance_nonnegative CHECK (frozen_gift_balance >= 0);
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'usage_logs_threshold_exempt_cost_nonnegative' AND conrelid = 'usage_logs'::regclass) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_threshold_exempt_cost_nonnegative CHECK (threshold_exempt_cost >= 0);
    END IF;
END $$;
