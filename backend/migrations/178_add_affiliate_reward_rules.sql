CREATE TABLE IF NOT EXISTS affiliate_reward_rules (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    required_qualified_invitees INTEGER NOT NULL,
    reward_type VARCHAR(32) NOT NULL,
    balance_value DECIMAL(20,8) NOT NULL DEFAULT 0,
    group_id BIGINT NULL,
    validity_days INTEGER NOT NULL DEFAULT 0,
    redeem_expires_in_days INTEGER NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT affiliate_reward_rules_required_positive CHECK (required_qualified_invitees > 0),
    CONSTRAINT affiliate_reward_rules_type CHECK (reward_type IN ('balance', 'subscription')),
    CONSTRAINT affiliate_reward_rules_balance_valid CHECK (
        reward_type <> 'balance' OR balance_value > 0
    ),
    CONSTRAINT affiliate_reward_rules_subscription_valid CHECK (
        reward_type <> 'subscription' OR (group_id IS NOT NULL AND group_id > 0 AND validity_days > 0)
    ),
    CONSTRAINT affiliate_reward_rules_expiry_nonnegative CHECK (redeem_expires_in_days >= 0)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_reward_rules_enabled_sort
    ON affiliate_reward_rules (enabled, sort_order, required_qualified_invitees, id);

CREATE TABLE IF NOT EXISTS affiliate_reward_claims (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    rule_id BIGINT NOT NULL REFERENCES affiliate_reward_rules(id),
    redeem_code_id BIGINT NOT NULL REFERENCES redeem_codes(id),
    qualified_invitee_count_snapshot INTEGER NOT NULL DEFAULT 0,
    claimed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, rule_id)
);

CREATE INDEX IF NOT EXISTS idx_affiliate_reward_claims_user
    ON affiliate_reward_claims (user_id, claimed_at DESC);
