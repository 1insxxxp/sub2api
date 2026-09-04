# Check-in Whitelist Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a check-in whitelist so matching users can check in without daily usage, cumulative usage, or cumulative recharge thresholds.

**Architecture:** Store normalized email/username entries as a JSON array in the existing system settings repository. The backend `CheckinService` loads the list and applies the exemption in the shared policy calculation used by both status and check-in flows. The existing admin check-in page edits the list, while the user status response exposes whether the current user is exempt so the UI can explain why the thresholds are bypassed.

**Tech Stack:** Go, Gin, existing settings repository, Vue 3, TypeScript, Vitest, Go tests.

---

## Scope and behavior

- Match either the user's email or username.
- Normalize values by trimming surrounding whitespace and comparing case-insensitively.
- Ignore blank entries and de-duplicate values before persistence.
- A matched user bypasses `min_daily_usage_count`, `min_total_usage_usd`, and `min_total_recharge_usd`.
- The global check-in switch, active check-in blacklist, one-check-in-per-Beijing-day rule, reward rules, and account authorization remain enforced.
- Existing configurations without the new setting behave exactly as before.

## Data flow

1. Admin check-in settings API reads and writes `checkin_whitelist` with the other check-in settings.
2. `CheckinService` loads the list for the current policy evaluation and resolves the current user from the existing user repository/entity client.
3. `currentCheckinPolicyState` marks a matching user exempt before applying usage-count and spend/recharge checks.
4. `GetStatus` returns `whitelist_exempt` and the effective threshold values for the frontend.
5. The admin page provides a multiline editor with normalization feedback and saves it together with the existing check-in configuration.

## Validation and error handling

- Reject malformed JSON for the whitelist setting by falling back to an empty list and logging a warning, matching existing tolerant settings behavior.
- Reject non-string or excessively long entries in update requests with a 400 response; keep a bounded list to protect settings size.
- Preserve blacklist precedence: a blacklisted user remains ineligible even when whitelisted.
- Use the same policy path for status and actual check-in to prevent UI/API disagreement.

## Testing

- Service tests cover email match, username match, case/whitespace normalization, non-match, empty/default configuration, blacklist precedence, and bypass of all three thresholds.
- Handler/API tests cover round-tripping the whitelist field.
- Frontend tests cover loading, editing, saving, and displaying whitelist exemption state.
- Run targeted Go and Vitest suites, then the relevant full package checks before integration.
