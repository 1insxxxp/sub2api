# Secondary Admin Commission Design

## Goal

Build a first version of a secondary-admin commission system in the admin workbench. Super admins can assign groups to secondary admins, and secondary admins can view authorized groups' daily balance-consumption flow in a calendar, from the authorization date through today.

## Confirmed Decisions

- Commission uses one global rate for all secondary admins and all assigned groups.
- The authorization start day counts from that day's 00:00 in the system group-usage timezone.
- First version only displays flow and estimated commission. It does not track settlement, payout, or paid/unpaid status.
- Clicking a calendar day opens day details.
- Day details show group-level summary first. Clicking a group expands request-level usage details for that day.
- Daily balance consumption uses `usage_logs.actual_cost`, matching the amount actually deducted from user balance.

## Current System Context

- Roles already include `admin`, `sub_admin`, and `user`.
- The admin workbench route already exists and is accessible to `sub_admin`.
- `usage_logs` already records `group_id`, token counts, model, user/API key/account relationships, request timestamps, and cost fields.
- `usage_group_daily_rollups` already stores daily group-level `actual_cost` by `bucket_date` and `group_id`.
- Existing full-admin usage endpoints are too broad for this feature because secondary admins must never query arbitrary group data by changing request parameters.

## Recommended Approach

Add a dedicated secondary-admin commission permission model and dedicated scoped APIs.

This keeps financial visibility separate from normal group routing or API-key permissions. A secondary admin seeing group consumption must not accidentally gain call permissions, and call permissions must not automatically grant financial visibility.

## Data Model

### Global Setting

Store a global commission rate in the existing system settings mechanism if available.

Suggested key:

- `sub_admin_commission_rate`

Suggested semantics:

- Decimal rate, for example `0.05` means 5%.
- Default `0`.
- Only super admins can update it.

### Authorization Table

Create a new table for secondary-admin group visibility:

- `id`
- `sub_admin_user_id`
- `group_id`
- `granted_date`
- `enabled`
- `created_by`
- `created_at`
- `updated_at`

Rules:

- Only users with role `sub_admin` can be assigned.
- Only active groups can be assigned.
- One active row per `sub_admin_user_id + group_id`.
- `granted_date` is the local natural date used for calendar visibility.
- If disabled, data from that group should no longer be visible in the workbench.

## API Design

### Super Admin APIs

Add super-admin-only endpoints to manage permissions and the global rate:

- `GET /api/v1/admin/sub-admin-commissions/settings`
- `PUT /api/v1/admin/sub-admin-commissions/settings`
- `GET /api/v1/admin/sub-admin-commissions/grants`
- `PUT /api/v1/admin/sub-admin-commissions/grants/:sub_admin_id`

The grant update endpoint receives the selected group IDs for a secondary admin and writes authorization rows with `granted_date` set to today's local date for newly added groups.

### Secondary Admin Workbench APIs

Add scoped endpoints for the current user:

- `GET /api/v1/admin/workbench/commission/grants`
- `GET /api/v1/admin/workbench/commission/calendar?month=YYYY-MM`
- `GET /api/v1/admin/workbench/commission/days/:date/groups`
- `GET /api/v1/admin/workbench/commission/days/:date/groups/:group_id/logs`

Security requirements:

- The backend must derive the secondary admin from the authenticated user.
- The backend must intersect every query with that user's enabled grant rows.
- A `group_id` in the URL is valid only if the current user has an enabled grant for it and the requested date is on or after `granted_date`.
- Request-level logs should expose only fields needed for reconciliation, not sensitive upstream credentials or internal secrets.

## Aggregation Rules

Calendar month response:

- Return one item per visible day from `granted_date` through today.
- Days before the earliest grant are omitted or returned as disabled.
- Future days are disabled.
- Historical closed days should prefer `usage_group_daily_rollups`.
- Today should use live `usage_logs` aggregation so the current day updates before rollups close.
- If a day is not covered by rollups, fall back to `usage_logs`.

Day group summary:

- Group by authorized `group_id`.
- Include `requests`, token totals, `actual_cost`, and `commission_amount`.
- `commission_amount = actual_cost * global_rate`.

Request-level detail:

- Filter by exact local day window and authorized group.
- Include time, user identifier if available, API key label if available, model, token totals, `actual_cost`, and status.
- Add pagination to avoid returning large days in one response.

## Admin Workbench UI

### Super Admin View

Add a commission management section to the admin workbench:

- Global commission rate input.
- Secondary-admin selector.
- Group multi-select or searchable transfer list.
- Save button.
- Display each grant's start date.

### Secondary Admin View

Add a commission calendar section:

- Month switcher.
- Group filter with "all assigned groups" as default.
- Calendar cells showing daily consumption and estimated commission.
- Month summary showing total consumption and estimated commission.
- Click a day to open a drawer or panel.

Day detail panel:

- Group summary rows by default.
- Expand a row to show request-level logs.
- Logs are paginated.

## Error Handling

- If no groups are assigned, show an empty state explaining that no commission groups are available.
- If the global rate is `0`, still show consumption and show commission as `0`.
- If a requested group/date is outside authorization scope, return `403`.
- If a date is invalid or future, return `400`.
- If rollup data is unavailable, fall back to live logs instead of failing the whole calendar.

## Alternatives Considered

### Reuse Existing Admin Usage APIs

Fastest to implement, but unsafe for secondary admins because the API surface is broad and easy to misuse with arbitrary filters.

### Dedicated Permission Table and Scoped APIs

Recommended. It keeps permissions explicit and gives the UI exactly the financial visibility it needs.

### Full Settlement System

Too large for the first version. Settlement state, payout records, and month-close workflows should be added after the read-only flow is proven.

## Testing Plan

Backend tests:

- Super admin can assign and remove groups for a secondary admin.
- Secondary admin can only read assigned groups.
- Authorization start date includes the whole granted day.
- Calendar aggregation uses `actual_cost`.
- Day details reject unassigned groups.
- Request log detail is paginated.

Frontend tests:

- Super admin can edit global rate and group grants.
- Secondary admin sees calendar only for assigned groups.
- Calendar day click opens group summary.
- Group row expansion loads request logs.
- Empty state appears when no groups are assigned.

## Rollout Notes

- Ship with global rate defaulting to `0` so enabling visibility does not imply accidental payout amounts.
- Keep the feature inside the existing admin workbench.
- Do not deploy until local backend, frontend, and targeted tests pass.
