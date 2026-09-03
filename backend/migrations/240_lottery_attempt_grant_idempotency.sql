ALTER TABLE lottery_attempt_grants
    ADD COLUMN IF NOT EXISTS request_key VARCHAR(128);

ALTER TABLE lottery_attempt_grants
    ADD COLUMN IF NOT EXISTS target_all BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE lottery_attempt_grants
SET request_key = 'legacy-' || id
WHERE request_key IS NULL;

ALTER TABLE lottery_attempt_grants
    ALTER COLUMN request_key SET NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS lottery_attempt_grants_request_user_uq
    ON lottery_attempt_grants (request_key, user_id);
