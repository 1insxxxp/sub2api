# 管理员收益图表 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 将管理员工作台的收益管理置于首位并默认打开，同时在复用现有月份数据的基础上展示响应式每日收益趋势图。

**Architecture:** `AdminWorkbenchView` 只调整工作台标签的顺序、默认状态和键盘导航顺序。`SubAdminCommissionCalendar` 继续请求并持有月份数据，将 `days` 和 `loading` 传给新的纯展示组件 `SubAdminCommissionTrendChart`；这样图表与日历共享一次请求、共享月份切换和错误边界，不需要后端改动。趋势图使用项目已有的 Chart.js/vue-chartjs，按日期排序后绘制余额消耗与收益金额两条金额折线。

**Tech Stack:** Vue 3 `<script setup>`, TypeScript, Tailwind CSS, Chart.js 4, vue-chartjs 5, Vitest and Vue Test Utils.

---

### Task 1: Lock the tab-order behavior with a failing test

**Files:**
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

**Step 1: Update the existing tab assertions to the requested contract**

Change the initial assertions so `workbench-tab-commission` has `aria-selected="true"` and `tabindex="0"`, while balance-transfer and leaderboard are unselected. Assert the commission panel is mounted and the balance-transfer panel is absent on first render. Update the keyboard test to use commission as the first tab: ArrowRight goes to balance-transfer, End goes to affiliate-leaderboard, and Home returns to commission.

**Step 2: Run the focused test before implementation**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: FAIL because `AdminWorkbenchView.vue` still initializes `activeTab` and `workbenchTabIds` with `balance-transfer`.

**Step 3: Commit the test-only expectation change**

```bash
git add frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts
git commit -m "test: expect earnings tab as workbench default"
```

### Task 2: Implement the first-tab ordering

**Files:**
- Modify: `frontend/src/views/admin/AdminWorkbenchView.vue:433-451`

**Step 1: Set the default and ordered IDs**

Initialize `activeTab` to `'commission'`, set `workbenchTabIds` to `['commission', 'balance-transfer', 'affiliate-leaderboard']`, and emit `workbenchTabs` in the same order. Keep all existing panel rendering and generated-code loading behavior intact.

**Step 2: Run the focused workbench tests**

Run: `cd frontend && pnpm vitest run src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: PASS.

### Task 3: Add a failing unit test for the daily trend component

**Files:**
- Create: `frontend/src/components/admin/workbench/__tests__/SubAdminCommissionTrendChart.spec.ts`

**Step 1: Mock Chart.js rendering and define fixtures**

Mock `vue-chartjs` `Line` with a component that renders `JSON.stringify(data)` so the test does not depend on a browser canvas. Mount the component with dates out of order, including a zero-value day.

**Step 2: Assert the data contract and states**

Assert the chart has a stable `data-test="commission-daily-chart"`, labels are sorted and shortened to `MM-DD`, both datasets preserve the same order and zero value, and the series names use the commission locale keys. Add cases for an empty array (localized empty state) and `loading=true` (loading state). Add a source-style assertion for `min-w-0 h-56 sm:h-64` to protect narrow-screen layout.

**Step 3: Run the new test before creating the component**

Run: `cd frontend && pnpm vitest run src/components/admin/workbench/__tests__/SubAdminCommissionTrendChart.spec.ts`

Expected: FAIL because the component does not exist yet.

### Task 4: Implement the responsive daily earnings chart

**Files:**
- Create: `frontend/src/components/admin/workbench/SubAdminCommissionTrendChart.vue`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue:1-110`

**Step 1: Build the chart component**

Accept `days: SubAdminCommissionCalendarDay[]` and `loading?: boolean`. Sort a copied array by `date`, use `date.slice(5)` labels, and create two datasets: `commission_amount` in the existing emerald theme and `actual_cost` in the existing blue theme. Configure responsive sizing, `maintainAspectRatio: false`, compact mobile ticks, dark-mode colors, non-intersecting tooltips with two-decimal currency formatting, and a wrapped legend. Render loading and empty branches without mounting `Line`.

**Step 2: Embed it without a second request**

Import the component in `SubAdminCommissionCalendar.vue` and render it below the month summary and above the calendar grid with `:days="days"` and `:loading="loading"`. Keep the existing month input, totals, selectable days, and error handling unchanged.

**Step 3: Run the component test**

Run: `cd frontend && pnpm vitest run src/components/admin/workbench/__tests__/SubAdminCommissionTrendChart.spec.ts`

Expected: PASS.

### Task 5: Add localized copy and mobile regression coverage

**Files:**
- Modify: `frontend/src/i18n/locales/zh/common.ts`
- Modify: `frontend/src/i18n/locales/en/common.ts`
- Modify: `frontend/src/i18n/__tests__/adminWorkbenchLocales.spec.ts`
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts`

**Step 1: Add matching locale keys**

Under `adminWorkbench.commission`, add translated keys for the chart title, balance-spend series, earnings series, and no-data copy. Keep the key tree identical in both locales.

**Step 2: Extend locale and source-layout tests**

Assert the new keys resolve in Chinese and English. Assert the calendar source includes the chart component, `commission-daily-chart`, and responsive width/height classes.

**Step 3: Run focused tests**

Run: `cd frontend && pnpm vitest run src/components/admin/workbench/__tests__/SubAdminCommissionTrendChart.spec.ts src/views/admin/__tests__/AdminWorkbenchView.spec.ts src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts src/i18n/__tests__/adminWorkbenchLocales.spec.ts`

Expected: PASS.

### Task 6: Verify the full frontend change and commit

**Files:**
- All files from Tasks 1–5

**Step 1: Run validation**

Run:

```bash
cd frontend
pnpm vitest run src/components/admin/workbench/__tests__/SubAdminCommissionTrendChart.spec.ts src/views/admin/__tests__/AdminWorkbenchView.spec.ts src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts src/api/__tests__/admin.subAdminCommission.spec.ts src/i18n/__tests__/adminWorkbenchLocales.spec.ts
pnpm typecheck
pnpm check:i18n
pnpm build
```

Expected: all commands exit 0; the build emits the existing Browserslist freshness notice only if present in the baseline.

**Step 2: Review the diff and keep user changes out**

Run `git diff --check` and `git status --short`; do not stage the user-modified `AGENTS.md`.

**Step 3: Commit the implementation**

```bash
git add frontend/src/components/admin/workbench/SubAdminCommissionTrendChart.vue frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue frontend/src/components/admin/workbench/__tests__/SubAdminCommissionTrendChart.spec.ts frontend/src/views/admin/AdminWorkbenchView.vue frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts frontend/src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts frontend/src/i18n/locales/zh/common.ts frontend/src/i18n/locales/en/common.ts frontend/src/i18n/__tests__/adminWorkbenchLocales.spec.ts
git commit -m "feat: add daily earnings trend to admin workbench"
```

