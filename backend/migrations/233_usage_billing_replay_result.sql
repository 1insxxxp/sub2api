ALTER TABLE usage_billing_dedup
    ADD COLUMN IF NOT EXISTS threshold_exempt_cost NUMERIC(20,8);

ALTER TABLE usage_billing_dedup_archive
    ADD COLUMN IF NOT EXISTS threshold_exempt_cost NUMERIC(20,8);
