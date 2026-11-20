SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE usage_pending_records
    ADD COLUMN IF NOT EXISTS pending_reason varchar(64) NOT NULL DEFAULT 'upstream_usage_missing',
    ADD COLUMN IF NOT EXISTS provider_evidence jsonb;

-- Delivered output may be the measured response's customer-facing projection.
ALTER TABLE usage_pending_records
    DROP CONSTRAINT IF EXISTS usage_pending_records_output_tokens_source_check,
    DROP CONSTRAINT IF EXISTS usage_pending_records_protected_input_check;
ALTER TABLE usage_pending_records
    ADD CONSTRAINT usage_pending_records_output_tokens_source_check CHECK (
        output_tokens_source IN ('upstream', 'unreported') OR
        (output_tokens_source = 'estimated' AND provider_evidence IS NOT NULL AND provider_evidence ? 'projected_output_tokens')
    ) NOT VALID;

-- A legacy writer retrying a protected identity must never copy raw counts
-- back into history. Existing rows have NULL evidence and keep their history.
ALTER TABLE usage_pending_records
    ADD CONSTRAINT usage_pending_records_protected_input_check CHECK (
        provider_evidence IS NULL OR
        (jsonb_typeof(provider_evidence) = 'object' AND input_tokens_source = 'unreported' AND
         input_tokens = 0 AND cache_read_tokens = 0 AND cache_creation_tokens = 0 AND
         cache_creation_5m_tokens = 0 AND cache_creation_1h_tokens = 0)
    ) NOT VALID;

COMMENT ON COLUMN usage_pending_records.provider_evidence IS
    'Private input-protection observations and optional provider price. Never a customer charge or quota debit.';
