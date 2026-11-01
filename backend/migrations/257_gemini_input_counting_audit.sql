-- Preserve the direct upstream count independently of discounted customer input.
-- Unknown historical counts remain NULL; no usage or cost history is rewritten.
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE usage_response_outcomes
    ADD COLUMN IF NOT EXISTS upstream_input_tokens INTEGER,
    ADD COLUMN IF NOT EXISTS input_token_policy VARCHAR(64) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS input_token_policy_count INTEGER;

COMMENT ON COLUMN usage_response_outcomes.upstream_input_tokens IS
    'Original reported uncached upstream input count before customer counting policy; NULL means no audit count captured';
COMMENT ON COLUMN usage_response_outcomes.input_token_policy IS
    'Internal customer input counting policy version; empty means no policy audit captured';
COMMENT ON COLUMN usage_response_outcomes.input_token_policy_count IS
    'Local counting policy candidate before the only-decrease cap; NULL means no policy count captured';
COMMENT ON COLUMN usage_response_outcomes.collector_version IS
    'Response evidence collector version; 0 denotes token-source or counting-policy audit without response evidence';
