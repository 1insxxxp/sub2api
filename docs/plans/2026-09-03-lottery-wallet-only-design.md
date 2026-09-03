# Lottery Wallet-Only Attempts Design

## Confirmed rule

Lottery attempts no longer come from an activity quota and never reset daily. The only sources are administrator grants and consecutive check-in rewards.

## Product behavior

- Remove the admin-facing activity attempt mode and attempt-limit controls.
- Keep the legacy activity fields in storage and API shapes for compatibility, but normalize/save them as zero-equivalent values and do not use them to authorize draws.
- Every user-facing and admin-facing balance view treats the persistent lottery attempt wallet as the source of truth.
- Each successful draw consumes exactly one wallet attempt and records `attempt_source=wallet`.
- Existing check-in and administrator grant flows continue crediting the same wallet and remain idempotent.

## Data flow

1. Check-in streak qualification credits the user's wallet.
2. An admin grant credits the selected users or all non-deleted users' wallets.
3. Public state reads wallet balance and returns it as the total/reward balance; activity balance is always zero.
4. Draw locks/debits one wallet attempt before committing the reward snapshot.

## Compatibility and error handling

- Do not drop database columns or rewrite historical draws.
- Existing activities with daily/total settings remain readable; their legacy values are ignored for new draws.
- A draw with no wallet balance returns the existing exhausted-attempt error.
- Existing idempotency keys still return the original draw without double-debiting.

## Testing scope

- Service tests cover wallet-only summaries, ignored activity limits, wallet debit/source recording, and exhausted balances.
- Admin view tests verify the legacy controls are absent while grants and balance tables remain.
- User view/API tests verify only wallet attempts are displayed and consumed.
