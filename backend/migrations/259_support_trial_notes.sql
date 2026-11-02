SET LOCAL lock_timeout = '3s';
SET LOCAL statement_timeout = '30s';

-- Repair only the default note on a proven, non-legacy support-page grant.
-- Wallets, amounts, operators and lifetime grant claims remain unchanged.
UPDATE redeem_codes AS code
SET notes = '测试额度-15刀'
WHERE code.source = 'support_trial'
  AND code.type = 'balance'
  AND code.status = 'used'
  AND code.value = 15
  AND code.notes = 'Support trial credit'
  AND EXISTS (
      SELECT 1
      FROM support_trial_grants AS claim
      WHERE claim.redeem_code_id = code.id
        AND claim.user_id = code.used_by
        AND claim.operator_id = code.created_by
        AND claim.amount = 15
        AND claim.legacy = FALSE
  );
