-- A source is recorded only when the adapter captured field-level evidence.
-- Do not infer provenance or rewrite costs for historical numeric zero values.
-- The migration runner executes this file in one transaction. Fail promptly
-- on a busy production table instead of queuing billing writes behind DDL.
SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

ALTER TABLE usage_response_outcomes
    ADD COLUMN IF NOT EXISTS input_tokens_source VARCHAR(20) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS output_tokens_source VARCHAR(20) NOT NULL DEFAULT 'unknown';

-- Existing rows acquire the valid constant default without a table rewrite.
-- NOT VALID still checks every new write, while avoiding a historical scan
-- under the schema-change lock. Scope guards to this table for safe replay.
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'usage_response_outcomes'::regclass
          AND conname = 'usage_response_outcomes_input_tokens_source_check'
    ) THEN
        ALTER TABLE usage_response_outcomes
            ADD CONSTRAINT usage_response_outcomes_input_tokens_source_check
            CHECK (input_tokens_source IN ('unknown', 'upstream', 'unreported', 'estimated')) NOT VALID;
    END IF;
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conrelid = 'usage_response_outcomes'::regclass
          AND conname = 'usage_response_outcomes_output_tokens_source_check'
    ) THEN
        ALTER TABLE usage_response_outcomes
            ADD CONSTRAINT usage_response_outcomes_output_tokens_source_check
            CHECK (output_tokens_source IN ('unknown', 'upstream', 'unreported', 'estimated')) NOT VALID;
    END IF;
END;
$$;

COMMENT ON COLUMN usage_response_outcomes.input_tokens_source IS
    'Field-level input count provenance; unreported differs from an upstream-reported zero';
COMMENT ON COLUMN usage_response_outcomes.output_tokens_source IS
    'Provenance of the customer output count; estimated means locally counted received output';
COMMENT ON COLUMN usage_response_outcomes.collector_version IS
    'Response evidence collector version; 0 denotes token-source metadata without response evidence';
