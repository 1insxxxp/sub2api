-- Group opt-in is independent of shared account configuration. Existing groups
-- keep their original customer accounting until an administrator enables it.
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS gemini_input_counting_enabled BOOLEAN NOT NULL DEFAULT FALSE;

COMMENT ON COLUMN groups.gemini_input_counting_enabled IS
    'Billing-group opt-in for the Gemini text-only, only-decrease customer input token policy';
