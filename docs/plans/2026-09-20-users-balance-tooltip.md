# 用户管理余额提示 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Prevent the admin users table's balance hint from covering table data while preserving the balance history action.

**Architecture:** Keep the existing balance button and click handler in `UsersView.vue`, remove only the absolutely positioned hover tooltip, and expose its text through `aria-label`. Extend the existing `UsersView.spec.ts` stub coverage with a focused regression assertion.

**Tech Stack:** Vue 3, TypeScript, Vitest, Vue Test Utils, Tailwind utility classes.

---

### Task 1: Add the failing regression test

**Files:**
- Modify: `frontend/src/views/admin/__tests__/UsersView.spec.ts`

**Step 1: Render the balance slot in the existing DataTable stub and assert the intended contract.**

The test should locate the balance history button, require the `aria-label`, and require that no hover-only tooltip class is rendered.

**Step 2: Run the focused test.**

Run: `cd frontend && npx vitest run src/views/admin/__tests__/UsersView.spec.ts`

Expected: FAIL until the component markup removes the hover tooltip and adds the accessible label.

### Task 2: Remove the covering hover layer

**Files:**
- Modify: `frontend/src/views/admin/UsersView.vue`

**Step 1: Keep the balance button and `handleBalanceHistory(row)` click behavior.**

Add a stable test hook and `aria-label` using the existing translated hint, while removing the wrapping `group` and absolutely positioned hover element.

**Step 2: Re-run the focused test.**

Expected: PASS, with no hover tooltip element in the rendered balance cell.

### Task 3: Verify the local UI

**Files:**
- No additional source files.

**Step 1: Run `git diff --check`.**

**Step 2: Run the focused Vitest test and the frontend build/type check used by the project.**

**Step 3: Open the local `/admin/users` page and inspect the first rows and horizontal scroll state.**

Expected: balance, ID, notes, and usage values remain unobstructed; clicking the balance still opens the history modal.
