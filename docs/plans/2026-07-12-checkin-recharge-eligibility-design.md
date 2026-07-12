# Check-in Recharge Eligibility Design

## Goal

Allow a user to check in when either their cumulative usage reaches the configured usage threshold or their cumulative credited balance reaches a new recharge threshold.

## Eligibility Rules

- The usage and recharge criteria use OR semantics.
- A positive usage threshold enables the usage criterion.
- A positive recharge threshold enables the recharge criterion.
- A zero value disables that individual criterion.
- When both thresholds are zero, check-in has no spend requirement and remains available to all otherwise eligible users.
- Recharge progress uses the existing `users.total_recharged` value. This includes positive balance credits such as payment recharges, administrator balance additions, and redemption credits when those flows use the standard balance update path.

## Backend Changes

Add a `checkin_min_total_recharge_usd` setting and expose it through the existing administrator check-in configuration API. Extend user check-in status with the configured recharge threshold and the user's cumulative recharge amount.

Replace the usage-only eligibility helper with a helper that evaluates enabled usage and recharge criteria. Use the same helper in status calculation, the pre-transaction check, and the transactional recheck so concurrent balance changes cannot produce inconsistent results.

Return a combined ineligibility reason when neither enabled criterion has been reached. Preserve unrestricted behavior when both settings are zero.

## Frontend Changes

Add a second USD input to the administrator check-in settings for the cumulative recharge threshold. The user check-in panel will show progress for every enabled criterion and state that satisfying either configured criterion enables check-in.

Update Chinese and English translations for the administrator label, progress text, and combined ineligibility message. Keep the existing responsive header behavior and avoid changing unrelated navigation.

## Compatibility

The new setting defaults to zero, so existing installations continue to use their current usage threshold without migration work. Existing API clients can ignore the additive response fields.

## Testing And Verification

- Backend unit tests cover usage-only, recharge-only, either criterion met, neither met, and both thresholds disabled.
- Handler/config tests verify the new setting is read and written.
- Frontend tests verify administrator editing and user progress rendering.
- Run backend tests, frontend tests, type checking, and production builds.
- Verify Vite HMR reflects frontend changes.
- Rebuild and restart only `sub2api-dev`, then verify backend version, health, and the affected check-in API response. PostgreSQL and Redis remain running.
