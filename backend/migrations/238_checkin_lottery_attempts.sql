CREATE TABLE IF NOT EXISTS lottery_attempt_wallets (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    balance INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_attempt_wallets_balance_check CHECK (balance >= 0)
);

CREATE TABLE IF NOT EXISTS lottery_attempt_ledger (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    delta INTEGER NOT NULL,
    balance_after INTEGER NOT NULL,
    source_type VARCHAR(32) NOT NULL,
    source_id BIGINT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_attempt_ledger_delta_check CHECK (delta <> 0),
    CONSTRAINT lottery_attempt_ledger_balance_check CHECK (balance_after >= 0),
    CONSTRAINT lottery_attempt_ledger_source_type_check CHECK (source_type IN ('checkin_streak', 'lottery_draw'))
);

CREATE UNIQUE INDEX IF NOT EXISTS lottery_attempt_ledger_source_uq
    ON lottery_attempt_ledger (source_type, source_id);

CREATE INDEX IF NOT EXISTS lottery_attempt_ledger_user_created_at_idx
    ON lottery_attempt_ledger (user_id, created_at DESC, id DESC);

ALTER TABLE user_checkins
    ADD COLUMN IF NOT EXISTS lottery_attempts_reward INTEGER NOT NULL DEFAULT 0;

ALTER TABLE lottery_draws
    ADD COLUMN IF NOT EXISTS attempt_source VARCHAR(20) NOT NULL DEFAULT 'activity';

UPDATE lottery_draws SET attempt_source = 'activity'
WHERE attempt_source IS NULL OR attempt_source = '';

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'lottery_draws_attempt_source_check'
    ) THEN
        ALTER TABLE lottery_draws
            ADD CONSTRAINT lottery_draws_attempt_source_check
            CHECK (attempt_source IN ('activity', 'wallet'));
    END IF;
END $$;

ALTER TABLE lottery_activities
    DROP CONSTRAINT IF EXISTS lottery_activities_attempt_limit_check;

ALTER TABLE lottery_activities
    ADD CONSTRAINT lottery_activities_attempt_limit_check
    CHECK (attempt_limit >= 0);
