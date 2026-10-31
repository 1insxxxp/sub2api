# Low-balance admission correction

Approved by the user on 2026-10-31, including commit, push and deployment after verification.

## Behavior

Token-billed requests no longer use historical request costs as a hard admission floor. Keep the configured minimum reserve, the positive-wallet check, per-user atomic in-flight reservations, key quotas, subscriptions and fixed-charge/batch holds. Do not change request parameters or output limits. Actual usage remains settled by the existing billing path; one request can still overdraw the wallet.

Before rejecting a wallet based on an auth snapshot or balance cache, read the primary database with a bounded timeout and coalesce concurrent reads for the same user. Recovery must preserve key/user/group/IP checks. Use fresh user/key copies in auth, never mutate shared snapshots. Do not write an authoritative balance into the deduction cache: a concurrent deduction can otherwise be counted twice. Invalidate the stale entry instead. Do not retain a verified balance in a WebSocket connection context.

An in-flight reservation rejection gets one authoritative balance check and at most one more atomic reservation attempt. Existing reservations remain intact. Return a transient, distinct error if other requests still occupy the balance; DB failures are service-unavailable errors. Unknown pricing in opt-in fail-closed mode also means service unavailable, not wallet exhaustion.

## Verification and release

Regressions cover expensive historical usage, zero/negative/threshold balances, stale-zero recharge recovery, unavailable DB, auth restrictions and in-flight overlap/settlement. Run all repository release gates, review the complete diff, preserve upstream version 0.2.19, push the exact commit, await CI and build off production. Read actual production routes, preserve runtime configuration and security patches, deploy parallel containers, verify both entrances and rollback resources before draining old instances. Production tests use isolated resources, never customer wallets or credentials.
