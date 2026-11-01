SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

-- Authorization is private and independent of the user's general role.
CREATE TABLE IF NOT EXISTS user_support_access (
    user_id BIGINT PRIMARY KEY CHECK (user_id > 0),
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by BIGINT NOT NULL CHECK (updated_by > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Lifetime business idempotency. Intentionally no foreign key to deletable
-- redeem codes, log retention, or temporary HTTP idempotency records.
CREATE TABLE IF NOT EXISTS support_trial_grants (
    user_id BIGINT PRIMARY KEY CHECK (user_id > 0),
    amount NUMERIC(20,8) NOT NULL DEFAULT 15 CHECK (amount = 15),
    operator_id BIGINT NOT NULL CHECK (operator_id > 0),
    redeem_code_id BIGINT NOT NULL CHECK (redeem_code_id > 0),
    legacy BOOLEAN NOT NULL DEFAULT FALSE,
    granted_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE user_support_access IS 'Private, explicit staff access to the safe user-support page without granting an administrator role';
COMMENT ON TABLE support_trial_grants IS 'Permanent one-per-user 15-credit support trial, including historical manual admin +15 evidence';
