-- Private, per-group upstream-account authorization rules for authenticated users.
CREATE TABLE IF NOT EXISTS group_account_user_exclusions (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (group_id, account_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_group_account_user_exclusions_user_group
    ON group_account_user_exclusions (user_id, group_id);

COMMENT ON TABLE group_account_user_exclusions IS 'Private administrator-only account exclusions; apply only to the named group and user.';
