# Gemini Pool Failover Limit Design

## Goal

Make Gemini requests retry a pool account at most two additional times and allow the Gemini failover loop to try up to ten different accounts.

## Scope

- Change the default pool-mode same-account retry budget from 3 to 2 retries.
- Change the default Gemini account-switch budget from 3 switches to 9 switches, which permits up to 10 distinct account selections.
- Keep explicit per-account `pool_mode_retry_count` values unchanged.
- Keep existing failover eligibility, request cancellation, streaming-write protection, cooldown, and model/error policy behavior unchanged.

## Rationale

The current failover state counts account switches, not distinct accounts. Therefore, ten accounts require nine switches after the first account. Reusing the existing bounded loop avoids introducing an unlimited traversal mode or changing selection semantics.

## Verification

- Backend tests cover the new default pool retry value and preserve explicit values.
- Configuration tests cover the Gemini default switch value of 9.
- Existing failover-loop tests continue to verify that the switch limit is enforced.
- Frontend account creation/edit defaults and hints display the new retry default of 2.
