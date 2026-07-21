# Admin Dashboard Load Reduction Design

## Goal

Reduce PostgreSQL and CPU spikes caused by admin dashboard aggregation queries without changing customer API behavior or removing administrator access to historical data.

## Scope

- Disable automatic refresh for the operations dashboard by default and globally.
- Change the regular admin dashboard's initial date range from the last seven days to the current day.
- Keep manual refresh and custom date-range selection available.
- Do not change gateway traffic, billing, usage recording, database retention, or customer limits.

## Behavior

The operations dashboard loads once when opened and refreshes only after an administrator explicitly requests it. Existing stored settings that enabled automatic refresh must not silently resume high-frequency polling.

The regular admin dashboard initially requests data for the current local day. Administrators can still select longer ranges when needed. Date changes continue to trigger the existing snapshot and user-trend loading flow.

## Verification

- Add focused frontend tests for the default dashboard date range.
- Add coverage that confirms operations auto-refresh remains disabled when settings are loaded.
- Run the relevant Vitest tests, frontend type checking, and production build.
- After deployment, compare PostgreSQL CPU, active query duration, and admin endpoint latency before and after opening the dashboards.

## Rollback

Revert the frontend behavior commit and redeploy the previous image. No schema or data migration is involved.
