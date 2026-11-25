CREATE TABLE account_registration_cooldowns (
    ip_address VARCHAR(64) PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX account_registration_cooldowns_expiry_idx ON account_registration_cooldowns (expires_at);

-- Only successful server-side CAPTCHA verification can create a receipt.
-- The registration transaction consumes it even when no challenge is required.
CREATE TABLE account_registration_captcha (
    ip_address VARCHAR(64) NOT NULL,
    browser_hash VARCHAR(64) NOT NULL,
    scope TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (ip_address, browser_hash, scope)
);
CREATE INDEX account_registration_captcha_expiry_idx ON account_registration_captcha (expires_at);

CREATE TABLE account_registration_risk_counts (
    day DATE NOT NULL,
    action VARCHAR(16) NOT NULL,
    hits BIGINT NOT NULL,
    PRIMARY KEY (day, action)
);
