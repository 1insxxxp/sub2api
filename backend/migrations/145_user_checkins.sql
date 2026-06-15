CREATE TABLE IF NOT EXISTS user_checkins (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date   VARCHAR(10) NOT NULL,
    reward_amount  DECIMAL(20,8) NOT NULL DEFAULT 0,
    balance_before DECIMAL(20,8) NOT NULL DEFAULT 0,
    balance_after  DECIMAL(20,8) NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS user_checkins_user_id_date_uq
    ON user_checkins (user_id, checkin_date);

CREATE INDEX IF NOT EXISTS user_checkins_checkin_date_idx
    ON user_checkins (checkin_date);

CREATE INDEX IF NOT EXISTS user_checkins_user_id_idx
    ON user_checkins (user_id);

CREATE TABLE IF NOT EXISTS user_checkin_blacklist (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    reason      TEXT,
    created_by  BIGINT,
    removed_by  BIGINT,
    removed_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_checkin_blacklist_active_user
    ON user_checkin_blacklist (user_id)
    WHERE removed_at IS NULL;

CREATE INDEX IF NOT EXISTS user_checkin_blacklist_user_id_idx
    ON user_checkin_blacklist (user_id);

CREATE INDEX IF NOT EXISTS user_checkin_blacklist_removed_at_idx
    ON user_checkin_blacklist (removed_at);
