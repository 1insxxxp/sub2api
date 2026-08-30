# Gift Redeem Balance Design

## Goal

Allow super administrators and secondary administrators to generate balance redeem codes as gift credit. Gift credit remains real spendable balance and all usage stays visible in financial and operational reports, but the credited amount and the portion of usage funded by it do not count toward check-in or activity eligibility thresholds.

## Current Behavior

- A positive balance redeem code calls the normal balance update path.
- Every positive balance update increases `users.total_recharged`.
- Check-in recharge eligibility reads `users.total_recharged`.
- Check-in usage eligibility sums `usage_logs.actual_cost` for the user.
- Users have one balance field, so a usage row cannot currently identify whether its cost came from paid or gifted credit.

Adding only a flag to the redeem code would correctly exclude the redemption from cumulative recharge, but it would not support excluding later usage because paid and gifted funds become indistinguishable after redemption.

## Selected Design

Keep `users.balance` as the total spendable balance and add `users.gift_balance` as the portion of that total that is exempt from eligibility thresholds. Gift balance is consumed before ordinary balance. Add `users.frozen_gift_balance` alongside the existing frozen balance so asynchronous image holds retain the same attribution while funds are reserved.

Add an immutable `threshold_exempt` flag to balance redeem codes. The flag defaults to false so existing and newly generated ordinary codes retain current behavior. The option is available only when generating balance redeem codes.

Add `usage_logs.threshold_exempt_cost` to record the part of each balance-billed request funded by gift balance. `actual_cost` remains unchanged and continues to power financial, usage, ranking, and administrator reports.

## Administrative UI

Both super administrator and secondary administrator generation forms show a toggle only for balance redeem codes:

> Gift credit (excluded from activity thresholds)

Help text explains that redemption increases spendable balance but not cumulative recharge, and that usage funded by this credit is excluded from check-in and activity spend thresholds while actual billing and usage records remain visible.

The toggle defaults to off. Generated-code lists display a compact gift-credit badge. The flag cannot be changed after generation, including while a code is unused, so audit meaning remains stable.

## Redemption Flow

Ordinary balance code redemption remains unchanged: increase `balance` and `total_recharged` by the code value.

Gift balance code redemption runs one atomic update:

- Increase `balance` by the code value.
- Increase `gift_balance` by the code value.
- Do not change `total_recharged`.
- Mark the code used in the same transaction.

Negative-value and non-balance codes cannot be marked as gift credit. The service validates this even if a client bypasses the UI.

Secondary-administrator balance transfers must preserve source integrity. All generated transfer codes, whether ordinary or marked as gift credit for the recipient, can use only the creator's ordinary balance (`balance - gift_balance`). Gift balance cannot be transferred into another redeem code. Deleting an unused transfer code therefore restores ordinary balance. This prevents exempt funds from being converted into ordinary credit on another account without introducing source-allocation metadata for refundable codes.

## Billing Flow

For a balance-billed request with cost `C`, atomically lock/update the user balance and calculate:

```text
gift_used = min(max(gift_balance, 0), C)
eligible_cost = C - gift_used
balance = balance - C
gift_balance = gift_balance - gift_used
```

The billing result returns `gift_used` to the gateway. Before the usage log is written, the gateway assigns it to `threshold_exempt_cost`. A request may therefore be split between gifted and ordinary balance.

Example: with USD 10 gift balance and USD 20 ordinary balance, a USD 12 request records `actual_cost = 12` and `threshold_exempt_cost = 10`. It contributes USD 2 to eligibility usage.

Subscription-billed requests do not consume gift balance. Other balance settlement paths, including batch image holds/captures and legacy/degraded billing, must preserve the same gift-first attribution rule or explicitly record a zero exempt amount only when no balance was deducted.

Batch image reservation moves the gift-funded portion from `gift_balance` to `frozen_gift_balance`. Capture consumes frozen gift first and records that amount as exempt usage; release returns the remaining frozen gift portion to available gift balance. The frozen pools are allocated globally in settlement order, matching the system's existing aggregate frozen-balance model.

## Eligibility Calculations

Recharge eligibility continues to read `users.total_recharged`; gift redemption never increments it.

Usage eligibility sums:

```text
GREATEST(actual_cost - threshold_exempt_cost, 0)
```

This expression applies only to check-in and activity threshold calculations. Existing operational and financial queries continue to sum full `actual_cost`.

Any administrator operation that rebuilds `total_recharged` from redeem history must exclude `threshold_exempt = true` codes. Paid-order calculations remain authoritative and unchanged.

## Compatibility And Migration

- `users.gift_balance` defaults to zero.
- `users.frozen_gift_balance` defaults to zero.
- `redeem_codes.threshold_exempt` defaults to false.
- `usage_logs.threshold_exempt_cost` defaults to zero.
- Existing codes and historical usage remain fully eligible and are not reclassified.
- The displayed user balance remains `users.balance`; no user-facing balance total changes.
- API responses may expose gift balance and exempt cost only on administrator/audit surfaces that need them.

## Consistency And Error Handling

- Redemption updates the code, total balance, gift balance, and recharge total in one transaction.
- Billing allocates gift credit in the same transaction as the total balance deduction to avoid concurrent requests spending the same gift credit.
- Exempt cost is quantized using the existing billing monetary scale.
- Database constraints or service validation prevent negative gift balance and prevent gift balance from exceeding the positive portion of total balance after normal operations.
- Refund and compensation paths must not manufacture gift balance. A future refund policy can add explicit source-aware behavior; it is outside this change.

## Testing

Backend tests cover:

- Ordinary redeem codes still increase balance and cumulative recharge.
- Gift redeem codes increase total and gift balances without increasing cumulative recharge.
- Mixed gift/ordinary deductions allocate gift first and record the split.
- Concurrent deductions cannot over-consume gift balance.
- Check-in cumulative usage excludes only `threshold_exempt_cost`.
- Existing rows with zero/default fields preserve current eligibility.
- Invalid gift flags on subscription, concurrency, or negative-value codes are rejected.
- Admin cumulative recharge rebuild excludes gift codes.
- Legacy and batch image balance paths follow the same attribution rules.

Frontend tests cover conditional toggle visibility, default-off behavior, request serialization, generated-code badges, responsive layout, and Chinese/English labels.

## Rollout

Deploy schema additions before enabling the UI. Defaults make the migration backward compatible. After deployment, verify one ordinary code and one gift code end to end, including the resulting balances, usage-log split, and check-in eligibility values.
