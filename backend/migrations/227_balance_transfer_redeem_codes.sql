-- Add balance-transfer redeem code permissions and audit metadata.

ALTER TABLE users
  ADD COLUMN IF NOT EXISTS balance_redeem_code_enabled BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE redeem_codes
  ADD COLUMN IF NOT EXISTS created_by BIGINT NULL,
  ADD COLUMN IF NOT EXISTS source VARCHAR(32) NOT NULL DEFAULT 'admin';

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1
    FROM pg_constraint
    WHERE conname = 'redeem_codes_created_by_fkey'
  ) THEN
    ALTER TABLE redeem_codes
      ADD CONSTRAINT redeem_codes_created_by_fkey
      FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE SET NULL NOT VALID;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_redeem_codes_created_by ON redeem_codes(created_by);
CREATE INDEX IF NOT EXISTS idx_redeem_codes_source ON redeem_codes(source);
