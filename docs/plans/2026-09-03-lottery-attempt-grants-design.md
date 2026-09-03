# Lottery Attempt Grants Design

## Goal

Allow administrators to add promotional lottery attempts to selected users or to every non-deleted user, while preserving a durable audit trail for each grant.

## Assumptions

- An administrator grant is a reward-wallet attempt and therefore is not constrained by the activity's daily or total free-attempt limit.
- “All users” means all users that are not soft-deleted, including any role; deleted accounts are excluded because they cannot use the attempts.
- A grant request contains a positive integer amount and an optional note. Selected-user requests must contain at least one valid, non-deleted user ID.

## Architecture

The existing `lottery_attempt_wallets` balance remains the source consumed after activity attempts. A new `lottery_attempt_grants` table stores one row per user grant and supplies a unique database ID for the corresponding `admin_grant` ledger row. This keeps the existing ledger uniqueness rule intact and makes every wallet increase auditable without relying on timestamps or synthetic IDs.

`LotteryService.GrantLotteryAttempts` validates the request and delegates to an optional administrator capability on the lottery repository. The production repository resolves the target users, opens one transaction, inserts a grant row, upserts the wallet, increments it, and writes an `admin_grant` ledger row for each target. Any failure rolls the complete batch back.

The admin API exposes `POST /admin/lottery/attempts/grant`. The existing lottery admin page gets a grant section with a target toggle, debounced user search/multi-select for selected mode, amount/note inputs, and a confirmation result showing how many users were affected. The page keeps the current activity, prize, inventory, and draw-record flows unchanged.

## Validation and errors

- Reject zero/negative amounts, amounts above the wallet-safe cap, missing target selection, mixed `all` plus explicit IDs, invalid IDs, and notes over the documented length.
- Reject deleted or missing explicit users rather than silently granting fewer users.
- Return the affected-user count and total attempts granted in the success payload.
- Keep route authorization in the existing admin route group; the handler records the authenticated administrator ID in each grant row.

## Testing

- Service tests cover request validation and delegation/result mapping.
- Repository/integration-oriented tests cover wallet balance, grant rows, ledger source type, rollback, selected users, and all-user exclusion of soft-deleted users where the repository test harness is available.
- Handler tests cover authenticated grant requests, pagination-independent response shape, and bad payloads.
- Frontend API tests assert the request path/payload; `LotteryView` tests cover selected-user search, all-user toggle, successful submission, and affected-count feedback.
- Run focused Go/Vitest tests, frontend typecheck/build, lint on changed files, and `git diff --check`.
