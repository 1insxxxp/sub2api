DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'users_gift_balance_within_balance'
          AND conrelid = 'users'::regclass
    ) THEN
        ALTER TABLE users
            ADD CONSTRAINT users_gift_balance_within_balance
            CHECK (gift_balance <= GREATEST(balance, 0)) NOT VALID;
    END IF;
END $$;
