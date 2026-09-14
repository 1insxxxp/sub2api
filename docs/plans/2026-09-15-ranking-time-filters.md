# Ranking Time Filters Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a shared natural-day time filter for the user consumption and user recharge rankings, with presets and custom dates, while keeping both rankings synchronized.

**Architecture:** Keep `UsageView` as the single owner of `startDate` and `endDate`. Add a reusable toolbar below the detail tabs and above ranking content; preset actions update the parent range and the existing child props/watchers trigger API reloads. The existing backend ranking endpoints already accept explicit `start_date`, `end_date`, and `timezone`, so no schema or SQL migration is needed.

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Vitest, existing `DateRangePicker`, Go/Gin ranking handlers and repository tests.

---

### Task 1: Add a focused date-range utility test (RED)

**Files:**
- Create: `frontend/src/views/admin/__tests__/rankingTimeRange.spec.ts`
- Reference: `frontend/src/views/admin/UsageView.vue`

**Step 1: Write the failing test**

Define the expected pure helper contract for `getRankingRangePreset(preset, today)`:
- `today` returns the supplied local date for both bounds.
- `yesterday` returns the previous local date for both bounds.
- `7d` returns today minus six calendar days through today.
- `30d` returns today minus 29 calendar days through today.
- Month boundaries use calendar arithmetic rather than milliseconds.

**Step 2: Run the test to verify it fails**

Run: `pnpm --dir frontend exec vitest run src/views/admin/__tests__/rankingTimeRange.spec.ts`
Expected: FAIL because the helper does not exist yet.

**Step 3: Commit**

Do not commit until the implementation task makes the test pass; keep the test as the red-first change in the working tree.

### Task 2: Implement the shared preset helper and toolbar

**Files:**
- Modify: `frontend/src/views/admin/UsageView.vue`
- Modify: `frontend/src/i18n/locales/zh/admin/resources.ts`
- Modify: `frontend/src/i18n/locales/en/admin/resources.ts`

**Step 1: Implement the minimal helper**

Add a pure, exported-for-test helper or colocated testable function that parses `YYYY-MM-DD` into a local calendar date and uses `setDate`/`setMonth` safe calendar arithmetic. Keep preset keys stable: `today`, `yesterday`, `last7Days`, `last30Days`.

**Step 2: Add the toolbar markup**

Render it only for `ranking` and `recharge` tabs, immediately below the detail tab buttons. Include:
- Buttons for 今日/昨日/最近 7 天/最近 30 天.
- Existing `DateRangePicker` for custom dates.
- A compact text label showing the active inclusive range.
- `flex-wrap`, gap, and narrow-screen width classes so controls wrap on mobile.

**Step 3: Wire state changes**

Use the existing `onDateRangeChange` path so charts and both ranking children receive the same range. Add `applyRankingPreset` that updates `startDate`, `endDate`, `filters.start_date`, `filters.end_date`, sets granularity, and calls the existing filter refresh once. Change the initial default and `resetFilters` target to today, preserving route query overrides.

**Step 4: Add localized labels**

Add Chinese and English labels for the toolbar title, four presets, custom range, and displayed range. Avoid raw English fallback keys in either locale.

### Task 3: Verify frontend behavior (GREEN)

**Files:**
- Modify: `frontend/src/views/admin/__tests__/UsageView.spec.ts`
- Modify: `frontend/src/components/admin/usage/__tests__/UserTokenRanking.spec.ts`
- Modify or create: `frontend/src/components/admin/usage/__tests__/RechargeRanking.spec.ts`

**Step 1: Extend tests**

Cover:
- Clicking each preset sends the same `start_date`/`end_date` to both ranking API calls.
- Cross-month preset calculation is calendar-correct.
- Custom range updates both children and resets their page to 1.
- Existing user/source/sort filters remain intact.
- The toolbar has wrapping classes and does not require horizontal overflow.

**Step 2: Run targeted tests**

Run: `pnpm --dir frontend exec vitest run src/views/admin/__tests__/rankingTimeRange.spec.ts src/views/admin/__tests__/UsageView.spec.ts src/components/admin/usage/__tests__/UserTokenRanking.spec.ts src/components/admin/usage/__tests__/RechargeRanking.spec.ts`
Expected: PASS.

**Step 3: Run type and locale checks**

Run: `pnpm --dir frontend run typecheck && pnpm --dir frontend run check:i18n`
Expected: PASS with no missing locale keys.

### Task 4: Confirm backend date semantics and guard regressions

**Files:**
- Modify: `backend/internal/handler/admin/usage_handler_test.go` (only if needed)
- Modify: `backend/internal/handler/admin/dashboard_handler_test.go` (only if needed)
- Modify: `backend/internal/repository/recharge_ranking_repo_test.go` (only if needed)

**Step 1: Add/adjust failing boundary tests**

Verify same-day ranges include the full local day, yesterday excludes today, and a range crossing a month boundary includes both dates. Ensure reverse ranges still return 400.

**Step 2: Run targeted Go tests**

Run: `go test -tags=unit ./internal/handler/admin ./internal/repository -run 'Date|Ranking' -count=1`
Expected: PASS.

**Step 3: Keep SQL unchanged unless a test proves a bug**

The existing queries already use half-open time intervals and accept the required parameters. Do not introduce a migration or alter aggregation sources for this UI-only change.

### Task 5: Full validation and release preparation

**Files:**
- No additional production files.

**Step 1: Run full checks**

Run:
- `make -C backend test-unit`
- `pnpm --dir frontend run typecheck`
- `pnpm --dir frontend run check:i18n`
- `pnpm --dir frontend run build`
- `git diff --check`

Expected: all commands pass.

**Step 2: Review the diff**

Confirm only ranking time-filter UI, localization, tests, and any proven backend boundary test changes are included. Confirm both ranking components still receive one shared range.

**Step 3: Commit implementation**

```bash
git add frontend/src/views/admin/UsageView.vue frontend/src/views/admin/__tests__/UsageView.spec.ts frontend/src/views/admin/__tests__/rankingTimeRange.spec.ts frontend/src/components/admin/usage/__tests__ frontend/src/i18n/locales backend/internal/handler/admin backend/internal/repository
git commit -m "feat: add time filters to usage rankings"
```
