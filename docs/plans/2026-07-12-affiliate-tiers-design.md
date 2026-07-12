# Affiliate Promotion Tiers Design

## Goal

Turn the current flat affiliate rebate into a cumulative promotion-level system that rewards sustained acquisition without monthly resets. Keep rebates as site balance, preserve administrator-specific rates, and recognize qualifying historical invitees at rollout.

## Product Rules

The default promotion levels are:

| Level | Cumulative qualified invitees | Rebate rate |
| --- | ---: | ---: |
| Standard | 0-2 | 8% |
| Bronze | 3-9 | 10% |
| Silver | 10-29 | 12% |
| Gold | 30 or more | 15% |

Levels are cumulative and do not reset monthly. Normal inactivity never lowers a level. An invalidated payment, refund, chargeback, or confirmed abuse may remove a qualification and cause the level to be recalculated because the original qualification no longer exists.

The qualification threshold, level thresholds, and rates are administrator-configurable settings. The values above are the rollout defaults. Validation requires non-negative qualification amounts, strictly increasing invitee thresholds, and rates from 0 through 100.

## Qualified Invitees

An invitee becomes qualified when successful real-money payment orders reach a cumulative USD-equivalent amount of $50.

- Multiple successful orders accumulate toward the threshold; the first order does not need to be $50.
- Each invitee contributes at most one qualified invitee to the inviter.
- Site balance grants, redeem codes, administrator balance adjustments, check-in rewards, affiliate transfers, and other non-payment credits do not qualify.
- Refunded, reversed, chargeback, cancelled, or otherwise invalid payment amounts do not qualify.
- Historical successful payment orders count at rollout.
- The qualification belongs to the inviter relationship recorded at registration and cannot be reassigned through later code changes.

Qualification state must be durable and idempotent. Payment callback retries must not increment an inviter more than once. Reconciliation must be able to recompute qualification from authoritative payment orders when refunds or historical inconsistencies are found.

## Rebate Resolution

The effective rebate rate uses this priority:

1. An administrator-specific rebate-rate override for the inviter.
2. The inviter's automatically calculated promotion-level rate.
3. The configured base rate, defaulting to 8%.

Clearing an administrator-specific override immediately returns the inviter to the rate determined by their qualified-invitee count.

When an inviter reaches a new level, all subsequent eligible recharges from both existing and future invitees use the new rate. Previously settled orders are never recalculated and receive no retroactive difference.

Existing freeze periods, affiliate relationship duration limits, per-invitee rebate caps, and ledger idempotency continue to apply after the tier rate is resolved.

## Persistence And Historical Backfill

Store durable qualification metadata on the invitee's affiliate relationship, including the qualification timestamp. Maintain or derive the inviter's qualified count through repository operations that are transaction-safe under concurrent payment callbacks.

The rollout includes an idempotent historical backfill that:

1. Finds users with an inviter relationship.
2. Sums their authoritative successful real-money payment orders net of invalidated orders.
3. Marks invitees at or above the configured $50 threshold as qualified.
4. Recomputes inviter qualified counts and resulting levels.

The backfill changes future effective rates only. It does not create rebate ledger entries or modify historical rebate amounts. It must be restartable and safe to run more than once.

## Backend Behavior

The affiliate service resolves the inviter's automatic tier before each new rebate accrual. Payment fulfillment updates qualification after an eligible payment becomes authoritative, using the same idempotency guarantees already applied to affiliate order rebates.

Affiliate user and administrator responses add:

- Current automatic level and level rate.
- Effective rate after a possible administrator override.
- Qualified invitee count.
- Next level threshold and remaining invitee count.
- Per-invitee qualification status and cumulative qualifying payment amount.

The settings API exposes the qualification amount and tier definitions. Invalid or overlapping configurations are rejected without changing active settings.

Refund and chargeback reconciliation removes payment amounts from qualification totals. If the total falls below the threshold, the qualification is removed and the inviter's level is recalculated. This exception does not conflict with permanent level retention because invalid revenue cannot establish a permanent benefit.

## Frontend Behavior

The user affiliate page shows the current level, effective rebate rate, qualified invitee count, and progress to the next level. It also shows all four configured levels and marks each invitee as either qualified or in progress, with qualifying payment progress such as `$35 / $50`.

When a user has an administrator-specific rate, the page labels it as a special rate while still showing the automatic level and progress. This avoids making level progress appear lost.

The administrator affiliate views show automatic level, qualified count, automatic tier rate, special override, and effective rate separately. Affiliate settings allow editing the qualification amount, thresholds, and rates with clear validation. Existing custom-code and custom-rate workflows remain available.

All new layouts must work at 320px and wider without horizontal page overflow. Dense level tables may use a compact mobile layout, while invitee payment progress must wrap without obscuring existing fields.

## Compatibility And Rollout

- The global base rebate changes from 10% to 8% at rollout.
- Existing inviters receive levels from historical qualified invitees before the new rates are used in production.
- Existing administrator-specific rates remain unchanged and continue to override automatic levels.
- Existing affiliate balances, frozen balances, transfer records, invite links, and inviter relationships remain unchanged.
- No first-recharge bonus or automatic trial-credit task is included in this feature.

Deployment must run the schema migration and historical backfill before enabling tier-based rate resolution. If backfill fails, the application must retain the existing flat/custom rate behavior rather than silently treating all established promoters as Standard.

## Observability And Administration

Log qualification, disqualification, level changes, backfill progress, and invalid configuration attempts with user and order identifiers. Administrator records should make it possible to explain why a user has a given level and effective rate.

The backfill reports scanned invitees, newly qualified invitees, unchanged invitees, disqualified invitees, and failures. A reconciliation command or service operation should be reusable after payment corrections.

## Testing And Verification

- Service tests cover all level boundaries, cumulative payments, multiple orders, rate priority, upgrades applying only to future rebates, and custom-rate clearing.
- Payment tests cover callback retries, concurrent threshold crossing, refunds, invalid statuses, and non-payment credits being excluded.
- Repository and migration tests cover durable qualification, count consistency, historical backfill idempotency, and rollback behavior.
- Handler and API contract tests cover the added user/admin fields and settings validation.
- Frontend tests cover level progress, special-rate presentation, invitee qualification progress, settings validation, localization, and mobile layouts.
- Run backend unit and integration tests, frontend tests, type checking, and production build.
- Verify historical backfill against a database snapshot before production deployment and compare qualified counts with successful payment totals.
