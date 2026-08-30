# Mobile Header Toolbar Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Deliver a compact, theme-consistent mobile header with no language switcher and stable access to every primary action.

**Architecture:** Keep all existing header behavior in `AppHeader.vue`, adding responsive wrappers and shared mobile-control hooks rather than creating new components. Use the existing Tailwind and global header styles for breakpoint-specific sizing, spacing, colors, and dark mode.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest, Vue Test Utils

---

### Task 1: Specify Mobile Header Behavior

**Files:**
- Modify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`

**Step 1: Write the failing tests**

Add focused assertions that the locale wrapper uses `hidden sm:block`, mobile action controls expose the shared compact class, and the check-in label uses `hidden sm:inline`.

**Step 2: Run tests to verify they fail**

Run: `pnpm test:run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: FAIL because the locale wrapper and unified mobile classes do not exist and the check-in label currently reappears at 360px.

### Task 2: Rebuild Header Markup and Styling

**Files:**
- Modify: `frontend/src/components/layout/AppHeader.vue`
- Modify: `frontend/src/style.css`

**Step 1: Implement the minimal responsive markup**

Wrap the locale switcher in a mobile-hidden container, add stable test hooks, apply the shared compact class to mobile controls, and hide the check-in label until `sm`.

**Step 2: Implement the mobile toolbar style**

Add a small-screen-only rule for fixed 36px controls, blue/cyan theme surfaces, non-wrapping actions, controlled spacing, focus visibility, and dark mode.

**Step 3: Run the focused tests**

Run: `pnpm test:run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: PASS.

### Task 3: Verify and Integrate

**Files:**
- Verify: `frontend/src/components/layout/AppHeader.vue`
- Verify: `frontend/src/style.css`
- Verify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`

**Step 1: Run type and build verification**

Run: `pnpm typecheck`

Run: `pnpm build`

Expected: both commands complete successfully.

**Step 2: Inspect the diff**

Confirm desktop-only controls remain unchanged and no unrelated files are modified.

**Step 3: Commit and merge**

Commit the verified implementation on `codex/mobile-header-toolbar`, then merge it into local `dev` without touching pre-existing uncommitted changes.
