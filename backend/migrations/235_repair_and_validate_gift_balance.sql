UPDATE users
SET gift_balance = LEAST(gift_balance, GREATEST(balance, 0))
WHERE gift_balance > GREATEST(balance, 0);

ALTER TABLE users
    VALIDATE CONSTRAINT users_gift_balance_within_balance;
