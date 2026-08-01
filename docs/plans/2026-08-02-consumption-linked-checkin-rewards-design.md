# Consumption-Linked Check-in Rewards Design

## Goal

Shift the daily check-in budget from broad random grants toward users who generated real usage on the previous Beijing calendar day. Keep a small random reward so check-in remains useful for users with no prior-day usage, while making the largest rewards visibly consumption-driven.

The production-data baseline used for this design covers the latest 30 days:

- 3,569 check-ins in the simulation sample
- 57.7% of check-ins had positive usage on the preceding day
- median preceding-day usage among those users: $14.37
- 75th percentile preceding-day usage: $41.52
- current random reward expected value: about $1.49 per check-in

## Approved Initial Rules

### Random base reward

| Amount | Probability |
| ---: | ---: |
| $0.30 | 40% |
| $0.50 | 30% |
| $0.80 | 20% |
| $1.50 | 8% |
| $3.00 | 2% |

The expected random base reward is $0.61. This reward is available even when preceding-day usage is zero.

### Usage rebate

- Preceding-day usage rebate rate: 8%
- Usage rebate cap: $8.00 per check-in
- Total check-in reward cap: $10.00
- Usage accounting timezone: `Asia/Shanghai`
- Usage base: the sum of successful usage-log `actual_cost` values from 00:00:00 through 23:59:59.999... on the calendar day before the check-in date
- Negative or absent totals are treated as zero
- Money is rounded to cents using the existing check-in money rounding convention

### Streak reward

Replace fixed streak grants with a percentage bonus based on the calculated usage rebate:

| Streak day | Rebate bonus |
| ---: | ---: |
| 7 | 10% |
| 15 | 15% |
| 30 | 20% |

The streak bonus is zero when the usage rebate is zero. It remains subject to the $10 total reward cap. This keeps the streak incentive aligned with consumption rather than reintroducing a large fixed subsidy.

### Reward calculation order

1. Roll the configured random base reward.
2. Sum preceding-day `actual_cost` and calculate `min(usage * 8%, $8.00)`.
3. If the new streak day is a configured milestone, calculate the configured percentage of the usage rebate.
4. Apply the $10 total cap.

Component amounts recorded for display must add up exactly to the credited amount. Preserve the random base first, then the usage rebate, then the streak bonus when the total cap truncates the reward. Also retain the raw preceding-day usage amount so support staff can reproduce the calculation.

## Configuration

Extend the check-in reward configuration with:

- `usage_rebate_enabled`
- `usage_rebate_rate_percent`
- `usage_rebate_cap`
- `total_reward_cap`
- streak rules expressed as `bonus_rate_percent`

All values are admin-configurable. Validation requires non-negative finite values, rebate percentages within 0-100, positive caps when the feature is enabled, unique streak days, and a total reward cap no lower than the smallest configured random reward.

Existing installations without the new fields must continue using random rewards and their existing fixed streak bonuses until an administrator saves the new mode. Deployment must not silently change production payouts.

The admin preview should show:

- random reward minimum, maximum, and expected value
- example rewards for preceding-day usage of $0, $10, $20, $50, and $100
- the configured usage rebate and total caps

## Persistence And Auditability

Add immutable calculation fields to each `user_checkins` record:

- `previous_day_usage_amount`
- `usage_rebate_amount`
- `reward_cap_adjustment` or an equivalent explicit value

Keep the existing base, streak, total, balance-before, and balance-after fields. Historical rows default the new monetary fields to zero.

The existing one-check-in-per-user-per-day constraint remains the idempotency boundary. A repeated submission returns the stored calculation and never recomputes the previous day's usage or credits the balance again.

## Service Flow

The check-in service continues to validate the global switch, cumulative usage/recharge eligibility, blacklist state, and existing daily record before opening its transaction.

Inside the existing check-in transaction it will:

1. Recheck the blacklist and daily uniqueness conditions.
2. Determine the preceding Beijing calendar-day UTC boundaries.
3. Sum `usage_logs.actual_cost` for the user within those boundaries.
4. Calculate and cap each reward component.
5. Atomically credit the user balance.
6. Store the calculation inputs, component outputs, and resulting balances.
7. Create the existing check-in reward balance-history record.

The usage query must be supported by an index beginning with `user_id` and `created_at`. Add or adjust a composite index only if the current schema does not already provide one.

## User Experience

After check-in, show an explicit breakdown instead of only the total:

```text
Random reward       $0.80
Yesterday's usage  $50.00
Usage rebate        $4.00
Streak bonus        $0.00
Credited today      $4.80
```

Before check-in, show yesterday's usage and the estimated rebate derived from it. The random portion remains presented as unknown until submission. Users with zero prior-day usage can still check in, maintain their streak, and receive the small random reward.

Recent check-in records should include the total and expose the same breakdown on demand. Mobile layouts must wrap monetary labels without truncating the amount.

## Admin Experience And Reporting

The check-in settings page adds a reward-mode section for the usage rebate controls and replaces fixed streak amount inputs with percentage inputs when usage-linked mode is enabled.

Admin records and aggregate statistics should separately expose:

- random base rewards
- usage rebates
- streak bonuses
- cap reductions
- preceding-day usage attributed to checked-in users

This allows operators to monitor rebate cost as a percentage of attributed usage and adjust the rate without guessing.

## Error Handling

- If the usage aggregation query fails, fail the check-in without crediting or creating a record. Do not silently fall back to zero usage.
- If configuration is invalid, reject the admin update and continue serving the last valid configuration.
- Concurrent submissions rely on the existing unique daily record and transaction handling; the losing request returns the persisted result.
- Balance cache invalidation continues only after a successful commit.

## Verification

Backend tests must cover:

- Beijing calendar-day boundaries, including UTC conversion
- zero usage, normal usage, usage rebate cap, and total cap
- all approved random tiers and deterministic reward-roll injection
- streak percentage milestones with and without usage
- component rounding and exact sum-to-total behavior
- idempotent retries and concurrent duplicate submissions
- aggregation failure rollback
- legacy configuration compatibility
- historical check-in record compatibility

Frontend tests must cover:

- admin configuration validation and preview calculations
- user pre-check-in estimate and post-check-in breakdown
- zero-usage messaging
- capped reward messaging
- mobile-width wrapping and no clipped monetary values

Before production rollout, run the formula against a read-only 30-day snapshot and compare projected random, usage-rebate, streak, and total costs. Start with the approved 8% / $8 / $10 settings and review the first seven days of actual rebate-to-usage ratio.

## Projected Cost

Applied to the 30-day production sample:

- projected random base: $2,177.09
- projected usage rebate: $4,736.57
- projected combined reward before streak bonuses and total-cap reductions: $6,913.66
- projected average before streak bonuses: $1.94 per check-in

This is approximately 30% above the current random-only theoretical cost, but the incremental budget is directed to users with actual preceding-day consumption.
