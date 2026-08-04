# Custom Groups Modal Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Manage custom model groups in a responsive modal opened from the API-key page without route navigation.

**Architecture:** Extract the current custom-group CRUD UI and state into a reusable `CustomGroupsManager` component with list/form modes. Mount it inside a full-width `BaseDialog` in `KeysView`, while the compatibility route wraps the same manager in `AppLayout`.

**Tech Stack:** Vue 3, TypeScript, Tailwind CSS, Vitest.

---

### Task 1: Add failing UI contracts

**Files:**
- Modify: `frontend/src/views/user/__tests__/KeysViewLayout.spec.ts`
- Create: `frontend/src/components/custom-groups/__tests__/CustomGroupsManager.spec.ts`
- Modify: `frontend/src/views/user/__tests__/CustomGroupsViewLayout.spec.ts`

Add assertions for a non-router modal trigger, responsive dialog, single-layer manager modes, and route wrapper reuse. Run the focused tests and confirm failure.

### Task 2: Extract the manager

**Files:**
- Create: `frontend/src/components/custom-groups/CustomGroupsManager.vue`
- Modify: `frontend/src/views/user/CustomGroupsView.vue`

Move list, CRUD state, and model selection into the manager. Replace nested dialogs with list/form conditional rendering. Keep the route as an `AppLayout` wrapper.

### Task 3: Add the API-key modal

**Files:**
- Modify: `frontend/src/views/user/KeysView.vue`

Replace the router link with an amber action button. Add a `BaseDialog` using `width="full"`, render `CustomGroupsManager`, and close without route changes.

### Task 4: Verify and run locally

Run focused tests, `pnpm typecheck`, full Vitest, and `pnpm build`. Restart the local backend so the embedded frontend contains the new modal UI, then commit.
