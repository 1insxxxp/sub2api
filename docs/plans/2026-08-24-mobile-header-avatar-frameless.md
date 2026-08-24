# Mobile Header Frameless Avatar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Show the mobile header avatar without a decorative button frame or avatar ring while preserving interaction and layout.

**Architecture:** Add dedicated semantic classes to the existing user-menu trigger and avatar container, then apply narrowly scoped mobile CSS overrides. Keep the shared mobile action sizing and desktop styles unchanged.

**Tech Stack:** Vue 3, Tailwind CSS, scoped/global CSS, Vitest, Vue Test Utils

---

### Task 1: Specify the frameless mobile avatar

**Files:**
- Modify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`

**Step 1: Write the failing test**

Add a test that verifies the user-menu trigger and avatar expose dedicated classes, and that the mobile CSS removes border, background, and shadow while retaining a visible focus state.

**Step 2: Run test to verify it fails**

Run: `pnpm --dir frontend exec vitest run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: FAIL because the dedicated avatar classes and mobile overrides do not exist yet.

### Task 2: Implement the mobile-only frameless treatment

**Files:**
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/style.css`

**Step 1: Add semantic hooks**

Add `app-header-user-trigger` to the user-menu button and `app-header-user-avatar` to the avatar container.

**Step 2: Add minimal mobile overrides**

Within the existing `max-width: 639px` block, clear the trigger border/background/shadow in light and dark themes, remove the avatar ring/shadow, and preserve a visible `focus-visible` outline.

**Step 3: Run the focused test**

Run: `pnpm --dir frontend exec vitest run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: PASS.

### Task 3: Verify the frontend

**Files:**
- Verify: `frontend/src/components/layout/AppHeader.vue`
- Verify: `frontend/src/style.css`
- Verify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`

**Step 1: Run typecheck**

Run: `pnpm --dir frontend run typecheck`

Expected: PASS with zero errors.

**Step 2: Run targeted lint**

Run: `pnpm --dir frontend exec eslint src/components/layout/AppHeader.vue src/components/layout/__tests__/AppHeader.spec.ts`

Expected: PASS with zero errors.

**Step 3: Verify the running page**

At a 320px mobile viewport, confirm the avatar has no decorative frame, remains clickable, retains keyboard focus feedback, and does not introduce horizontal overflow.
