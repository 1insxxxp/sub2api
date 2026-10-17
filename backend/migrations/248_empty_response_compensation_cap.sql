-- A NULL amount preserves the original full-refund policy for existing claims.
-- New claims freeze the per-request capped refund without changing the original charge.
ALTER TABLE empty_response_claims
    ADD COLUMN IF NOT EXISTS compensation_amount DECIMAL(20,10);
