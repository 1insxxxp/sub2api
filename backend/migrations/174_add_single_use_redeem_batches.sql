-- Restricted activity batches let each account redeem at most one code from a
-- generation operation. Claims intentionally do not reference redeem_codes so
-- deleting a used code cannot restore eligibility for the same batch.

ALTER TABLE redeem_codes
    ADD COLUMN IF NOT EXISTS batch_id VARCHAR(64);

CREATE INDEX IF NOT EXISTS redeemcode_batch_id
    ON redeem_codes(batch_id);

CREATE TABLE IF NOT EXISTS redeem_batch_claims (
    id             BIGSERIAL PRIMARY KEY,
    batch_id       VARCHAR(64) NOT NULL,
    user_id        BIGINT NOT NULL,
    redeem_code_id BIGINT NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT redeembatchclaim_batch_id_user_id UNIQUE (batch_id, user_id)
);

CREATE INDEX IF NOT EXISTS redeembatchclaim_user_id
    ON redeem_batch_claims(user_id);

CREATE INDEX IF NOT EXISTS redeembatchclaim_redeem_code_id
    ON redeem_batch_claims(redeem_code_id);
