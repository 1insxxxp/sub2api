# Commission Calendar Responsive Layout Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild the admin workbench spend calendar and day detail layout so amounts are readable on desktop and mobile without wrapping inside narrow calendar cells.

**Architecture:** Keep all existing APIs and state flow. Move full monetary values into summary components outside the seven-column calendar grid, keep cells as compact date/status selectors, and reshape day group cards with explicit content regions that collapse to one column below the desktop breakpoint.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vue Test Utils

---

### Task 1: Lock the responsive calendar contract

**Files:**
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`
- Test: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

**Step 1: Write the failing test**

Update the existing mobile calendar test to require a date-only cell, a selected-day summary containing both full amounts, and stable responsive layout markers.

**Step 2: Run test to verify it fails**

Run: `pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: FAIL because the old calendar still renders desktop amounts inside each date cell and the new summary/layout markers do not exist.

### Task 2: Rebuild the calendar hierarchy

**Files:**
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue`

**Step 1: Write minimal implementation**

Replace in-cell monetary blocks with a compact status marker, add two month summary blocks, and render a single selected-day monetary summary below the calendar.

**Step 2: Run focused test**

Run: `pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: PASS for calendar layout assertions.

### Task 3: Rebuild the day detail cards

**Files:**
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue`
- Modify: `frontend/src/components/admin/workbench/SubAdminCommissionPanel.vue`
- Modify: `frontend/src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

**Step 1: Write the failing test**

Require the panel to use the new desktop column ratio and the group card to expose separate mobile-safe name, metrics, amount, and action regions.

**Step 2: Run test to verify it fails**

Run: `pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: FAIL on the missing layout markers.

**Step 3: Write minimal implementation**

Use a 60/40 desktop grid, keep the detail panel aligned to the top, and change each group row to a responsive grid with non-wrapping monetary values and a full-width mobile action.

**Step 4: Run focused test**

Run: `pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: PASS.

### Task 4: Verify the finished layout

**Files:**
- Verify: `frontend/src/components/admin/workbench/SubAdminCommissionCalendar.vue`
- Verify: `frontend/src/components/admin/workbench/SubAdminCommissionDayDrawer.vue`
- Verify: `frontend/src/components/admin/workbench/SubAdminCommissionPanel.vue`

**Step 1: Run automated verification**

Run: `pnpm typecheck`

Run: `pnpm test:run src/views/admin/__tests__/AdminWorkbenchView.spec.ts`

Expected: both commands PASS.

**Step 2: Run visual verification**

Open `http://localhost:3000/admin/workbench` at desktop and mobile widths. Confirm all labels, amounts, group names, and controls remain visible without overlap or horizontal page scrolling.
