# Mobile Header Flat Actions Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rebuild the mobile header actions as flat, theme-colored icon controls while retaining the subscription status pill and frameless avatar.

**Architecture:** Change only the existing mobile CSS block and focused component assertions. Reuse the current markup, icon components, control dimensions, and toolbar budget so no behavior or desktop layout changes.

**Tech Stack:** Vue 3, Tailwind CSS, CSS custom properties, Vitest, Vue Test Utils

---

### Task 1: Specify the flat mobile action states

**Files:**
- Modify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`

**Step 1: Update the focused style test**

Require the shared mobile action rule to use a transparent border/background and no shadow. Require hover and focus states to use subtle theme-color feedback.

**Step 2: Run the test and confirm it fails**

Run: `pnpm --dir frontend exec vitest run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: FAIL because the current controls still render outlined gradient boxes.

### Task 2: Implement the flat mobile action style

**Files:**
- Modify: `frontend/src/style.css`

**Step 1: Flatten the shared action rule**

Keep the 36px dimensions and centered icon alignment, but replace the border, gradient, and raised shadow with transparent surfaces.

**Step 2: Add restrained interaction feedback**

Use a subtle theme-tinted hover background, a slightly stronger active background, and an accessible focus ring. Keep disabled and dark-mode states visually quiet.

**Step 3: Preserve status hierarchy**

Leave the subscription status pill and frameless avatar overrides intact.

**Step 4: Run the focused suite**

Run: `pnpm --dir frontend exec vitest run src/components/layout/__tests__/AppHeader.spec.ts`

Expected: PASS.

### Task 3: Verify the frontend

**Files:**
- Verify: `frontend/src/style.css`
- Verify: `frontend/src/components/layout/__tests__/AppHeader.spec.ts`

**Step 1: Run typecheck**

Run: `pnpm --dir frontend run typecheck`

Expected: PASS.

**Step 2: Run targeted lint**

Run: `pnpm --dir frontend exec eslint src/components/layout/AppHeader.vue src/components/layout/__tests__/AppHeader.spec.ts`

Expected: PASS.

**Step 3: Check local services**

Verify `localhost:3000` and `127.0.0.1:18081/health` both return HTTP 200.
