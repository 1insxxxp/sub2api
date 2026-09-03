CREATE TABLE IF NOT EXISTS lottery_attempt_grants (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount INTEGER NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_by BIGINT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_attempt_grants_amount_check CHECK (amount > 0)
);

CREATE INDEX IF NOT EXISTS lottery_attempt_grants_user_created_at_idx
    ON lottery_attempt_grants (user_id, created_at DESC, id DESC);

ALTER TABLE lottery_attempt_ledger
    DROP CONSTRAINT IF EXISTS lottery_attempt_ledger_source_type_check;

ALTER TABLE lottery_attempt_ledger
    ADD CONSTRAINT lottery_attempt_ledger_source_type_check
    CHECK (source_type IN ('checkin_streak', 'lottery_draw', 'admin_grant'));
