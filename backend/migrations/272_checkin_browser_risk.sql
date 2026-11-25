CREATE TABLE IF NOT EXISTS checkin_browser_claims (
    browser_hash VARCHAR(64) NOT NULL,
    checkin_date VARCHAR(10) NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (browser_hash, checkin_date)
);
CREATE TABLE IF NOT EXISTS checkin_browser_reviews (
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    browser_hash VARCHAR(64) NOT NULL,
    checkin_date VARCHAR(10) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at TIMESTAMPTZ,
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (user_id, browser_hash, checkin_date)
);
CREATE INDEX IF NOT EXISTS checkin_browser_reviews_status_created_idx ON checkin_browser_reviews (status, created_at DESC);
CREATE TABLE IF NOT EXISTS checkin_browser_whitelist (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS checkin_browser_audit (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    browser_hash VARCHAR(64) NOT NULL,
    checkin_date VARCHAR(10) NOT NULL,
    action VARCHAR(24) NOT NULL,
    actor_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reason TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
