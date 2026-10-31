SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

-- Independent page visibility for second-level administrators. New grants
-- default off; the migration preserves the two pages existing administrators
-- could already access, without touching commission calculation or balances.
CREATE TABLE IF NOT EXISTS sub_admin_workbench_access (
    user_id BIGINT PRIMARY KEY CHECK (user_id > 0),
    commission_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    affiliate_leaderboard_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    updated_by BIGINT CHECK (updated_by IS NULL OR updated_by > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

INSERT INTO sub_admin_workbench_access (
    user_id, commission_enabled, affiliate_leaderboard_enabled, updated_by
)
SELECT id, TRUE, TRUE, NULL
FROM users
WHERE role = 'sub_admin' AND deleted_at IS NULL
ON CONFLICT (user_id) DO NOTHING;

COMMENT ON TABLE sub_admin_workbench_access IS 'Independent commission and affiliate leaderboard access for second-level administrators; NULL actor denotes the migration snapshot';
COMMENT ON TABLE user_support_access IS 'Explicit safe user-support access for second-level administrators only; active main administrators have automatic access and ordinary users are never authorized';
