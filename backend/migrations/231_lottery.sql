CREATE TABLE IF NOT EXISTS lottery_activities (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status VARCHAR(20) NOT NULL DEFAULT 'draft',
    attempt_mode VARCHAR(20) NOT NULL DEFAULT 'daily',
    attempt_limit INTEGER NOT NULL DEFAULT 1,
    starts_at TIMESTAMPTZ,
    ends_at TIMESTAMPTZ,
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_activities_status_check CHECK (status IN ('draft', 'active', 'disabled', 'ended')),
    CONSTRAINT lottery_activities_attempt_mode_check CHECK (attempt_mode IN ('daily', 'total')),
    CONSTRAINT lottery_activities_attempt_limit_check CHECK (attempt_limit > 0),
    CONSTRAINT lottery_activities_dates_check CHECK (starts_at IS NULL OR ends_at IS NULL OR starts_at <= ends_at)
);

CREATE UNIQUE INDEX IF NOT EXISTS lottery_activities_one_active_uq
    ON lottery_activities (status)
    WHERE (status = 'active');

CREATE INDEX IF NOT EXISTS lottery_activities_status_idx
    ON lottery_activities (status);

CREATE INDEX IF NOT EXISTS lottery_activities_attempt_mode_idx
    ON lottery_activities (attempt_mode);

CREATE TABLE IF NOT EXISTS lottery_prizes (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT NOT NULL REFERENCES lottery_activities(id) ON DELETE CASCADE,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    type VARCHAR(20) NOT NULL,
    weight INTEGER NOT NULL DEFAULT 1,
    balance_amount DECIMAL(20,8),
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_prizes_type_check CHECK (type IN ('balance', 'product')),
    CONSTRAINT lottery_prizes_weight_check CHECK (weight > 0),
    CONSTRAINT lottery_prizes_balance_check CHECK (
        type <> 'balance' OR (balance_amount IS NOT NULL AND balance_amount > 0)
    )
);

CREATE INDEX IF NOT EXISTS lottery_prizes_activity_enabled_sort_idx
    ON lottery_prizes (activity_id, enabled, sort_order, id);

CREATE TABLE IF NOT EXISTS lottery_prize_items (
    id BIGSERIAL PRIMARY KEY,
    prize_id BIGINT NOT NULL REFERENCES lottery_prizes(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'available',
    claimed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    claimed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_prize_items_status_check CHECK (status IN ('available', 'claimed'))
);

CREATE INDEX IF NOT EXISTS lottery_prize_items_prize_status_idx
    ON lottery_prize_items (prize_id, status, id);

CREATE TABLE IF NOT EXISTS lottery_draws (
    id BIGSERIAL PRIMARY KEY,
    activity_id BIGINT REFERENCES lottery_activities(id) ON DELETE SET NULL,
    prize_id BIGINT REFERENCES lottery_prizes(id) ON DELETE SET NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prize_name VARCHAR(120) NOT NULL,
    prize_type VARCHAR(20) NOT NULL,
    balance_amount DECIMAL(20,8),
    product_content TEXT,
    attempt_key VARCHAR(128) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT lottery_draws_prize_type_check CHECK (prize_type IN ('balance', 'product'))
);

CREATE UNIQUE INDEX IF NOT EXISTS lottery_draws_attempt_key_uq
    ON lottery_draws (attempt_key);

CREATE INDEX IF NOT EXISTS lottery_draws_user_created_at_idx
    ON lottery_draws (user_id, created_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS lottery_draws_activity_created_at_idx
    ON lottery_draws (activity_id, created_at DESC, id DESC);
