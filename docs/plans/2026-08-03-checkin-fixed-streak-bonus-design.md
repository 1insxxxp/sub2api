# Fixed Check-in Streak Bonus Design

## Goal

Keep usage rebates and streak bonuses independent. Enabling the previous-day usage rebate must never change a streak reward from a fixed balance credit into a rebate percentage multiplier.

## Reward Model

The daily reward has three independent parts:

1. A random base reward selected from the configured tiers.
2. An optional previous-day usage rebate, calculated from usage and constrained by its configured caps.
3. A fixed streak bonus from the matching `streak_rules[].bonus_amount` milestone.

The base reward and usage rebate remain subject to the configured usage-reward cap. The fixed streak bonus is added afterward so a configured milestone always credits its full fixed amount, preserving the original streak-reward semantics.

## Configuration and Compatibility

- The admin check-in page always edits streak rewards as USD amounts.
- Toggling usage rebates does not rewrite streak rules.
- Config saves always preserve and submit `bonus_amount`.
- `bonus_rate_percent` remains in API types temporarily for backward-compatible reads, but the UI no longer writes it and reward calculation no longer consumes it.
- Existing production fixed milestones remain unchanged.
- No database schema migration is required.
- A legacy configuration that contains only percentage streak rules is rejected rather than silently interpreting a percentage as a dollar amount.

## Backend Behavior

Configuration normalization always validates every streak rule through the existing money-scaling path, independent of `usage_rebate_enabled`. Reward calculation computes the capped base/rebate portion first, then adds the matching fixed streak amount. Audit fields continue to record the fixed amount in `bonus_reward_amount`, and `total_reward_amount` includes all three parts.

## Frontend Behavior

The admin page always labels the streak value as an extra USD reward and always binds it to `bonus_amount`. Adding, cloning, validating, and saving rules no longer branch on the usage-rebate switch. User-facing next-milestone text always formats the fixed amount as currency.

## Error Handling

Invalid or missing fixed amounts fail existing streak-rule validation. The system does not auto-convert historical percentage values because such conversion would assign an arbitrary monetary meaning and could create unintended payouts.

## Testing

- Backend configuration tests prove fixed amounts remain valid when usage rebates are enabled.
- Backend calculation tests prove the fixed milestone is added in full alongside a capped usage rebate.
- Admin component tests prove enabling usage rebates does not show or submit percentage streak fields.
- User header tests prove the next streak milestone is displayed as a fixed currency reward.
- Existing check-in, type-check, lint, and production-build gates remain required.

## Deployment Scope

This change is implemented and tested locally first. It is not pushed or deployed until explicitly requested after local acceptance.
