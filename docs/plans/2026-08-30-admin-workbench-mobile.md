# Admin Workbench Mobile Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make every `/admin/workbench` tab readable and operable at 320px-430px widths without horizontal page overflow.

**Architecture:** Keep the existing Vue components and API calls, and add one consistent mobile-first responsive layout across the page shell, balance-transfer cards, commission calendar/dialog, and affiliate leaderboard. Desktop markup remains available at existing breakpoints; mobile-specific composition is expressed with Tailwind responsive utilities and verified by focused source/component tests.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vue Test Utils

---

### Task 1: Lock the responsive page shell and balance-transfer layout

**Files:**
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts`
- Modify: `frontend/src/views/admin/AdminWorkbenchView.vue`

**Step 1: Write the failing tests**

Add assertions requiring:

- a three-column mobile tab grid that switches to the desktop flex row at `sm`
- compact mobile page spacing
- a bounded mobile generated-result area
- stacked generated-result header actions
- a two-column mobile action toolbar and width-safe code cards

**Step 2: Run the tests to verify they fail**

Run:

```bash
cd frontend
pnpm vitest run src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts
```

Expected: FAIL because the current tabs use horizontal scrolling and generated-result controls do not expose the new mobile contracts.

**Step 3: Implement the page and balance-transfer layout**

- Change the workbench tab navigation to `grid grid-cols-3` below `sm` and preserve `sm:flex` above it.
- Give each tab a stable mobile height with stacked icon/label content.
- Reduce mobile padding to `px-3 py-4` and card padding to `p-4`, restoring current spacing at `sm`.
- Make the generated-now header and copy action stack below 360px.
- Cap the generated-results list on mobile and keep internal vertical scrolling.
- Use a two-column control grid below `sm`; make refresh span the full row where necessary.
- Keep code-card actions on a dedicated final row and allow codes/notes to wrap within the card.

**Step 4: Run the focused tests**

Run the Task 1 Vitest command and expect PASS.

### Task 2: Build a compact mobile commission calendar and centered day dialog

**Files:**
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionManagement.vue`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue`

**Step 1: Write the failing tests**

Require source contracts for:

- a seven-column mobile calendar grid
- compact day cells with mobile-only data markers instead of full inline amounts
- a centered viewport-bounded mobile dialog
- responsive two-column amount tiles and wrapping log identity fields
- mobile-safe management header/actions and group selector cards

**Step 2: Run the focused test and verify failure**

Run the mobile layout spec and expect the current one/two-column calendar and bottom sheet classes to fail.

**Step 3: Implement the commission layout**

- Render weekday labels and seven calendar columns at every viewport.
- Use square, fixed-format day cells on mobile; show only date and a data marker below `sm`.
- Keep compact full amounts in desktop day cells.
- Center the day dialog with `items-center`, mobile page insets, safe maximum height, and consistent rounded corners.
- Use two amount tiles from 360px upward and one column below it.
- Stack the management grants heading/button on mobile and keep group names wrapping within the viewport.

**Step 4: Run the focused tests**

Run the mobile layout spec and expect PASS.

### Task 3: Refine the affiliate leaderboard mobile cards

**Files:**
- Modify: `frontend/src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts`
- Modify: `frontend/src/components/admin/workbench/AdminAffiliateLeaderboardPanel.vue`

**Step 1: Write a failing component test**

Assert that mobile cards contain a two-column metric grid, a full-width rebate metric, and width-safe identity text while preserving avatar and fallback behavior.

**Step 2: Run the component test and verify failure**

```bash
cd frontend
pnpm vitest run src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts
```

Expected: FAIL because the current metrics use three equal columns.

**Step 3: Implement the mobile card changes**

- Keep rank, avatar, and identity in the top row.
- Change metrics to two columns.
- Let the rebate metric span both columns with amount aligned independently from its label.
- Preserve non-wrapping numeric values and wrapping email/username text.

**Step 4: Run the component test**

Expect PASS.

### Task 4: Verify behavior and visual containment

**Files:**
- Test: all files modified above

**Step 1: Run focused tests**

```bash
cd frontend
pnpm vitest run \
  src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts \
  src/views/admin/__tests__/AdminWorkbenchView.spec.ts \
  src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts
```

**Step 2: Run static and production checks**

```bash
pnpm typecheck
pnpm build
```

**Step 3: Run visual verification**

Start the local frontend, inspect `/admin/workbench` at 375x812 and 1440x900, and verify:

- `document.documentElement.scrollWidth === document.documentElement.clientWidth`
- tab labels and actions remain fully visible
- the calendar fits seven columns without amount overflow
- the day dialog stays inside the viewport and scrolls internally
- leaderboard metrics do not collide or wrap unpredictably

**Step 4: Check the diff**

```bash
git diff --check
git status --short
```

**Step 5: Commit**

```bash
git add frontend/src/views/admin/AdminWorkbenchView.vue \
  frontend/src/views/admin/__tests__/AdminWorkbenchView.mobile.spec.ts \
  frontend/src/components/admin/workbench/SubAdminCommissionManagement.vue \
  frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue \
  frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue \
  frontend/src/components/admin/workbench/AdminAffiliateLeaderboardPanel.vue \
  frontend/src/components/admin/workbench/__tests__/AdminAffiliateLeaderboardPanel.spec.ts \
  docs/plans/2026-08-30-admin-workbench-mobile.md
git commit -m "fix(admin): refine workbench mobile layout"
```

