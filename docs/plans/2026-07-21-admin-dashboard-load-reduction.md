# Admin Dashboard Load Reduction Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Reduce admin-triggered PostgreSQL aggregation load by defaulting the main dashboard to today and preventing operations dashboard polling.

**Architecture:** Keep all existing dashboard endpoints and manual controls. Narrow only the initial main-dashboard query window, and make the operations dashboard load once without starting its countdown timer even when persisted settings previously enabled polling.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils, Vite

---

### Task 1: Default The Admin Dashboard To Today

**Files:**
- Modify: `frontend/src/views/admin/DashboardView.vue`
- Test: `frontend/src/views/admin/__tests__/DashboardView.spec.ts`

**Step 1: Update the existing range test to fail**

Rename the test to `uses the current local day as the default dashboard range`. Assert that both `start_date` and `end_date` equal `formatLocalDate(new Date())`, with hourly granularity.

**Step 2: Run the focused test**

Run: `npm run test:run -- src/views/admin/__tests__/DashboardView.spec.ts`

Expected: FAIL because `start_date` is still yesterday for most of the day.

**Step 3: Implement the minimal range change**

Replace `getLast24HoursRangeDates` with a helper that returns today's local date for both range boundaries:

```ts
const getCurrentDayRangeDates = (): { start: string; end: string } => {
  const today = formatLocalDate(new Date())
  return { start: today, end: today }
}
```

Use that helper to initialize `startDate` and `endDate`. Keep hourly granularity.

**Step 4: Run the focused test**

Run: `npm run test:run -- src/views/admin/__tests__/DashboardView.spec.ts`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/DashboardView.vue frontend/src/views/admin/__tests__/DashboardView.spec.ts
git commit -m "perf(admin): default dashboard statistics to today"
```

### Task 2: Disable Operations Dashboard Polling

**Files:**
- Modify: `frontend/src/views/admin/ops/OpsDashboard.vue`
- Create: `frontend/src/views/admin/ops/__tests__/OpsDashboardAutoRefresh.spec.ts`

**Step 1: Add a failing source-contract test**

Read `OpsDashboard.vue` and assert that `loadDashboardAdvancedSettings` explicitly assigns `false` to `autoRefreshEnabled` and resets the countdown, rather than copying `settings.auto_refresh_enabled` into runtime state. Also assert that the manual `fetchData` path remains present.

**Step 2: Run the focused test**

Run: `npm run test:run -- src/views/admin/ops/__tests__/OpsDashboardAutoRefresh.spec.ts`

Expected: FAIL because the page currently enables polling from persisted settings.

**Step 3: Implement the minimal runtime guard**

Keep loading presentation settings, but force polling off:

```ts
autoRefreshEnabled.value = false
autoRefreshIntervalMs.value = settings.auto_refresh_interval_seconds * 1000
autoRefreshCountdown.value = 0
```

Do not start `resumeCountdown` on mount. Retain manual refresh and all existing fetch logic.

**Step 4: Run the focused test**

Run: `npm run test:run -- src/views/admin/ops/__tests__/OpsDashboardAutoRefresh.spec.ts`

Expected: PASS.

**Step 5: Commit**

```bash
git add frontend/src/views/admin/ops/OpsDashboard.vue frontend/src/views/admin/ops/__tests__/OpsDashboardAutoRefresh.spec.ts
git commit -m "perf(admin): disable operations dashboard polling"
```

### Task 3: Verify The Frontend

**Files:**
- Verify only

**Step 1: Run both focused test files**

Run: `npm run test:run -- src/views/admin/__tests__/DashboardView.spec.ts src/views/admin/ops/__tests__/OpsDashboardAutoRefresh.spec.ts`

Expected: PASS.

**Step 2: Run type checking**

Run: `npm run typecheck`

Expected: PASS.

**Step 3: Run the production build**

Run: `npm run build`

Expected: PASS and write assets to `backend/internal/web/dist`.

**Step 4: Inspect the final diff**

Run: `git diff --check` and `git status --short`.

Expected: no whitespace errors and only expected generated build output, if tracked by the repository.
