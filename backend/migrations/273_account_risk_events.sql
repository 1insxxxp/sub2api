CREATE TABLE account_risk_events (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('register','login','checkin')),
    browser_hash VARCHAR(64) NOT NULL DEFAULT '',
    ip_address VARCHAR(64) NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX account_risk_events_created_idx ON account_risk_events (created_at);
CREATE INDEX account_risk_events_user_time_idx ON account_risk_events (user_id, created_at DESC);
CREATE INDEX account_risk_registration_ip_time_idx ON account_risk_events (ip_address, created_at DESC) WHERE event_type='register';
CREATE INDEX checkin_browser_claims_time_idx ON checkin_browser_claims (created_at);
CREATE INDEX checkin_browser_reviews_time_idx ON checkin_browser_reviews (created_at);
CREATE INDEX checkin_browser_audit_time_idx ON checkin_browser_audit (created_at);

-- Hourly distinct IPs keep interactive association searches off the usage ledger.
CREATE TABLE account_risk_usage_ips (
    bucket TIMESTAMPTZ NOT NULL,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ip_address VARCHAR(64) NOT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (bucket,user_id,ip_address)
);
CREATE TABLE account_risk_usage_windows (
    bucket TIMESTAMPTZ PRIMARY KEY,
    refreshed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
